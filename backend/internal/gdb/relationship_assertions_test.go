package gdb

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/godlev/LANnventory/internal/backup"
	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/relationships"
	"gorm.io/gorm"
)

func seedRelationshipHost(t *testing.T, id int, mac string) models.Host {
	t.Helper()
	host := models.Host{ID: id, Mac: mac, Name: "relationship-host", Now: 1}
	if err := UpdateWithError("now", host); err != nil {
		t.Fatalf("seed host %d: %v", id, err)
	}
	return host
}

func manualRelationship(kind relationships.Type, sourceID, targetID uint64, note string) relationships.ManualCreateInput {
	return relationships.ManualCreateInput{
		Type: kind,
		Source: relationships.Endpoint{EntityType: relationships.EntityHost, EntityID: sourceID},
		Target: relationships.Endpoint{EntityType: relationships.EntityHost, EntityID: targetID},
		Note: note,
	}
}

func TestRelationshipMigrationCreatesUniqueIndexesAndIsIdempotent(t *testing.T) {
	startSelectTestDB(t)
	if !db.Migrator().HasTable(relationshipAssertionsTable) {
		t.Fatalf("%s missing", relationshipAssertionsTable)
	}
	for _, column := range []string{"ID", "RELATIONSHIP_TYPE", "SOURCE_ENTITY_TYPE", "SOURCE_ENTITY_ID", "TARGET_ENTITY_TYPE", "TARGET_ENTITY_ID", "PROVENANCE", "SOURCE_INSTANCE", "SOURCE_DETAIL", "NOTE", "CREATED_AT", "UPDATED_AT"} {
		if !db.Table(relationshipAssertionsTable).Migrator().HasColumn(&models.RelationshipAssertion{}, column) {
			t.Fatalf("relationship assertion missing %s", column)
		}
	}
	if !db.Table(relationshipAssertionsTable).Migrator().HasIndex(&models.RelationshipAssertion{}, "idx_relationship_assertion_identity") {
		t.Fatal("missing relationship uniqueness index")
	}
	if err := migrate(db); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
}

func TestManualRelationshipPersistenceUniqueAndProvenanceProtected(t *testing.T) {
	startSelectTestDB(t)
	seedRelationshipHost(t, 1, "AA:BB:CC:DD:38:01")
	seedRelationshipHost(t, 2, "AA:BB:CC:DD:38:02")

	first, err := CreateManualRelationship(manualRelationship(relationships.TypeManages, 1, 2, "  managed  "))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if first.ID == 0 || first.Provenance != "manual" || first.Note != "managed" || first.SourceInstance != "" {
		t.Fatalf("saved = %+v", first)
	}
	if _, err := CreateManualRelationship(manualRelationship(relationships.TypeManages, 1, 2, "replacement")); !errors.Is(err, ErrRelationshipAlreadyExists) {
		t.Fatalf("duplicate = %v, want ErrRelationshipAlreadyExists", err)
	}
	rows, err := ListManualRelationshipsByHost(2)
	if err != nil || len(rows) != 1 || rows[0].Note != "managed" {
		t.Fatalf("duplicate modified original rows=%+v err=%v", rows, err)
	}
	reverse, err := CreateManualRelationship(manualRelationship(relationships.TypeManages, 2, 1, "reverse"))
	if err != nil {
		t.Fatalf("directed reverse: %v", err)
	}
	if reverse.ID == first.ID {
		t.Fatalf("reverse collision: %+v", reverse)
	}

	if _, err := CreateManualRelationship(manualRelationship(relationships.TypeConnectedTo, 1, 9, "")); !errors.Is(err, relationships.ErrEntityNotFound) {
		t.Fatalf("missing current target: %v", err)
	}
	if _, err := CreateManualRelationship(manualRelationship(relationships.TypeMatchesHost, 1, 2, "")); err == nil {
		t.Fatal("reserved HOSTS/MATCHES_HOST edge was accepted")
	}

	updated, err := UpdateManualRelationshipNote(first.ID, "  revised\nnote ")
	if err != nil || updated.Note != "revised\nnote" {
		t.Fatalf("manual note update=%+v err=%v", updated, err)
	}
	if _, err := UpdateManualRelationshipNote(first.ID, strings.Repeat("x", 501)); !errors.Is(err, relationships.ErrNoteTooLong) {
		t.Fatalf("invalid note update: %v", err)
	}

	system := models.RelationshipAssertion{
		RelationshipType: string(relationships.TypeConnectedTo),
		SourceEntityType: "host", SourceEntityID: 1,
		TargetEntityType: "host", TargetEntityID: 2,
		Provenance: "system", SourceInstance: "source-test",
		CreatedAt: "2026-10-18T00:00:00Z", UpdatedAt: "2026-10-18T00:00:00Z",
	}
	if err := db.Table(relationshipAssertionsTable).Create(&system).Error; err != nil {
		t.Fatalf("different provenance should coexist: %v", err)
	}
	if err := DeleteManualRelationship(system.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("manual deletion touched system-owned claim: %v", err)
	}
	if _, err := UpdateManualRelationshipNote(system.ID, "cannot change"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("manual note update touched system-owned claim: %v", err)
	}
	if err := DeleteManualRelationship(first.ID); err != nil {
		t.Fatalf("delete manual: %v", err)
	}
	if err := DeleteManualRelationship(first.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("duplicate delete: %v", err)
	}
	// Reopening must retain the opposite directed claim and system assertion.
	if err := Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := StartErr(); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	rows, err = ListManualRelationshipsByHost(1)
	if err != nil || len(rows) != 1 || rows[0].ID != reverse.ID {
		t.Fatalf("persisted manual assertions=%+v err=%v", rows, err)
	}
	var systemCount int64
	if err := db.Table(relationshipAssertionsTable).Where(`"ID" = ?`, system.ID).Count(&systemCount).Error; err != nil || systemCount != 1 {
		t.Fatalf("system-owned claim missing after reopen count=%d err=%v", systemCount, err)
	}
}

