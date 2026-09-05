package gdb

import (
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestHostMetadataMigrationCreatesSeparateTableOnly(t *testing.T) {
	startSelectTestDB(t)

	if !db.Migrator().HasTable(hostMetadataTable) {
		t.Fatal("host_metadata table was not migrated")
	}

	for _, table := range []string{"now", "history"} {
		for _, column := range []string{"OWNER", "LOCATION", "NOTES", "TAGS_JSON", "PINNED"} {
			if db.Table(table).Migrator().HasColumn(&models.Host{}, column) {
				t.Fatalf("%s unexpectedly has metadata column %s", table, column)
			}
		}
	}
}

func TestSelectCurrentHostsWithMetadataDefaults(t *testing.T) {
	startSelectTestDB(t)
	seedExportCurrentHost(t, models.Host{
		ID:    1,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		Known: 1,
		Now:   1,
	})

	hosts, ok := SelectCurrentHostsWithMetadata()
	if !ok {
		t.Fatal("SelectCurrentHostsWithMetadata failed")
	}
	if len(hosts) != 1 {
		t.Fatalf("hosts len = %d, want 1", len(hosts))
	}
	if hosts[0].Owner != "" || hosts[0].Location != "" || hosts[0].Notes != "" || hosts[0].Pinned {
		t.Fatalf("default metadata = %+v, want empty and unpinned", hosts[0])
	}
	if hosts[0].Tags == nil || len(hosts[0].Tags) != 0 {
		t.Fatalf("default Tags = %#v, want empty array", hosts[0].Tags)
	}
}

func TestHostMetadataUpsertAndBatchSelection(t *testing.T) {
	startSelectTestDB(t)

	owner := "Miroslav"
	location := "Office"
	notes := "Main Proxmox host"
	tags := []string{"server", "critical"}
	pinned := true
	metadata, err := UpsertHostMetadata("AA:BB:CC:DD:EE:10", models.HostMetadataUpdate{
		Owner:    &owner,
		Location: &location,
		Notes:    &notes,
		Tags:     &tags,
		Pinned:   &pinned,
	})
	if err != nil {
		t.Fatalf("UpsertHostMetadata: %v", err)
	}
	if metadata.Owner != owner || metadata.Location != location || metadata.Notes != notes || !metadata.Pinned {
		t.Fatalf("metadata fields = %+v, want submitted values", metadata)
	}
	assertStringSlice(t, models.DecodeMetadataTags(metadata.TagsJSON), tags, "stored tags")

	byMAC, err := SelectHostMetadataForMACs([]string{"AA:BB:CC:DD:EE:10", "AA:BB:CC:DD:EE:11"})
	if err != nil {
		t.Fatalf("SelectHostMetadataForMACs: %v", err)
	}
	if len(byMAC) != 1 || byMAC["AA:BB:CC:DD:EE:10"].Owner != owner {
		t.Fatalf("metadata batch result = %+v, want one matching row", byMAC)
	}
}

func TestCurrentHostUpdatePreservesHostMetadata(t *testing.T) {
	startSelectTestDB(t)
	host := models.Host{
		ID:    1,
		Name:  "router",
		IP:    "192.168.1.1",
		Mac:   "AA:BB:CC:DD:EE:01",
		Known: 1,
		Now:   1,
	}
	seedExportCurrentHost(t, host)

	owner := "Network Team"
	tags := []string{"gateway"}
	pinned := true
	if _, err := UpsertHostMetadata(host.Mac, models.HostMetadataUpdate{
		Owner:  &owner,
		Tags:   &tags,
		Pinned: &pinned,
	}); err != nil {
		t.Fatalf("UpsertHostMetadata: %v", err)
	}

	host.Name = "router-renamed"
	host.IP = "192.168.1.254"
	host.Now = 0
	if err := UpdateWithError("now", host); err != nil {
		t.Fatalf("scanner-style UpdateWithError: %v", err)
	}

	metadata, ok, err := SelectHostMetadataByMAC(host.Mac)
	if err != nil {
		t.Fatalf("SelectHostMetadataByMAC: %v", err)
	}
	if !ok || metadata.Owner != owner || !metadata.Pinned {
		t.Fatalf("metadata after current host update = %+v, ok=%v", metadata, ok)
	}
	assertStringSlice(t, models.DecodeMetadataTags(metadata.TagsJSON), tags, "preserved tags")
}

func TestMalformedMetadataTagsJSONDoesNotCrashHostRetrieval(t *testing.T) {
	startSelectTestDB(t)
	seedExportCurrentHost(t, models.Host{
		ID:    1,
		Name:  "router",
		Mac:   "AA:BB:CC:DD:EE:01",
		Known: 1,
		Now:   1,
	})

	if err := db.Table(hostMetadataTable).Save(&models.HostMetadata{
		Mac:      "AA:BB:CC:DD:EE:01",
		Owner:    "Network Team",
		TagsJSON: "not-json",
		Pinned:   true,
	}).Error; err != nil {
		t.Fatalf("seed malformed metadata: %v", err)
	}

	host, err := SelectHostWithMetadataByID(1)
	if err != nil {
		t.Fatalf("SelectHostWithMetadataByID: %v", err)
	}
	if host.Owner != "Network Team" || !host.Pinned {
		t.Fatalf("host metadata fields = %+v, want owner/pinned preserved", host)
	}
	if host.Tags == nil || len(host.Tags) != 0 {
		t.Fatalf("malformed TagsJSON exposed tags = %#v, want empty array", host.Tags)
	}
}

func assertStringSlice(t *testing.T, got, want []string, label string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s len = %d, want %d: got %#v want %#v", label, len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q; got %#v", label, i, got[i], want[i], got)
		}
	}
}
