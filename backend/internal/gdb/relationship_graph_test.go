package gdb

import (
	"errors"
	"fmt"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
	"gorm.io/gorm"
)

func TestRelationshipGraphSourceOfTruthAndHostDeletion(t *testing.T) {
	startSelectTestDB(t)
	hypervisor:=seedRelationshipHost(t,1,"AA:BB:CC:DD:38:B1")
	guest:=seedRelationshipHost(t,2,"AA:BB:CC:DD:38:B2")
	_ = seedRelationshipHost(t,3,"AA:BB:CC:DD:38:B3")
	workload,err:=UpsertInfrastructureWorkload(hypervisor.Mac,models.InfrastructureWorkloadUpsert{
		NativeID:"119",WorkloadType:"vm",Name:"plex-immich",Status:"running",Source:models.InfrastructureWorkloadSourceProxmoxAPI,
	},"2026-10-18T09:00:00Z")
	if err!=nil {t.Fatal(err)}
	if _,err:=SetInfrastructureWorkloadHostLink(workload.Workload.ID,guest,models.InfrastructureWorkloadLinkSourceExactMAC,"2026-10-18T09:01:00Z");err!=nil{t.Fatal(err)}
	if _,err:=CreateManualRelationship(manualRelationship(relationships.TypeDependsOn,2,3,"DNS dependency"));err!=nil{t.Fatal(err)}
	graph,err:=SelectRelationshipGraph(0,100)
	if err!=nil || len(graph.Edges)!=3 || len(graph.Nodes)!=4 || graph.Truncated {
		t.Fatalf("global graph edges=%d nodes=%d truncated=%v err=%v: %+v",len(graph.Edges),len(graph.Nodes),graph.Truncated,err,graph)
	}
	byID:=map[string]relationships.GraphEdge{}
	for _,edge:=range graph.Edges{byID[edge.ID]=edge}
	if byID["hosts:"+itoaGraph(workload.Workload.ID)].Source.EntityID!=uint64(hypervisor.ID) ||
		byID["matches:"+itoaGraph(workload.Workload.ID)].Target.EntityID!=uint64(guest.ID) {
		t.Fatalf("missing native ownership and match: %+v",byID)
	}
	if byID["matches:"+itoaGraph(workload.Workload.ID)].Provenance!=relationships.ProvenanceSystem {
		t.Fatalf("exact-MAC provenance wrong: %+v",byID)
	}
	focus,err:=SelectRelationshipGraph(uint64(guest.ID),100)
	if err!=nil || len(focus.Edges)!=2 {
		t.Fatalf("guest one-hop graph=%+v err=%v",focus,err)
	}
	parent,err:=SelectRelationshipGraph(uint64(hypervisor.ID),100)
	if err!=nil || len(parent.Edges)!=1 || parent.Edges[0].Type!=relationships.TypeHosts {
		t.Fatalf("hypervisor one-hop graph=%+v err=%v",parent,err)
	}
	if _,err:=SelectRelationshipGraph(99999,100);!errors.Is(err,gorm.ErrRecordNotFound) {
		t.Fatalf("absent scoped Host=%v",err)
	}
	if _,err:=SelectRelationshipGraph(0,0);!errors.Is(err,relationships.ErrInvalidEndpoint) {
		t.Fatalf("invalid limit=%v",err)
	}
	// Historical-only hosts must never satisfy current ownership or match.
	if err:=db.Table(infrastructureWorkloadHostLinksTable).
		Where(`"WORKLOAD_ID" = ?`,workload.Workload.ID).
		Update("HOST_MAC","AA:BB:CC:DD:38:FF").Error;err!=nil{t.Fatal(err)}
	graph,err=SelectRelationshipGraph(0,100)
	if err!=nil {t.Fatal(err)}
	for _,edge:=range graph.Edges {
		if edge.Type==relationships.TypeMatchesHost {t.Fatalf("stale MAC created false MATCHES edge: %+v",edge)}
	}
	if err:=db.Table(infrastructureWorkloadHostLinksTable).
		Where(`"WORKLOAD_ID" = ?`,workload.Workload.ID).
		Update("HOST_MAC",guest.Mac).Error;err!=nil{t.Fatal(err)}
	// Workload retirement must retain source-owned graph evidence with
	// explicit retirement metadata.
	if err:=db.Table(infrastructureWorkloadsTable).
		Where(`"ID" = ?`,workload.Workload.ID).
		Update("RETIRED_AT","2026-10-18T10:00:00Z").Error;err!=nil{t.Fatal(err)}
	graph,err=SelectRelationshipGraph(0,100)
	if err!=nil {t.Fatal(err)}
	for _,edge:=range graph.Edges {
		if edge.Type==relationships.TypeHosts && edge.RetiredAt=="" {t.Fatal("retirement metadata not propagated")}
	}
	if err:=DeleteCurrentHostWithMetadata(guest);err!=nil{t.Fatal(err)}
	graph,err=SelectRelationshipGraph(0,100)
	if err!=nil || len(graph.Edges)!=1 || graph.Edges[0].Type!=relationships.TypeHosts {
		t.Fatalf("guest delete left orphan edges %+v err=%v",graph,err)
	}
}

func TestRelationshipGraphBoundedQueriesAndLegacyStaleSources(t *testing.T) {
	startSelectTestDB(t)
	hypervisor:=seedRelationshipHost(t,1,"AA:BB:CC:DD:38:C1")
	for i:=1;i<=8;i++{
		if _,err:=UpsertInfrastructureWorkload(hypervisor.Mac,models.InfrastructureWorkloadUpsert{
			NativeID:itoaGraph(uint(i)),WorkloadType:"container",Name:"container",Status:"stopped",Source:models.InfrastructureWorkloadSourceScriptImport,
		},"2026-10-18T09:00:00Z");err!=nil{t.Fatal(err)}
	}
	graph,err:=SelectRelationshipGraph(0,3)
	if err!=nil || len(graph.Edges)!=3 || !graph.Truncated {
		t.Fatalf("bounded graph %+v err=%v",graph,err)
	}
	again,err:=SelectRelationshipGraph(0,3)
	if err!=nil || len(again.Edges)!=len(graph.Edges){t.Fatalf("unstable query %+v err=%v",again,err)}
	for i:=range graph.Edges {
		if graph.Edges[i].ID!=again.Edges[i].ID{t.Fatalf("unstable edge order %+v vs %+v",graph.Edges,again.Edges)}
	}
	focused,err:=SelectRelationshipGraph(uint64(hypervisor.ID),3)
	if err!=nil || len(focused.Edges)!=3 || !focused.Truncated {
		t.Fatalf("scoped bounded graph %+v err=%v",focused,err)
	}
}

func itoaGraph(id uint) string {
	return fmt.Sprintf("%d",id)
}
