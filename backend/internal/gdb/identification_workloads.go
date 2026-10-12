package gdb

import (
	"sort"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

// SelectInfrastructureWorkloadsByInterfaceMAC returns workload aggregates whose
// retained interface inventory currently contains the supplied MAC. This is a
// read-only identity lookup; callers must still evaluate workload retirement
// and source freshness before treating the match as current evidence.
func SelectInfrastructureWorkloadsByInterfaceMAC(mac string) ([]models.InfrastructureWorkloadRecord, error) {
	canonical, err := identity.NormalizeMAC(mac)
	if err != nil {
		return nil, err
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		return nil, err
	}
	defer release()

	var workloadIDs []uint
	if err := activeDB.Table(infrastructureWorkloadInterfacesTable).
		Where(`"MAC" = ?`, canonical).
		Order(`"WORKLOAD_ID" ASC, "ID" ASC`).
		Pluck("WORKLOAD_ID", &workloadIDs).Error; err != nil {
		return nil, err
	}

	unique := make(map[uint]struct{}, len(workloadIDs))
	ids := make([]uint, 0, len(workloadIDs))
	for _, id := range workloadIDs {
		if id == 0 {
			continue
		}
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	records := make([]models.InfrastructureWorkloadRecord, 0, len(ids))
	for _, id := range ids {
		record, found, err := selectInfrastructureWorkloadByIDDB(activeDB, id)
		if err != nil {
			return nil, err
		}
		if found {
			records = append(records, record)
		}
	}
	return records, nil
}
