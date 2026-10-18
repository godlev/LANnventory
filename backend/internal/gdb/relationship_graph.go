package gdb

import (
	"errors"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
	"gorm.io/gorm"
)

// SelectRelationshipGraph provides a bounded, repeatable-read (on PostgreSQL)
// snapshot of source-owned assertions and existing Phase 35 Proxmox inventory.
// A nonzero hostID limits the read to one-hop incidents for a CURRENT Host.
// Each category fetches at most limit+1 rows; lookups are batched rather than
// loading a workload/Host separately for every match.
func SelectRelationshipGraph(hostID uint64, limit int) (relationships.Graph, error) {
	if limit < 1 || limit > relationships.MaxGraphLimit {
		return relationships.Graph{}, relationships.ErrInvalidEndpoint
	}
	if hostID > uint64(^uint(0)>>1) {
		return relationships.Graph{}, relationships.ErrInvalidEndpoint
	}
	activeDB, release, err := acquireDB()
	if err != nil {return relationships.Graph{},err}
	defer release()

	var graph relationships.Graph
	err = exportTransaction(activeDB,func(tx *gorm.DB) error {
		snapshot := relationships.GraphSnapshot{}
		cap := limit+1
		truncated := false
		var focus models.Host
		var focusMAC string
		if hostID > 0 {
			if err := tx.Table("now").Where(`"ID" = ?`, hostID).First(&focus).Error;err!=nil{return err}
			focusMAC, _ = identity.NormalizeMAC(focus.Mac)
		}

		// Assertions in the generic table never supersede the two native
		// Proxmox predicates, even if a legacy database contains such rows.
		manualQuery := tx.Table(relationshipAssertionsTable).
			Where(`"RELATIONSHIP_TYPE" NOT IN ?`, []string{string(relationships.TypeHosts),string(relationships.TypeMatchesHost)})
		if hostID > 0 {
			manualQuery = manualQuery.Where(`("SOURCE_ENTITY_TYPE" = ? AND "SOURCE_ENTITY_ID" = ?) OR ("TARGET_ENTITY_TYPE" = ? AND "TARGET_ENTITY_ID" = ?)`,
				string(relationships.EntityHost),hostID,string(relationships.EntityHost),hostID)
		}
		if err:=manualQuery.Order(`"ID" ASC`).Limit(cap).Find(&snapshot.Assertions).Error;err!=nil{return err}
		if len(snapshot.Assertions)==cap {truncated=true}

		workloadQuery:=tx.Table(infrastructureWorkloadsTable)
		if hostID>0 {
			if focusMAC=="" {
				workloadQuery=workloadQuery.Where("1 = 0")
			}else{
				workloadQuery=workloadQuery.Where(`UPPER("HYPERVISOR_MAC") = ?`, focusMAC)
			}
		}
		if err:=workloadQuery.Order(`"ID" ASC`).Limit(cap).Find(&snapshot.Workloads).Error;err!=nil{return err}
		if len(snapshot.Workloads)==cap {truncated=true}

		linkQuery:=tx.Table(infrastructureWorkloadHostLinksTable)
		if hostID>0 {
			if focusMAC=="" {
				linkQuery=linkQuery.Where("1 = 0")
			}else{
				linkQuery=linkQuery.Where(`"HOST_ID" = ? AND UPPER("HOST_MAC") = ?`,hostID,focusMAC)
			}
		}
		if err:=linkQuery.Order(`"WORKLOAD_ID" ASC`).Limit(cap).Find(&snapshot.Links).Error;err!=nil{return err}
		if len(snapshot.Links)==cap {truncated=true}

		// Links may reference workloads not present in the bounded ownership
		// selection. Fetch missing workload records in one additional query.
		loadedWorkloads:=make(map[uint]bool,len(snapshot.Workloads))
		for _,w:=range snapshot.Workloads {loadedWorkloads[w.ID]=true}
		missingWorkloadIDs:=make([]uint,0,len(snapshot.Links))
		for _,link:=range snapshot.Links {
			if !loadedWorkloads[link.WorkloadID] {
				loadedWorkloads[link.WorkloadID]=true
				missingWorkloadIDs=append(missingWorkloadIDs,link.WorkloadID)
			}
		}
		if len(missingWorkloadIDs)>0 {
			var linked []models.InfrastructureWorkload
			if err:=tx.Table(infrastructureWorkloadsTable).Where(`"ID" IN ?`,missingWorkloadIDs).Find(&linked).Error;err!=nil{return err}
			snapshot.Workloads=append(snapshot.Workloads,linked...)
		}
		hostIDs:=make(map[uint64]bool)
		hostMACs:=make(map[string]bool)
		if hostID>0 {hostIDs[hostID]=true}
		for _,assertion:=range snapshot.Assertions {
			if assertion.SourceEntityType==string(relationships.EntityHost) {hostIDs[assertion.SourceEntityID]=true}
			if assertion.TargetEntityType==string(relationships.EntityHost) {hostIDs[assertion.TargetEntityID]=true}
		}
		for _,link:=range snapshot.Links {
			if link.HostID>0 {hostIDs[uint64(link.HostID)]=true}
		}
		for _,workload:=range snapshot.Workloads {
			if mac,err:=identity.NormalizeMAC(workload.HypervisorMac);err==nil {hostMACs[mac]=true}
		}
		ids:=make([]uint64,0,len(hostIDs))
		for id:=range hostIDs {if id>0 {ids=append(ids,id)}}
		macs:=make([]string,0,len(hostMACs))
		for mac:=range hostMACs {macs=append(macs,mac)}
		if len(ids)>0 || len(macs)>0 {
			query:=tx.Table("now")
			switch {
			case len(ids)>0 && len(macs)>0:
				query=query.Where(`"ID" IN ? OR UPPER("MAC") IN ?`,ids,macs)
			case len(ids)>0:
				query=query.Where(`"ID" IN ?`,ids)
			default:
				query=query.Where(`UPPER("MAC") IN ?`,macs)
			}
			if err:=query.Order(`"ID" ASC`).Find(&snapshot.Hosts).Error;err!=nil{return err}
		}

		graph=relationships.ProjectGraph(snapshot,hostID,limit,truncated)
		return nil
	})
	if err!=nil {
		if errors.Is(err,gorm.ErrRecordNotFound) {return relationships.Graph{},gorm.ErrRecordNotFound}
		return relationships.Graph{},err
	}
	return graph,nil
}

