package proxmoxsnapshot

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSnapshotJSONContractDoesNotExposeRawConfigFields(t *testing.T) {
	snapshot := Snapshot{
		SchemaVersion:    SchemaVersion,
		CollectorVersion: CollectorVersion,
		Source:           SourceScriptImport,
		CollectedAt:      "2026-09-19T09:00:00Z",
		Complete:         true,
		Node: NodeSnapshot{
			Hostname:   "proxmox",
			PVEVersion: "pve-manager/9.2.10",
			Status:     "online",
		},
		Workloads: []WorkloadSnapshot{{
			NativeID:     "119",
			WorkloadType: "vm",
			Name:         "ubuntu-plex-immich",
			Status:       "running",
			Interfaces: []InterfaceSnapshot{{
				Name:              "net0",
				Mac:               "BC:24:11:A2:40:12",
				Bridge:            "vmbr0",
				VLANTag:           "20",
				ConfiguredAddress: "10.4.1.27/24",
				ConfiguredNetwork: "10.4.1.0/24",
			}},
		}},
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	body := string(payload)

	for _, required := range []string{
		`"schemaVersion":1`,
		`"collectorVersion":"1.0.0"`,
		`"source":"script-import"`,
		`"complete":true`,
		`"nativeId":"119"`,
		`"workloadType":"vm"`,
		`"interfaces"`,
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("snapshot JSON missing %s: %s", required, body)
		}
	}
	for _, forbidden := range []string{
		"password", "token", "secret", "privateKey", "rawConfig",
		"disk", "scsi0", "virtio0", "args", "userData",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("snapshot JSON exposed forbidden field %q: %s", forbidden, body)
		}
	}
}

func TestEmptySnapshotUsesWorkloadArray(t *testing.T) {
	snapshot := Snapshot{
		SchemaVersion:    SchemaVersion,
		CollectorVersion: CollectorVersion,
		Source:           SourceScriptImport,
		Complete:         true,
		Workloads:        []WorkloadSnapshot{},
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(payload), `"workloads":[]`) {
		t.Fatalf("empty workloads must be an array: %s", payload)
	}
}
