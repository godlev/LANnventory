package gdb

import (
	"sort"

	"github.com/godlev/LANnventory/internal/models"
)

// SelectInfrastructureWorkloadSummaries counts current non-retired workloads
// by owning hypervisor. Running and stopped workloads are both inventory and
// therefore both count; retired rows are historical and intentionally excluded.
func SelectInfrastructureWorkloadSummaries() ([]models.InfrastructureWorkloadSummary, error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var workloads []models.InfrastructureWorkload
	if err := activeDB.Table(infrastructureWorkloadsTable).
		Where(`"RETIRED_AT" = ? OR "RETIRED_AT" IS NULL`, "").
		Find(&workloads).Error; err != nil {
		return nil, err
	}

	byHypervisor := make(map[string]*models.InfrastructureWorkloadSummary)
	for _, workload := range workloads {
		summary := byHypervisor[workload.HypervisorMac]
		if summary == nil {
			summary = &models.InfrastructureWorkloadSummary{HypervisorMac: workload.HypervisorMac}
			byHypervisor[workload.HypervisorMac] = summary
		}
		switch workload.WorkloadType {
		case models.InfrastructureWorkloadTypeVM:
			summary.VMCount++
		case models.InfrastructureWorkloadTypeContainer:
			summary.ContainerCount++
		}
	}

	result := make([]models.InfrastructureWorkloadSummary, 0, len(byHypervisor))
	for _, summary := range byHypervisor {
		result = append(result, *summary)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].HypervisorMac < result[j].HypervisorMac
	})
	return result, nil
}
