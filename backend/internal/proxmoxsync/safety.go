package proxmoxsync

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoximport"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
	"github.com/godlev/LANnventory/internal/workloadmatch"
)

type AutomaticSafetyResult struct {
	Safe    bool
	Reasons []string
}

type automaticWorkloadMatch struct {
	NativeID            string
	WorkloadType        string
	ManualLinked        bool
	ExistingExactHostID int
	Result              workloadmatch.Result
}

func (s *Service) evaluateAutomaticSafety(
	hypervisorMac string,
	snapshot proxmoxsnapshot.Snapshot,
	preview proxmoximport.Preview,
) (AutomaticSafetyResult, error) {
	if !snapshot.Complete || !preview.ApplyAllowed {
		return assessAutomaticSafety(snapshot, preview, nil), nil
	}
	matches, err := automaticWorkloadMatches(hypervisorMac, snapshot)
	if err != nil {
		return AutomaticSafetyResult{}, err
	}
	return assessAutomaticSafety(snapshot, preview, matches), nil
}

func assessAutomaticSafety(
	snapshot proxmoxsnapshot.Snapshot,
	preview proxmoximport.Preview,
	matches []automaticWorkloadMatch,
) AutomaticSafetyResult {
	reasons := make([]string, 0)
	if !snapshot.Complete {
		reasons = append(reasons, "snapshot is incomplete")
	}
	if !preview.ApplyAllowed {
		if len(preview.BlockedReasons) > 0 {
			reasons = append(reasons, preview.BlockedReasons...)
		} else {
			reasons = append(reasons, "preview contains blocking conflicts")
		}
	}

	for _, item := range matches {
		if item.ManualLinked {
			continue
		}
		label := item.WorkloadType + " " + item.NativeID
		if item.Result.ExactAmbiguous {
			reasons = append(reasons, label+" has ambiguous exact-MAC Host matches")
			continue
		}
		if item.ExistingExactHostID > 0 && item.Result.DeterministicExactHostID != item.ExistingExactHostID {
			reasons = append(reasons, label+" existing exact-MAC Host link is no longer deterministic")
			continue
		}
		for _, candidate := range item.Result.Candidates {
			if candidate.PossibleIPConflict && !candidate.Rejected {
				reasons = append(reasons, fmt.Sprintf("%s has an unresolved possible IP conflict with Host %d", label, candidate.HostID))
				break
			}
		}
	}
	sort.Strings(reasons)
	return AutomaticSafetyResult{Safe: len(reasons) == 0, Reasons: reasons}
}

func automaticWorkloadMatches(hypervisorMac string, snapshot proxmoxsnapshot.Snapshot) ([]automaticWorkloadMatch, error) {
	current, err := loadCurrentState(hypervisorMac, models.InfrastructureWorkloadSourceProxmoxAPI)
	if err != nil {
		return nil, err
	}
	hosts, ok := gdb.Select("now")
	if !ok {
		return nil, errors.New("failed to load current hosts for automatic matching")
	}
	addresses, err := gdb.SelectAllHostAddresses()
	if err != nil {
		return nil, err
	}
	discovery, err := gdb.SelectAllHostDiscoveryEvidence()
	if err != nil {
		return nil, err
	}

	currentByKey := make(map[string]models.InfrastructureWorkloadRecord, len(current.Workloads))
	for _, record := range current.Workloads {
		currentByKey[workloadSafetyKey(record.Workload.WorkloadType, record.Workload.NativeID)] = record
	}

	result := make([]automaticWorkloadMatch, 0, len(snapshot.Workloads))
	for _, workload := range snapshot.Workloads {
		key := workloadSafetyKey(workload.WorkloadType, workload.NativeID)
		record := currentByKey[key]
		record.Workload.HypervisorMac = hypervisorMac
		record.Workload.NativeID = workload.NativeID
		record.Workload.WorkloadType = workload.WorkloadType
		record.Workload.NodeName = workload.NodeName
		record.Workload.Name = workload.Name
		record.Workload.Status = workload.Status
		record.Workload.Source = models.InfrastructureWorkloadSourceProxmoxAPI
		record.Interfaces = make([]models.InfrastructureWorkloadInterface, 0, len(workload.Interfaces))
		for _, iface := range workload.Interfaces {
			record.Interfaces = append(record.Interfaces, models.InfrastructureWorkloadInterface{
				WorkloadID:        record.Workload.ID,
				Name:              iface.Name,
				Mac:               iface.Mac,
				Bridge:            iface.Bridge,
				VLANTag:           iface.VLANTag,
				ConfiguredAddress: iface.ConfiguredAddress,
				ConfiguredNetwork: iface.ConfiguredNetwork,
			})
		}

		match := workloadmatch.Match(record, hypervisorMac, hosts, addresses, discovery)
		if record.Workload.ID > 0 {
			rejections, err := gdb.SelectInfrastructureWorkloadCandidateRejections(record.Workload.ID)
			if err != nil {
				return nil, err
			}
			rejectionByHost := make(map[int]models.InfrastructureWorkloadCandidateRejection, len(rejections))
			for _, rejection := range rejections {
				rejectionByHost[rejection.HostID] = rejection
			}
			for i := range match.Candidates {
				candidate := &match.Candidates[i]
				rejection, found := rejectionByHost[candidate.HostID]
				candidate.Rejected = found &&
					strings.EqualFold(strings.TrimSpace(rejection.HostMac), strings.TrimSpace(candidate.Mac)) &&
					rejection.EvidenceFingerprint == candidate.EvidenceFingerprint
			}
		}

		item := automaticWorkloadMatch{
			NativeID:     workload.NativeID,
			WorkloadType: workload.WorkloadType,
			Result:       match,
		}
		if record.Link != nil {
			item.ManualLinked = record.Link.LinkSource == models.InfrastructureWorkloadLinkSourceManual
			if record.Link.LinkSource == models.InfrastructureWorkloadLinkSourceExactMAC {
				item.ExistingExactHostID = record.Link.HostID
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func workloadSafetyKey(workloadType, nativeID string) string {
	return strings.TrimSpace(workloadType) + "\x00" + strings.TrimSpace(nativeID)
}
