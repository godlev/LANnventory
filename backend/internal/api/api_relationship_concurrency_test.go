package api

import (
	"net/http"
	"sync"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
)

func TestRelationshipAPIConcurrentIdenticalCreatesReturnCreatedAndConflicts(t *testing.T) {
	router := setupTestRouter(t)
	source := seedHost(t, models.Host{Name: "source", Mac: "AA:BB:CC:38:A6:01"})
	target := seedHost(t, models.Host{Name: "target", Mac: "AA:BB:CC:38:A6:02"})
	payload := relationshipAPIPayload(relationships.TypeManages, source.ID, target.ID)

	const writers = 16
	start := make(chan struct{})
	statuses := make([]int, writers)
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			statuses[index] = relationshipAPIRequest(router, http.MethodPost, "/api/relationships", payload).Code
		}(i)
	}
	close(start)
	wg.Wait()

	created, conflicts := 0, 0
	for _, status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected concurrent POST status: %d", status)
		}
	}
	if created != 1 || conflicts != writers-1 {
		t.Fatalf("created=%d conflicts=%d, want 1 and %d", created, conflicts, writers-1)
	}
}
