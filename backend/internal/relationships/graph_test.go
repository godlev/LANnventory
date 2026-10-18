package relationships

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func graphFixture() GraphSnapshot {
	return GraphSnapshot{
		Hosts: []models.Host{
			{ID:1,Name:"pve",Mac:"AA:BB:CC:DD:01:01",IP:"10.4.1.6"},
			{ID:2,Name:"guest",Mac:"AA:BB:CC:DD:01:02",IP:"10.4.1.27"},
			{ID:3,Name:"router",Mac:"AA:BB:CC:DD:01:03",IP:"10.4.1.1"},
		},
		Workloads: []models.InfrastructureWorkload{
			{ID:17,HypervisorMac:"aa:bb:cc:dd:01:01",NativeID:"119",Name:"media",WorkloadType:"vm",Source:"proxmox-api",Status:"running"},
			{ID:18,HypervisorMac:"AA:BB:CC:DD:01:01",NativeID:"132",Name:"container",WorkloadType:"container",Source:"manual",RetiredAt:"2026-10-01T00:00:00Z"},
		},
		Links: []models.InfrastructureWorkloadHostLink{
			{WorkloadID:17,HostID:2,HostMac:"AA:BB:CC:DD:01:02",LinkSource:"exact-mac",LinkedAt:"2026-10-18T00:00:00Z"},
			{WorkloadID:18,HostID:2,HostMac:"AA:BB:CC:DD:01:02",LinkSource:"manual"},
		},
		Assertions: []models.RelationshipAssertion{
			{ID:30,RelationshipType:"manages",SourceEntityType:"host",SourceEntityID:3,TargetEntityType:"host",TargetEntityID:2,Provenance:"manual",Note:"admin"},
			{ID:31,RelationshipType:"depends_on",SourceEntityType:"host",SourceEntityID:2,TargetEntityType:"host",TargetEntityID:3,Provenance:"system",SourceDetail:"future-source",SourceInstance:"plugin-1"},
			{ID:32,RelationshipType:"hosts",SourceEntityType:"host",SourceEntityID:3,TargetEntityType:"workload",TargetEntityID:17,Provenance:"manual"}, // cannot shadow native owner
			{ID:33,RelationshipType:"connected_to",SourceEntityType:"host",SourceEntityID:3,TargetEntityType:"host",TargetEntityID:909,Provenance:"manual"}, // dangling
		},
	}
}

func TestGraphProjectionCombinesNativeOwnersMatchesAndManualClaims(t *testing.T) {
	got:=ProjectGraph(graphFixture(),0,100,false)
	if got.Truncated || len(got.Edges)!=6 || len(got.Nodes)!=5 {
		t.Fatalf("projection edges=%d nodes=%d truncated=%v: %+v",len(got.Edges),len(got.Nodes),got.Truncated,got)
	}
	edges:=map[string]GraphEdge{}
	for _,e:=range got.Edges {
		if _,seen:=edges[e.ID];seen{t.Fatalf("duplicate edge %q",e.ID)}
		edges[e.ID]=e
	}
	owned:=edges["hosts:17"]
	if owned.Type!=TypeHosts || owned.Source!= (Endpoint{EntityHost,1}) ||
		owned.Target!=(Endpoint{EntityWorkload,17}) || owned.Provenance!=ProvenanceProxmox ||
		owned.SourceDetail!="proxmox-api" || owned.Owner!="infrastructure_workloads" || !owned.ReadOnly {
		t.Fatalf("invalid Proxmox ownership %+v",owned)
	}
	inventoried:=edges["hosts:18"]
	if inventoried.Provenance!=ProvenanceManual || !inventoried.ReadOnly || inventoried.RetiredAt=="" {
		t.Fatalf("invalid manually inventoried, retired workload %+v",inventoried)
	}
	matched:=edges["matches:17"]
	if matched.Provenance!=ProvenanceSystem || matched.SourceDetail!="exact-mac" ||
		matched.Source!=(Endpoint{EntityWorkload,17}) || matched.Target!=(Endpoint{EntityHost,2}) ||
		!matched.ReadOnly || matched.Owner!="infrastructure_workload_host_links" {
		t.Fatalf("invalid exact-match projection %+v",matched)
	}
	if manualMatch:=edges["matches:18"];manualMatch.Provenance!=ProvenanceManual || !manualMatch.ReadOnly{
		t.Fatalf("manual Phase35 link must still be read-only from generic CRUD: %+v",manualMatch)
	}
	if manual:=edges["manual:30"];manual.Note!="admin" || manual.ReadOnly || manual.SourceLabel!="Manages" || manual.InverseLabel!="Managed by" {
		t.Fatalf("invalid user assertion %+v",manual)
	}
	if future:=edges["assertion:31"];future.Provenance!=ProvenanceSystem || !future.ReadOnly || future.SourceInstance!="plugin-1" {
		t.Fatalf("invalid source assertion %+v",future)
	}
	for _,id:=range []string{"manual:32","manual:33"} {
		if _,ok:=edges[id];ok{t.Fatalf("unexpected invalid/manual shadow edge %s",id)}
	}
	node:=map[string]GraphNode{}
	for _,n:=range got.Nodes {node[n.ID]=n}
	if node["workload:17"].WorkloadType!="vm" || node["host:2"].IP!="10.4.1.27" || node["workload:18"].RetiredAt==""{
		t.Fatalf("lost node metadata: %+v",node)
	}
	if _,ok:=node["host:17"];ok{t.Fatal("invented host for VM")}
}

