package backup

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/models"
)

func TestRelationshipAssertionsBackupFormatAndEmptyNormalization(t *testing.T) {
	assertion := models.RelationshipAssertion{
		ID: 44, RelationshipType: "manages",
		SourceEntityType: "host", SourceEntityID: 1,
		TargetEntityType: "host", TargetEntityID: 2,
		Provenance: "manual", SourceInstance: "", SourceDetail: "",
		Note: "admin", CreatedAt: "2026-10-18T00:00:00Z",
		UpdatedAt: "2026-10-18T00:00:00Z",
	}
	data := DataFromModels(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, []models.RelationshipAssertion{assertion})
	doc := NewDocument(data, "test", time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC))
	if doc.FormatVersion != 6 || len(doc.Data.RelationshipAssertions) != 1 {
		t.Fatalf("version or assertions mismatch: %+v", doc)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var parsed Document
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Data.RelationshipAssertions[0].ID != 44 || parsed.Data.RelationshipAssertions[0].Provenance != "manual" {
		t.Fatalf("backup relationship lost: %+v", parsed.Data.RelationshipAssertions)
	}
	legacy := NewDocument(Data{}, "old", time.Now())
	if legacy.Data.RelationshipAssertions == nil || len(legacy.Data.RelationshipAssertions) != 0 {
		t.Fatalf("old/empty normalized data = %+v", legacy.Data)
	}
}
