package gdb

import (
	"errors"
	"testing"

	"github.com/godlev/LANnventory/internal/correlation"
	"github.com/godlev/LANnventory/internal/models"
)

func TestIdentityCorrelationDecisionCanonicalPairAndUpdate(t *testing.T) {
	startSelectTestDB(t)
	macA := "02:AA:BB:CC:DD:10"
	macB := "06:AA:BB:CC:DD:20"

	created, err := SetIdentityCorrelationDecision(macB, macA, models.IdentityCorrelationConfirmed, "2026-09-17 18:00:00")
	if err != nil {
		t.Fatalf("SetIdentityCorrelationDecision create: %v", err)
	}
	if created.MacA != macA || created.MacB != macB {
		t.Fatalf("canonical pair = %s/%s, want %s/%s", created.MacA, created.MacB, macA, macB)
	}
	if created.Decision != models.IdentityCorrelationConfirmed || created.CreatedAt != "2026-09-17 18:00:00" || created.UpdatedAt != created.CreatedAt {
		t.Fatalf("created decision = %+v", created)
	}

	updated, err := SetIdentityCorrelationDecision(macA, macB, models.IdentityCorrelationRejected, "2026-09-17 18:05:00")
	if err != nil {
		t.Fatalf("SetIdentityCorrelationDecision update: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("updated ID = %d, want %d", updated.ID, created.ID)
	}
	if updated.Decision != models.IdentityCorrelationRejected || updated.CreatedAt != created.CreatedAt || updated.UpdatedAt != "2026-09-17 18:05:00" {
		t.Fatalf("updated decision = %+v", updated)
	}

	selected, found, err := SelectIdentityCorrelationDecision(macB, macA)
	if err != nil || !found {
		t.Fatalf("SelectIdentityCorrelationDecision found=%v err=%v", found, err)
	}
	if selected.Decision != models.IdentityCorrelationRejected {
		t.Fatalf("selected decision = %+v", selected)
	}

	rows, err := SelectIdentityCorrelationDecisionsForMAC(macA)
	if err != nil || len(rows) != 1 || rows[0].ID != created.ID {
		t.Fatalf("SelectIdentityCorrelationDecisionsForMAC = %+v err=%v", rows, err)
	}
}

func TestIdentityCorrelationDecisionValidationAndDelete(t *testing.T) {
	startSelectTestDB(t)
	macA := "02:AA:BB:CC:DD:31"
	macB := "06:AA:BB:CC:DD:32"

	if _, err := SetIdentityCorrelationDecision(macA, macA, models.IdentityCorrelationConfirmed, "2026-09-17 18:00:00"); err == nil {
		t.Fatal("expected same-MAC pair to be rejected")
	}
	if _, err := SetIdentityCorrelationDecision(macA, macB, "maybe", "2026-09-17 18:00:00"); err == nil {
		t.Fatal("expected invalid decision to be rejected")
	}
	if _, err := SetIdentityCorrelationDecision(macA, macB, models.IdentityCorrelationConfirmed, ""); err == nil {
		t.Fatal("expected empty timestamp to be rejected")
	}

	if _, err := SetIdentityCorrelationDecision(macA, macB, models.IdentityCorrelationConfirmed, "2026-09-17 18:01:00"); err != nil {
		t.Fatalf("seed decision: %v", err)
	}
	if err := DeleteIdentityCorrelationDecision(macB, macA); err != nil {
		t.Fatalf("DeleteIdentityCorrelationDecision: %v", err)
	}
	if _, found, err := SelectIdentityCorrelationDecision(macA, macB); err != nil || found {
		t.Fatalf("decision should be deleted, found=%v err=%v", found, err)
	}
}

func TestIdentityCorrelationDecisionRejectsTransitiveConflict(t *testing.T) {
	startSelectTestDB(t)
	macA := "02:AA:BB:CC:DD:61"
	macB := "06:AA:BB:CC:DD:62"
	macC := "0A:AA:BB:CC:DD:63"

	if _, err := SetIdentityCorrelationDecision(macA, macB, models.IdentityCorrelationConfirmed, "2026-09-17 18:20:00"); err != nil {
		t.Fatalf("confirm A/B: %v", err)
	}
	if _, err := SetIdentityCorrelationDecision(macB, macC, models.IdentityCorrelationConfirmed, "2026-09-17 18:21:00"); err != nil {
		t.Fatalf("confirm B/C: %v", err)
	}
	if _, err := SetIdentityCorrelationDecision(macA, macC, models.IdentityCorrelationRejected, "2026-09-17 18:22:00"); !errors.Is(err, correlation.ErrDecisionConflict) {
		t.Fatalf("reject A/C error = %v, want ErrDecisionConflict", err)
	}
	if _, found, err := SelectIdentityCorrelationDecision(macA, macC); err != nil || found {
		t.Fatalf("conflicting A/C decision should not persist, found=%v err=%v", found, err)
	}
}

func TestIdentityCorrelationDecisionRejectsMergeAcrossRejectedBoundary(t *testing.T) {
	startSelectTestDB(t)
	macA := "02:AA:BB:CC:DD:71"
	macB := "06:AA:BB:CC:DD:72"
	macC := "0A:AA:BB:CC:DD:73"

	if _, err := SetIdentityCorrelationDecision(macA, macB, models.IdentityCorrelationRejected, "2026-09-17 18:30:00"); err != nil {
		t.Fatalf("reject A/B: %v", err)
	}
	if _, err := SetIdentityCorrelationDecision(macB, macC, models.IdentityCorrelationConfirmed, "2026-09-17 18:31:00"); err != nil {
		t.Fatalf("confirm B/C: %v", err)
	}
	if _, err := SetIdentityCorrelationDecision(macA, macC, models.IdentityCorrelationConfirmed, "2026-09-17 18:32:00"); !errors.Is(err, correlation.ErrDecisionConflict) {
		t.Fatalf("confirm A/C error = %v, want ErrDecisionConflict", err)
	}
}

func TestIdentityCorrelationDecisionSurvivesCurrentHostDeletion(t *testing.T) {
	startSelectTestDB(t)
	macA := "02:AA:BB:CC:DD:41"
	macB := "06:AA:BB:CC:DD:42"

	if err := UpdateWithError("now", models.Host{
		Name: "phone-old", Iface: "eth0", IP: "10.4.1.51", Mac: macA,
		Date: "2026-09-17 18:00:00", Known: 1, Now: 1, DeviceType: "phone",
	}); err != nil {
		t.Fatalf("seed current host: %v", err)
	}
	hosts := SelectByMAC("now", macA)
	if len(hosts) != 1 {
		t.Fatalf("seeded hosts = %+v", hosts)
	}

	decision, err := SetIdentityCorrelationDecision(macA, macB, models.IdentityCorrelationConfirmed, "2026-09-17 18:02:00")
	if err != nil {
		t.Fatalf("seed decision: %v", err)
	}
	if err := DeleteCurrentHostWithMetadata(hosts[0]); err != nil {
		t.Fatalf("DeleteCurrentHostWithMetadata: %v", err)
	}

	selected, found, err := SelectIdentityCorrelationDecision(macA, macB)
	if err != nil || !found {
		t.Fatalf("decision lost after host deletion: found=%v err=%v", found, err)
	}
	if selected.ID != decision.ID || selected.Decision != models.IdentityCorrelationConfirmed {
		t.Fatalf("retained decision = %+v, want %+v", selected, decision)
	}
}

func TestIdentityCorrelationDecisionMigrationIsIdempotent(t *testing.T) {
	startSelectTestDB(t)
	macA := "02:AA:BB:CC:DD:51"
	macB := "06:AA:BB:CC:DD:52"

	created, err := SetIdentityCorrelationDecision(macA, macB, models.IdentityCorrelationConfirmed, "2026-09-17 18:10:00")
	if err != nil {
		t.Fatalf("seed decision: %v", err)
	}

	activeDB, release, err := acquireDB()
	if err != nil {
		t.Fatalf("acquireDB: %v", err)
	}
	if err := migrate(activeDB); err != nil {
		release()
		t.Fatalf("second migrate: %v", err)
	}
	release()

	selected, found, err := SelectIdentityCorrelationDecision(macA, macB)
	if err != nil || !found {
		t.Fatalf("decision missing after repeated migrate: found=%v err=%v", found, err)
	}
	if selected.ID != created.ID || selected.Decision != created.Decision {
		t.Fatalf("decision changed after repeated migrate: got %+v want %+v", selected, created)
	}
}
