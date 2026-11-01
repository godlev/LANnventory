package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
)

// Regression: bounded graph responses must not leak unrelated edges, must
// report truncation, and must stay consistent after manual mutation.
func TestRelationshipGraphAPIScopedLimitAndMutationLifecycle(t *testing.T) {
	router := setupTestRouter(t)
	a := seedHost(t, models.Host{Name: "focus", Mac: "AA:BB:CC:38:E6:01"})
	b := seedHost(t, models.Host{Name: "peer-b", Mac: "AA:BB:CC:38:E6:02"})
	c := seedHost(t, models.Host{Name: "peer-c", Mac: "AA:BB:CC:38:E6:03"})
	d := seedHost(t, models.Host{Name: "unrelated", Mac: "AA:BB:CC:38:E6:04"})

	create := func(kind relationships.Type, source, target int) uint64 {
		t.Helper()
		res := relationshipAPIRequest(router, http.MethodPost, "/api/relationships",
			relationshipAPIPayload(kind, source, target))
		if res.Code != http.StatusCreated {
			t.Fatalf("POST relationship = %d: %s", res.Code, res.Body.String())
		}
		var row RelationshipResponse
		if err := json.Unmarshal(res.Body.Bytes(), &row); err != nil { t.Fatal(err) }
		return row.ID
	}
	first := create(relationships.TypeManages, a.ID, b.ID)
	create(relationships.TypeDependsOn, c.ID, a.ID)
	create(relationships.TypeConnectedTo, b.ID, d.ID)

	graphAt := func(path string) relationships.Graph {
		t.Helper()
		res := getPath(router, path)
		if res.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", path, res.Code, res.Body.String())
		}
		var graph relationships.Graph
		if err := json.Unmarshal(res.Body.Bytes(), &graph); err != nil { t.Fatal(err) }
		return graph
	}
	scopedPath := fmt.Sprintf("/api/relationships/graph?hostId=%d&limit=1", a.ID)
	scoped := graphAt(scopedPath)
	if len(scoped.Edges) != 1 || len(scoped.Nodes) != 2 || !scoped.Truncated {
		t.Fatalf("scoped limited graph: %+v", scoped)
	}
	for _, edge := range scoped.Edges {
		if edge.Source.EntityID != uint64(a.ID) && edge.Target.EntityID != uint64(a.ID) {
			t.Fatalf("non-incident edge leaked into focused graph: %+v", edge)
		}
	}
	full := graphAt(fmt.Sprintf("/api/relationships/graph?hostId=%d&limit=500", a.ID))
	if len(full.Edges) != 2 || len(full.Nodes) != 3 || full.Truncated {
		t.Fatalf("unbounded focused graph: %+v", full)
	}
	global := graphAt("/api/relationships/graph?limit=500")
	if len(global.Edges) != 3 || len(global.Nodes) != 4 || global.Truncated {
		t.Fatalf("global graph: %+v", global)
	}
	for _, edge := range full.Edges {
		if edge.ReadOnly || edge.Provenance != relationships.ProvenanceManual {
			t.Fatalf("manual relationship unexpectedly source-owned: %+v", edge)
		}
	}
	if res := relationshipAPIRequest(router, http.MethodDelete,
		fmt.Sprintf("/api/relationships/%d", first), ""); res.Code != http.StatusNoContent {
		t.Fatalf("delete first edge: %d: %s", res.Code, res.Body.String())
	}
	after := graphAt(scopedPath)
	if len(after.Edges) != 1 || len(after.Nodes) != 2 || after.Truncated {
		t.Fatalf("focused graph after deletion: %+v", after)
	}
	if after.Edges[0].Type != relationships.TypeDependsOn {
		t.Fatalf("unexpected surviving edge: %+v", after.Edges[0])
	}
}
