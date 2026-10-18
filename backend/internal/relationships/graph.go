package relationships

import (
	"sort"
	"strconv"
	"strings"

	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

const (
	DefaultGraphLimit = 100
	MaxGraphLimit = 500
)

// GraphSnapshot is a bounded, consistent inventory read. It contains current
// Hosts (not address history), real Workloads, existing Phase 35 match links,
// and source-owned relationship assertions. No network scanning occurs.
type GraphSnapshot struct {
	Hosts      []models.Host
	Workloads  []models.InfrastructureWorkload
	Links      []models.InfrastructureWorkloadHostLink
	Assertions []models.RelationshipAssertion
}

type GraphNode struct {
	ID           string     `json:"id"`
	EntityType   EntityType `json:"entityType"`
	EntityID     uint64     `json:"entityId"`
	Label        string     `json:"label"`
	Mac          string     `json:"mac,omitempty"`
	IP           string     `json:"ip,omitempty"`
	WorkloadType string     `json:"workloadType,omitempty"`
	NativeID     string     `json:"nativeId,omitempty"`
	Status       string     `json:"status,omitempty"`
	RetiredAt    string     `json:"retiredAt,omitempty"`
}

type GraphEdge struct {
	ID             string         `json:"id"`
	Type           Type           `json:"type"`
	Source         Endpoint       `json:"source"`
	Target         Endpoint       `json:"target"`
	Directionality Directionality `json:"directionality"`
	SourceLabel    string         `json:"sourceLabel"`
	InverseLabel   string         `json:"inverseLabel"`
	Provenance     Provenance     `json:"provenance"`
	SourceDetail   string         `json:"sourceDetail,omitempty"`
	SourceInstance string         `json:"sourceInstance,omitempty"`
	Owner          string         `json:"owner"`
	ReadOnly       bool           `json:"readOnly"`
	Note           string         `json:"note,omitempty"`
	RetiredAt      string         `json:"retiredAt,omitempty"`
	CreatedAt      string         `json:"createdAt,omitempty"`
	UpdatedAt      string         `json:"updatedAt,omitempty"`
}

type Graph struct {
	Nodes     []GraphNode `json:"nodes"`
	Edges     []GraphEdge `json:"edges"`
	Truncated bool        `json:"truncated"`
}

// ProjectGraph is deterministic and side-effect-free. FocusHostID==0 projects
// all supplied claims; otherwise it returns only edges incident to that
// current Host (one hop, never recursive traversal). DB callers must bound
// source collections and pass sourceTruncated for incomplete collections.
func ProjectGraph(s GraphSnapshot, focusHostID uint64, limit int, sourceTruncated bool) Graph {
	if limit < 1 || limit > MaxGraphLimit {
		limit = DefaultGraphLimit
	}
	hostsByID := make(map[uint64]models.Host, len(s.Hosts))
	hostsByMAC := make(map[string]uint64, len(s.Hosts))
	for _, h := range s.Hosts {
		if h.ID < 1 { continue }
		id := uint64(h.ID)
		hostsByID[id] = h
		if mac, err := identity.NormalizeMAC(h.Mac); err == nil {
			if previous, exists := hostsByMAC[mac]; exists && previous != id {
				hostsByMAC[mac] = 0 // ambiguous MAC must not invent ownership
			} else if !exists {
				hostsByMAC[mac] = id
			}
		}
	}
	workloadsByID := make(map[uint64]models.InfrastructureWorkload, len(s.Workloads))
	for _, w := range s.Workloads {
		if w.ID > 0 { workloadsByID[uint64(w.ID)] = w }
	}
	edges := make([]GraphEdge, 0, len(s.Workloads)+len(s.Links)+len(s.Assertions))
	appendEdge := func(id string, kind Type, source, target Endpoint, provenance Provenance, detail, instance, owner, note, retiredAt, created, updated string, readOnly bool) {
		_, _, err := ValidatePair(kind, source, target)
		if err != nil { return }
		if focusHostID > 0 && !(source.EntityType == EntityHost && source.EntityID == focusHostID) &&
			!(target.EntityType == EntityHost && target.EntityID == focusHostID) { return }
		def, ok := LookupType(kind)
		if !ok { return }
		edges = append(edges, GraphEdge{
			ID: id, Type: kind, Source: source, Target: target,
			Directionality: def.Directionality,
			SourceLabel: def.SourceLabel, InverseLabel: def.InverseLabel,
			Provenance: provenance, SourceDetail: detail, SourceInstance: instance,
			Owner: owner, ReadOnly: readOnly, Note: note, RetiredAt: retiredAt,
			CreatedAt: created, UpdatedAt: updated,
		})
	}

	// Explicit source assertions never replace native Proxmox associations.
	// An assertion using a reserved native predicate is ignored defensively.
	for _, row := range s.Assertions {
		if row.ID == 0 { continue }
		kind := Type(row.RelationshipType)
		if kind == TypeHosts || kind == TypeMatchesHost || !IsValidProvenance(Provenance(row.Provenance)) { continue }
		src := Endpoint{EntityType: EntityType(row.SourceEntityType), EntityID: row.SourceEntityID}
		dst := Endpoint{EntityType: EntityType(row.TargetEntityType), EntityID: row.TargetEntityID}
		if src.EntityType != EntityHost || dst.EntityType != EntityHost { continue } // 38A scope
		if _, ok := hostsByID[src.EntityID]; !ok { continue }
		if _, ok := hostsByID[dst.EntityID]; !ok { continue }
		provenance := Provenance(row.Provenance)
		if provenance == ProvenanceManual {
			def, ok := LookupType(kind)
			if !ok || !def.ManualAllowed { continue }
		}
		prefix := "assertion:"
		if provenance == ProvenanceManual { prefix = "manual:" }
		appendEdge(prefix+strconv.FormatUint(row.ID,10), kind, src, dst, provenance,
			row.SourceDetail, row.SourceInstance, "relationship_assertions", row.Note, "", row.CreatedAt, row.UpdatedAt,
			provenance != ProvenanceManual)
	}

	// Authoritative Host->Workload ownership from infrastructure_workloads.
	for _, workload := range s.Workloads {
		if workload.ID == 0 { continue }
		mac, err := identity.NormalizeMAC(workload.HypervisorMac)
		if err != nil { continue }
		hostID := hostsByMAC[mac]
		if hostID == 0 { continue }
		var provenance Provenance
		switch workload.Source {
		case models.InfrastructureWorkloadSourceManual:
			provenance = ProvenanceManual
		case models.InfrastructureWorkloadSourceScriptImport, models.InfrastructureWorkloadSourceProxmoxAPI:
			provenance = ProvenanceProxmox
		default:
			continue
		}
		appendEdge("hosts:"+strconv.FormatUint(uint64(workload.ID),10), TypeHosts,
			Endpoint{EntityHost, hostID}, Endpoint{EntityWorkload, uint64(workload.ID)},
			provenance, workload.Source, mac, "infrastructure_workloads", "",
			workload.RetiredAt, "", workload.UpdatedAt, true)
	}

	// Authoritative Workload->Host link. Require BOTH current Host ID and
	// current Host MAC to prevent stale matches after ID/MAC reuse.
	for _, link := range s.Links {
		workload, ok := workloadsByID[uint64(link.WorkloadID)]
		if !ok || link.WorkloadID == 0 || link.HostID <= 0 { continue }
		host, ok := hostsByID[uint64(link.HostID)]
		if !ok { continue }
		currentMAC, currentErr := identity.NormalizeMAC(host.Mac)
		recordedMAC, recordedErr := identity.NormalizeMAC(link.HostMac)
		if currentErr != nil || recordedErr != nil || currentMAC != recordedMAC { continue }
		var provenance Provenance
		switch link.LinkSource {
		case models.InfrastructureWorkloadLinkSourceManual:
			provenance = ProvenanceManual
		case models.InfrastructureWorkloadLinkSourceExactMAC:
			provenance = ProvenanceSystem
		default:
			continue
		}
		parentMAC, _ := identity.NormalizeMAC(workload.HypervisorMac)
		appendEdge("matches:"+strconv.FormatUint(uint64(link.WorkloadID),10), TypeMatchesHost,
			Endpoint{EntityWorkload, uint64(link.WorkloadID)}, Endpoint{EntityHost, uint64(link.HostID)},
			provenance, link.LinkSource, parentMAC, "infrastructure_workload_host_links", "",
			workload.RetiredAt, link.LinkedAt, link.UpdatedAt, true)
	}

	sort.Slice(edges,func(i,j int) bool {
		a,b:=edges[i],edges[j]
		if a.Type != b.Type {return a.Type < b.Type}
		if a.Source.EntityType != b.Source.EntityType {return a.Source.EntityType < b.Source.EntityType}
		if a.Source.EntityID != b.Source.EntityID {return a.Source.EntityID < b.Source.EntityID}
		if a.Target.EntityType != b.Target.EntityType {return a.Target.EntityType < b.Target.EntityType}
		if a.Target.EntityID != b.Target.EntityID {return a.Target.EntityID < b.Target.EntityID}
		return a.ID < b.ID
	})
	truncated := sourceTruncated || len(edges)>limit
	if len(edges)>limit {edges=edges[:limit]}
	nodesByKey:=make(map[string]GraphNode)
	for _, edge:=range edges {
		for _, endpoint:=range []Endpoint{edge.Source,edge.Target} {
			if _, ok:=nodesByKey[endpoint.Key()];ok {continue}
			switch endpoint.EntityType {
			case EntityHost:
				host,ok:=hostsByID[endpoint.EntityID]
				if !ok {continue}
				label:=strings.TrimSpace(host.Name)
				if label=="" {label=host.Mac}
				nodesByKey[endpoint.Key()]=GraphNode{
					ID:endpoint.Key(),EntityType:EntityHost,EntityID:endpoint.EntityID,
					Label:label,Mac:host.Mac,IP:host.IP,
				}
			case EntityWorkload:
				w,ok:=workloadsByID[endpoint.EntityID]
				if !ok {continue}
				label:=strings.TrimSpace(w.Name)
				if label=="" {label=w.NativeID}
				nodesByKey[endpoint.Key()]=GraphNode{
					ID:endpoint.Key(),EntityType:EntityWorkload,EntityID:endpoint.EntityID,
					Label:label,WorkloadType:w.WorkloadType,NativeID:w.NativeID,
					Status:w.Status,RetiredAt:w.RetiredAt,
				}
			}
		}
	}
	nodes:=make([]GraphNode,0,len(nodesByKey))
	for _, node:=range nodesByKey {nodes=append(nodes,node)}
	sort.Slice(nodes,func(i,j int)bool{
		if nodes[i].EntityType != nodes[j].EntityType {return nodes[i].EntityType<nodes[j].EntityType}
		return nodes[i].EntityID<nodes[j].EntityID
	})
	return Graph{Nodes:nodes,Edges:edges,Truncated:truncated}
}
