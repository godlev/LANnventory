package api

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/portscan"
)

var errPortScanInProgress = errors.New("a port scan is already running for this host")

var scanPortRange = portscan.ScanRange

type portScanJobStatus struct {
	ID          string `json:"id"`
	HostID      int    `json:"hostId"`
	StartPort   int    `json:"startPort"`
	EndPort     int    `json:"endPort"`
	CurrentPort int    `json:"currentPort"`
	Scanned     int    `json:"scanned"`
	Total       int    `json:"total"`
	Running     bool   `json:"running"`
	Completed   bool   `json:"completed"`
	Cancelled   bool   `json:"cancelled"`
	OpenPorts   []int  `json:"openPorts"`
	StartedAt   string `json:"startedAt"`
	FinishedAt  string `json:"finishedAt"`
	Error       string `json:"error"`
}

type portScanJob struct {
	mu           sync.RWMutex
	status       portScanJobStatus
	openPorts    map[int]struct{}
	observations map[int]bool
	cancel       context.CancelFunc
}

type portScanJobManager struct {
	mu           sync.Mutex
	jobs         map[string]*portScanJob
	activeByHost map[int]string
}

func newPortScanJobManager() *portScanJobManager {
	return &portScanJobManager{
		jobs:         make(map[string]*portScanJob),
		activeByHost: make(map[int]string),
	}
}

var portScanJobs = newPortScanJobManager()

func (m *portScanJobManager) Start(host models.Host, startPort, endPort int) (portScanJobStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupLocked(time.Now().Add(-time.Hour))
	if activeID := m.activeByHost[host.ID]; activeID != "" {
		if active := m.jobs[activeID]; active != nil {
			active.mu.RLock()
			running := active.status.Running
			active.mu.RUnlock()
			if running {
				return portScanJobStatus{}, errPortScanInProgress
			}
		}
		delete(m.activeByHost, host.ID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	id := fmt.Sprintf("%d-%d", host.ID, time.Now().UnixNano())
	job := &portScanJob{
		status: portScanJobStatus{
			ID:        id,
			HostID:    host.ID,
			StartPort: startPort,
			EndPort:   endPort,
			Total:     endPort - startPort + 1,
			Running:   true,
			OpenPorts: []int{},
			StartedAt: time.Now().UTC().Format(time.RFC3339),
		},
		openPorts:    make(map[int]struct{}),
		observations: make(map[int]bool),
		cancel:       cancel,
	}
	m.jobs[id] = job
	m.activeByHost[host.ID] = id

	go m.run(ctx, host, job)
	return job.snapshot(), nil
}

func (m *portScanJobManager) run(ctx context.Context, host models.Host, job *portScanJob) {
	err := scanPortRange(
		ctx,
		host.IP,
		job.status.StartPort,
		job.status.EndPort,
		portscan.DefaultWorkers,
		portscan.DefaultTimeout,
		func(result portscan.Result) {
			job.record(result)
		},
	)

	observations := job.observationSnapshot()
	observedAt := time.Now().Format(models.HostEventDateLayout)
	if len(observations) > 0 {
		if _, persistErr := gdb.ReconcileHostPortObservations(host, observations, observedAt); persistErr != nil {
			if err == nil {
				err = persistErr
			}
		}
	}

	cancelled := errors.Is(err, context.Canceled)
	if cancelled {
		err = nil
	}
	job.finish(cancelled, err)
	jobID := job.snapshot().ID

	m.mu.Lock()
	if m.activeByHost[host.ID] == jobID {
		delete(m.activeByHost, host.ID)
	}
	m.mu.Unlock()
}

func (m *portScanJobManager) Active(hostID int) (portScanJobStatus, bool) {
	m.mu.Lock()
	id := m.activeByHost[hostID]
	job := m.jobs[id]
	m.mu.Unlock()
	if job == nil {
		return portScanJobStatus{}, false
	}

	status := job.snapshot()
	if !status.Running {
		return portScanJobStatus{}, false
	}
	return status, true
}

func (m *portScanJobManager) Status(hostID int, id string) (portScanJobStatus, bool) {
	m.mu.Lock()
	job := m.jobs[id]
	m.mu.Unlock()
	if job == nil {
		return portScanJobStatus{}, false
	}

	status := job.snapshot()
	if status.HostID != hostID {
		return portScanJobStatus{}, false
	}
	return status, true
}

func (m *portScanJobManager) Cancel(hostID int, id string) (portScanJobStatus, bool) {
	m.mu.Lock()
	job := m.jobs[id]
	m.mu.Unlock()
	if job == nil {
		return portScanJobStatus{}, false
	}

	status := job.snapshot()
	if status.HostID != hostID {
		return portScanJobStatus{}, false
	}
	if status.Running && job.cancel != nil {
		job.cancel()
	}
	return job.snapshot(), true
}

func (m *portScanJobManager) cleanupLocked(before time.Time) {
	for id, job := range m.jobs {
		status := job.snapshot()
		if status.Running || status.FinishedAt == "" {
			continue
		}
		finishedAt, err := time.Parse(time.RFC3339, status.FinishedAt)
		if err == nil && finishedAt.Before(before) {
			delete(m.jobs, id)
		}
	}
}

func (job *portScanJob) record(result portscan.Result) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.status.CurrentPort = result.Port
	job.status.Scanned++
	job.observations[result.Port] = result.Open
	if result.Open {
		job.openPorts[result.Port] = struct{}{}
	}
}

func (job *portScanJob) observationSnapshot() map[int]bool {
	job.mu.RLock()
	defer job.mu.RUnlock()

	copyMap := make(map[int]bool, len(job.observations))
	for port, open := range job.observations {
		copyMap[port] = open
	}
	return copyMap
}

func (job *portScanJob) finish(cancelled bool, err error) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.status.Running = false
	job.status.Completed = true
	job.status.Cancelled = cancelled
	job.status.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		job.status.Error = err.Error()
	}
}

func (job *portScanJob) snapshot() portScanJobStatus {
	job.mu.RLock()
	defer job.mu.RUnlock()

	status := job.status
	status.OpenPorts = make([]int, 0, len(job.openPorts))
	for port := range job.openPorts {
		status.OpenPorts = append(status.OpenPorts, port)
	}
	sort.Ints(status.OpenPorts)
	return status
}