func TestHostDeletionPrunesBothDirectionsButPreservesUnrelated(t *testing.T) {
	startSelectTestDB(t)
	a := seedRelationshipHost(t, 1, "AA:BB:CC:DD:38:11")
	seedRelationshipHost(t, 2, "AA:BB:CC:DD:38:12")
	seedRelationshipHost(t, 3, "AA:BB:CC:DD:38:13")
	for _, pair := range [][2]uint64{{1, 2}, {3, 1}, {2, 3}} {
		if _, err := CreateManualRelationship(manualRelationship(relationships.TypeDependsOn, pair[0], pair[1], "")); err != nil {
			t.Fatal(err)
		}
	}
	if err := DeleteCurrentHostWithMetadata(a); err != nil {
		t.Fatalf("delete current host: %v", err)
	}
	for _, id := range []uint64{1, 2, 3} {
		rows, err := ListManualRelationshipsByHost(id)
		if err != nil {
			t.Fatal(err)
		}
		if (id == 1 && len(rows) != 0) || (id == 2 && len(rows) != 1) || (id == 3 && len(rows) != 1) {
			t.Fatalf("host %d relations after cleanup: %+v", id, rows)
		}
	}
	// Failing a later deletion step must rollback relationship cleanup too.
	// SQLite trigger is local to this test DB; no production schema changes.
	a = seedRelationshipHost(t, 4, "AA:BB:CC:DD:38:14")
	if _, err := CreateManualRelationship(manualRelationship(relationships.TypeConnectedTo, 4, 2, "")); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER test_block_host_delete BEFORE DELETE ON "now" WHEN OLD."ID" = 4 BEGIN SELECT RAISE(ABORT, 'blocked'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := DeleteCurrentHostWithMetadata(a); err == nil {
		t.Fatal("expected failed host deletion")
	}
	rows, err := ListManualRelationshipsByHost(4)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rollback did not restore assertion rows=%+v err=%v", rows, err)
	}
	if len(SelectByMAC("now", a.Mac)) != 1 {
		t.Fatal("host deletion did not rollback")
	}
}

func TestLogicalBackupV6IncludesManualAssertionsAndLegacyJSONRemainsDecodable(t *testing.T) {
	startSelectTestDB(t)
	seedRelationshipHost(t, 1, "AA:BB:CC:DD:38:21")
	seedRelationshipHost(t, 2, "AA:BB:CC:DD:38:22")
	saved, err := CreateManualRelationship(manualRelationship(relationships.TypeConnectedTo, 1, 2, "switch port 8"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := ExportData()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if backup.FormatVersion != 6 || len(data.RelationshipAssertions) != 1 {
		t.Fatalf("format=%d exported assertions=%+v", backup.FormatVersion, data.RelationshipAssertions)
	}
	claim := data.RelationshipAssertions[0]
	if claim.ID != saved.ID || claim.Provenance != "manual" || claim.Note != "switch port 8" ||
		claim.SourceEntityType != "host" || claim.TargetEntityID != 2 {
		t.Fatalf("exported claim=%+v", claim)
	}
	doc := backup.NewDocument(data, "test", time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC))
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var decoded backup.Document
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.FormatVersion != 6 || len(decoded.Data.RelationshipAssertions) != 1 {
		t.Fatalf("backup roundtrip: %+v", decoded)
	}
	var v5 backup.Document
	if err := json.Unmarshal([]byte(`{"format":"lannventory-backup","formatVersion":5,"data":{"currentHosts":[]}}`), &v5); err != nil {
		t.Fatalf("decode legacy v5: %v", err)
	}
	if v5.FormatVersion != 5 || len(v5.Data.RelationshipAssertions) != 0 {
		t.Fatalf("legacy backup corrupted: %+v", v5)
	}
}
