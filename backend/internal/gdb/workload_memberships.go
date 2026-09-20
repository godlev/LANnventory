package gdb

import "github.com/godlev/LANnventory/internal/models"

// SelectInfrastructureWorkloadMemberships returns the current persisted MATCHES
// relations together with their workload inventory records. Host and
// hypervisor display data are resolved by the API from the current Host table.
func SelectInfrastructureWorkloadMemberships() ([]models.InfrastructureWorkloadMembershipRecord, error) {
	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var links []models.InfrastructureWorkloadHostLink
	if err := activeDB.Table(infrastructureWorkloadHostLinksTable).
		Order(`"HOST_ID" ASC, "WORKLOAD_ID" ASC`).
		Find(&links).Error; err != nil {
		return nil, err
	}

	result := make([]models.InfrastructureWorkloadMembershipRecord, 0, len(links))
	for _, link := range links {
		var workload models.InfrastructureWorkload
		if err := activeDB.Table(infrastructureWorkloadsTable).
			Where(`"ID" = ?`, link.WorkloadID).
			First(&workload).Error; err != nil {
			continue
		}
		result = append(result, models.InfrastructureWorkloadMembershipRecord{
			Workload: workload,
			Link:     link,
		})
	}
	return result, nil
}