func TestGraphProjectionScopedSingleHopAndDeterministic(t *testing.T) {
	guest:=ProjectGraph(graphFixture(),2,100,false)
	if len(guest.Edges)!=4 {
		t.Fatalf("guest edges=%d want 4: %+v",len(guest.Edges),guest.Edges)
	}
	for _,e:=range guest.Edges {
		if e.Source.EntityType!=EntityHost || e.Source.EntityID!=2 {
			if e.Target.EntityType!=EntityHost || e.Target.EntityID!=2 {
				t.Fatalf("not one-hop incident to host2: %+v",e)
			}
		}
		if e.Type==TypeHosts {t.Fatalf("guest scope unexpectedly traversed to hypervisor ownership: %+v",e)}
	}
	parent:=ProjectGraph(graphFixture(),1,100,false)
	if len(parent.Edges)!=2 {
		t.Fatalf("hypervisor owner edges=%d want2: %+v",len(parent.Edges),parent.Edges)
	}
	empty:=ProjectGraph(graphFixture(),999,100,false)
	if len(empty.Edges)!=0 || len(empty.Nodes)!=0 {t.Fatalf("missing Host scope leaked %+v",empty)}
	first:=ProjectGraph(graphFixture(),0,2,false)
	if len(first.Edges)!=2 || !first.Truncated {t.Fatalf("limit failed %+v",first)}
	for _,e:=range first.Edges {
		for _,v:=range []Endpoint{e.Source,e.Target} {
			found:=false
			for _,n:=range first.Nodes {if n.ID==v.Key(){found=true;break}}
			if !found {t.Fatalf("edge missing vertex %v",v)}
		}
	}
	reordered:=graphFixture()
	for i,j:=0,len(reordered.Workloads)-1;i<j;i,j=i+1,j-1 {reordered.Workloads[i],reordered.Workloads[j]=reordered.Workloads[j],reordered.Workloads[i]}
	for i,j:=0,len(reordered.Assertions)-1;i<j;i,j=i+1,j-1 {reordered.Assertions[i],reordered.Assertions[j]=reordered.Assertions[j],reordered.Assertions[i]}
	a,_:=json.Marshal(ProjectGraph(graphFixture(),0,100,false))
	b,_:=json.Marshal(ProjectGraph(reordered,0,100,false))
	if !reflect.DeepEqual(a,b){t.Fatalf("graph output order depends on input order\n%s\n%s",a,b)}
	if !ProjectGraph(graphFixture(),0,100,true).Truncated {t.Fatal("missing source-limit warning")}
}

func TestGraphProjectionSkipsStaleLinksAndAmbiguousHypervisors(t *testing.T) {
	s:=graphFixture()
	s.Links=append(s.Links,models.InfrastructureWorkloadHostLink{WorkloadID:99,HostID:2,HostMac:"AA:BB:CC:DD:01:02",LinkSource:"manual"})
	s.Links[0].HostMac="AA:BB:CC:DD:01:FF"
	got:=ProjectGraph(s,0,100,false)
	for _,e:=range got.Edges{
		if e.ID=="matches:17" || e.ID=="matches:99" {t.Fatalf("stale or orphan match projected: %+v",e)}
	}
	s.Hosts=append(s.Hosts,models.Host{ID:4,Mac:"AA:BB:CC:DD:01:01"})
	got=ProjectGraph(s,0,100,false)
	for _,e:=range got.Edges {
		if e.Type==TypeHosts{t.Fatalf("ambiguous hypervisor MAC must not fabricate ownership: %+v",e)}
	}
}
