package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
)

func TestRelationshipGraphAPIProjectsSourceOwnedAndManualEvidence(t *testing.T) {
	router:=setupTestRouter(t)
	hypervisor:=seedHost(t,models.Host{Name:"pve",Mac:"AA:BB:CC:38:D4:01",DeviceType:"server"})
	guest:=seedHost(t,models.Host{Name:"guest",Mac:"AA:BB:CC:38:D4:02",DeviceType:"server"})
	other:=seedHost(t,models.Host{Name:"switch",Mac:"AA:BB:CC:38:D4:03"})
	record,err:=gdb.UpsertInfrastructureWorkload(hypervisor.Mac,models.InfrastructureWorkloadUpsert{
		NativeID:"101",WorkloadType:models.InfrastructureWorkloadTypeVM,Name:"guest vm",
		Status:models.InfrastructureWorkloadStatusRunning,Source:models.InfrastructureWorkloadSourceProxmoxAPI,
	},"2026-10-18T09:00:00Z")
	if err!=nil {t.Fatal(err)}
	if _,err:=gdb.SetInfrastructureWorkloadHostLink(record.Workload.ID,guest,
		models.InfrastructureWorkloadLinkSourceManual,"2026-10-18T09:01:00Z");err!=nil{t.Fatal(err)}
	created:=relationshipAPIRequest(router,http.MethodPost,"/api/relationships",relationshipAPIPayload(
		relationships.TypeManages,other.ID,guest.ID))
	if created.Code!=201 {t.Fatalf("manual create status=%d body=%s",created.Code,created.Body.String())}
	response:=getPath(router,"/api/relationships/graph")
	if response.Code!=200 {t.Fatalf("graph status=%d body=%s",response.Code,response.Body.String())}
	var graph relationships.Graph
	if err:=json.Unmarshal(response.Body.Bytes(),&graph);err!=nil{t.Fatal(err)}
	if len(graph.Edges)!=3 || len(graph.Nodes)!=4 || graph.Truncated {
		t.Fatalf("graph result %+v",graph)
	}
	kinds:=map[relationships.Type]relationships.GraphEdge{}
	for _,edge:=range graph.Edges{kinds[edge.Type]=edge}
	if kinds[relationships.TypeHosts].Source.EntityID!=uint64(hypervisor.ID) ||
		!kinds[relationships.TypeHosts].ReadOnly || kinds[relationships.TypeHosts].SourceDetail!="proxmox-api" {
		t.Fatalf("ownership %+v",kinds[relationships.TypeHosts])
	}
	if kinds[relationships.TypeMatchesHost].Target.EntityID!=uint64(guest.ID) ||
		!kinds[relationships.TypeMatchesHost].ReadOnly || kinds[relationships.TypeMatchesHost].SourceDetail!="manual" {
		t.Fatalf("matched Host %+v",kinds[relationships.TypeMatchesHost])
	}
	if kinds[relationships.TypeManages].ReadOnly || kinds[relationships.TypeManages].Provenance!="manual" {
		t.Fatalf("manual assertion %+v",kinds[relationships.TypeManages])
	}
	scoped:=getPath(router,fmt.Sprintf("/api/relationships/graph?hostId=%d",guest.ID))
	if scoped.Code!=200 || json.Unmarshal(scoped.Body.Bytes(),&graph)!=nil || len(graph.Edges)!=2 {
		t.Fatalf("guest incident graph status=%d body=%s",scoped.Code,scoped.Body.String())
	}
	parent:=getPath(router,fmt.Sprintf("/api/relationships/graph?hostId=%d",hypervisor.ID))
	if parent.Code!=200 || json.Unmarshal(parent.Body.Bytes(),&graph)!=nil || len(graph.Edges)!=1 || graph.Edges[0].Type!=relationships.TypeHosts {
		t.Fatalf("parent graph status=%d body=%s",parent.Code,parent.Body.String())
	}
	manualList:=getPath(router,fmt.Sprintf("/api/host/%d/relationships",guest.ID))
	if manualList.Code!=200 {t.Fatalf("existing manual API broken: %d",manualList.Code)}
	var manualRows []RelationshipResponse
	if err:=json.Unmarshal(manualList.Body.Bytes(),&manualRows);err!=nil || len(manualRows)!=1 || manualRows[0].Type!=relationships.TypeManages {
		t.Fatalf("manual response changed %+v err=%v",manualRows,err)
	}
}

func TestRelationshipGraphAPIValidatesHostAndEdgeLimit(t *testing.T) {
	router:=setupTestRouter(t)
	tests:=[]struct{path string;status int}{
		{"/api/relationships/graph",200},
		{"/api/relationships/graph?limit=1",200},
		{"/api/relationships/graph?limit=500",200},
		{"/api/relationships/graph?limit=0",400},
		{"/api/relationships/graph?limit=501",400},
		{"/api/relationships/graph?limit=nan",400},
		{"/api/relationships/graph?hostId=-1",400},
		{"/api/relationships/graph?hostId=0",400},
		{"/api/relationships/graph?hostId=999999",404},
	}
	for _,tt:=range tests{
		r:=getPath(router,tt.path)
		if r.Code!=tt.status{t.Fatalf("GET %s =%d want %d body=%s",tt.path,r.Code,tt.status,r.Body.String())}
	}
}
