package proxmoximport

import (
	"reflect"
	"slices"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
)

func TestValidateAndNormalizeSnapshot(t *testing.T) {
	snapshot := proxmoxsnapshot.Snapshot{
		SchemaVersion:    proxmoxsnapshot.SchemaVersion,
		CollectorVersion: " 1.0.0 ",
		Source:           proxmoxsnapshot.SourceScriptImport,
		CollectedAt:      "2026-09-19T12:00:00+03:00",
		Complete:         true,
		Node: proxmoxsnapshot.NodeSnapshot{
			Hostname:   " pve-1 ",
			PVEVersion: " pve-manager/9.2.10 ",
			Status:     " ONLINE ",
		},
		Workloads: []proxmoxsnapshot.WorkloadSnapshot{
			{
				NativeID:     "127",
				WorkloadType: "container",
				Name:         " yubal ",
				Status:       "running",
				Interfaces: []proxmoxsnapshot.InterfaceSnapshot{{
					Name:              "eth0",
					Mac:               "aa:bb:cc:dd:ee:70",
					Bridge:            " vmbr0 ",
					VLANTag:           "30",
					ConfiguredAddress: "10.4.1.70/24",
				}},
			},
			{
				NativeID:     "119",
				WorkloadType: "vm",
				Name:         "media",
				Status:       "stopped",
				Interfaces:   []proxmoxsnapshot.InterfaceSnapshot{},
			},
		},
	}

	normalized, err := ValidateAndNormalize(snapshot)
	if err != nil {
		t.Fatalf("ValidateAndNormalize: %v", err)
	}
	if normalized.CollectedAt != "2026-09-19T09:00:00Z" {
		t.Fatalf("CollectedAt = %q", normalized.CollectedAt)
	}
	if normalized.Node.Hostname != "pve-1" || normalized.Node.Status != "online" {
		t.Fatalf("node = %+v", normalized.Node)
	}
	if normalized.Workloads[0].NativeID != "127" || normalized.Workloads[1].NativeID != "119" {
		t.Fatalf("workload order = %+v", normalized.Workloads)
	}
	iface := normalized.Workloads[0].Interfaces[0]
	if iface.Mac != "AA:BB:CC:DD:EE:70" || iface.ConfiguredNetwork != "10.4.1.0/24" {
		t.Fatalf("normalized interface = %+v", iface)
	}
}

func TestValidateAndNormalizeRejectsInvalidContract(t *testing.T) {
	valid := validSnapshot()
	tests := []struct {
		name   string
		mutate func(*proxmoxsnapshot.Snapshot)
	}{
		{"schema", func(s *proxmoxsnapshot.Snapshot) { s.SchemaVersion = 99 }},
		{"source", func(s *proxmoxsnapshot.Snapshot) { s.Source = "api" }},
		{"time", func(s *proxmoxsnapshot.Snapshot) { s.CollectedAt = "yesterday" }},
		{"complete-errors", func(s *proxmoxsnapshot.Snapshot) { s.CollectionErrors = []string{"failed"} }},
		{"node", func(s *proxmoxsnapshot.Snapshot) { s.Node.Hostname = "" }},
		{"native-id", func(s *proxmoxsnapshot.Snapshot) { s.Workloads[0].NativeID = "00119" }},
		{"status", func(s *proxmoxsnapshot.Snapshot) { s.Workloads[0].Status = "paused" }},
		{"bad-mac", func(s *proxmoxsnapshot.Snapshot) { s.Workloads[0].Interfaces[0].Mac = "not-a-mac" }},
		{"bad-network", func(s *proxmoxsnapshot.Snapshot) { s.Workloads[0].Interfaces[0].ConfiguredNetwork = "10.5.0.0/24" }},
		{"bad-vlan", func(s *proxmoxsnapshot.Snapshot) { s.Workloads[0].Interfaces[0].VLANTag = "4095" }},
		{"duplicate-workload", func(s *proxmoxsnapshot.Snapshot) { s.Workloads = append(s.Workloads, s.Workloads[0]) }},
		{"duplicate-interface", func(s *proxmoxsnapshot.Snapshot) {
			s.Workloads[0].Interfaces = append(s.Workloads[0].Interfaces, s.Workloads[0].Interfaces[0])
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := valid
			snapshot.Workloads = append([]proxmoxsnapshot.WorkloadSnapshot(nil), valid.Workloads...)
			snapshot.Workloads[0].Interfaces = append([]proxmoxsnapshot.InterfaceSnapshot(nil), valid.Workloads[0].Interfaces...)
			tt.mutate(&snapshot)
			if _, err := ValidateAndNormalize(snapshot); err == nil {
				t.Fatal("invalid snapshot unexpectedly accepted")
			}
		})
	}
}

func TestBuildPreviewSeparatesAddsUpdatesRetiresAndManualConflicts(t *testing.T) {
	snapshot := validSnapshot()
	snapshot.Workloads = append(snapshot.Workloads,
		proxmoxsnapshot.WorkloadSnapshot{
			NativeID:     "120",
			WorkloadType: "vm",
			Name:         "new-vm",
			Status:       "running",
			Interfaces:   []proxmoxsnapshot.InterfaceSnapshot{},
		},
		proxmoxsnapshot.WorkloadSnapshot{
			NativeID:     "121",
			WorkloadType: "vm",
			Name:         "manual-collision",
			Status:       "running",
			Interfaces:   []proxmoxsnapshot.InterfaceSnapshot{},
		},
	)

	current := CurrentState{
		SourceState: &models.ProxmoxSourceState{
			HypervisorMac:   "AA:BB:CC:DD:EE:10",
			Source:          models.InfrastructureWorkloadSourceScriptImport,
			NodeHostname:    "pve-old",
			NodePVEVersion:  "pve-manager/9.1",
			NodeClusterName: "",
			NodeStatus:      "online",
		},
		ManagedHypervisor: &models.HypervisorProfile{
			Mac:      "AA:BB:CC:DD:EE:10",
			Platform: "proxmox-ve",
			NodeName: "managed-name",
			Version:  "manual-version",
		},
		Workloads: []models.InfrastructureWorkloadRecord{
			{
				Workload: models.InfrastructureWorkload{
					ID:           1,
					NativeID:     "119",
					WorkloadType: "vm",
					Name:         "old-media",
					Status:       "stopped",
					Source:       models.InfrastructureWorkloadSourceScriptImport,
				},
				Interfaces: []models.InfrastructureWorkloadInterface{{Name: "net0", Mac: "AA:BB:CC:DD:EE:19", Bridge: "vmbr1"}},
			},
			{
				Workload: models.InfrastructureWorkload{
					ID:           2,
					NativeID:     "118",
					WorkloadType: "container",
					Name:         "old-container",
					Status:       "running",
					Source:       models.InfrastructureWorkloadSourceScriptImport,
				},
			},
			{
				Workload: models.InfrastructureWorkload{
					ID:           3,
					NativeID:     "121",
					WorkloadType: "vm",
					Name:         "manual",
					Status:       "running",
					Source:       models.InfrastructureWorkloadSourceManual,
				},
			},
		},
	}

	preview, err := BuildPreview("AA:BB:CC:DD:EE:10", snapshot, current)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	if preview.ApplyAllowed {
		t.Fatal("preview with manual collision unexpectedly applyable")
	}
	if preview.Summary.Added != 1 || preview.Summary.Updated != 1 || preview.Summary.Retired != 1 || preview.Summary.Conflicts != 1 {
		t.Fatalf("summary = %+v", preview.Summary)
	}
	if preview.Node.Action != "update" || preview.Node.Before == nil {
		t.Fatalf("node diff = %+v", preview.Node)
	}
	if len(preview.ManagedConflicts) < 1 {
		t.Fatalf("managed conflicts = %+v", preview.ManagedConflicts)
	}
	if preview.PreviewToken == "" || preview.SnapshotDigest == "" {
		t.Fatalf("missing preview token/digest: %+v", preview)
	}
}

func TestBuildPreviewBlocksIncompleteSnapshotAndTokenChangesWithState(t *testing.T) {
	snapshot := validSnapshot()
	current := CurrentState{}
	preview1, err := BuildPreview("AA:BB:CC:DD:EE:10", snapshot, current)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	if !preview1.ApplyAllowed {
		t.Fatalf("complete conflict-free preview blocked: %+v", preview1.BlockedReasons)
	}

	changed := current
	changed.Workloads = []models.InfrastructureWorkloadRecord{{
		Workload: models.InfrastructureWorkload{
			ID:           55,
			NativeID:     "999",
			WorkloadType: "vm",
			Name:         "other",
			Status:       "running",
			Source:       models.InfrastructureWorkloadSourceManual,
		},
	}}
	preview2, err := BuildPreview("AA:BB:CC:DD:EE:10", snapshot, changed)
	if err != nil {
		t.Fatalf("BuildPreview changed: %v", err)
	}
	if preview1.PreviewToken == preview2.PreviewToken {
		t.Fatal("preview token did not change when current state changed")
	}

	snapshot.Complete = false
	snapshot.CollectionErrors = []string{"qemu guest list unavailable"}
	incomplete, err := BuildPreview("AA:BB:CC:DD:EE:10", snapshot, CurrentState{})
	if err != nil {
		t.Fatalf("BuildPreview incomplete: %v", err)
	}
	if incomplete.ApplyAllowed || len(incomplete.BlockedReasons) == 0 {
		t.Fatalf("incomplete preview = %+v", incomplete)
	}
}

func TestScriptAndAPISnapshotsNormalizeSemanticallyEquivalent(t *testing.T) {
	script := validSnapshot()
	script.Workloads[0].NodeName = "pve-1"
	api := script
	api.Source = proxmoxsnapshot.SourceProxmoxAPI
	api.Workloads = append([]proxmoxsnapshot.WorkloadSnapshot(nil), script.Workloads...)
	api.Workloads[0].Interfaces = append([]proxmoxsnapshot.InterfaceSnapshot(nil), script.Workloads[0].Interfaces...)

	scriptNormalized, err := ValidateAndNormalize(script)
	if err != nil {
		t.Fatalf("normalize script snapshot: %v", err)
	}
	apiNormalized, err := ValidateAndNormalize(api)
	if err != nil {
		t.Fatalf("normalize API snapshot: %v", err)
	}

	if scriptNormalized.Source != proxmoxsnapshot.SourceScriptImport || apiNormalized.Source != proxmoxsnapshot.SourceProxmoxAPI {
		t.Fatalf("source provenance script=%q api=%q", scriptNormalized.Source, apiNormalized.Source)
	}
	scriptNormalized.Source = ""
	apiNormalized.Source = ""
	if scriptNormalized.CollectorVersion != apiNormalized.CollectorVersion {
		t.Fatalf("collector version differs script=%q api=%q", scriptNormalized.CollectorVersion, apiNormalized.CollectorVersion)
	}
	if !reflect.DeepEqual(scriptNormalized, apiNormalized) {
		t.Fatalf("normalized semantic snapshots differ:\nscript=%+v\napi=%+v", scriptNormalized, apiNormalized)
	}
}

func TestAPISnapshotTransitionsExistingScriptWorkloadWithoutConflict(t *testing.T) {
	snapshot := validSnapshot()
	snapshot.Source = proxmoxsnapshot.SourceProxmoxAPI
	snapshot.Workloads[0].NodeName = "pve-1"

	current := CurrentState{
		Workloads: []models.InfrastructureWorkloadRecord{{
			Workload: models.InfrastructureWorkload{
				ID: 1, NativeID: "119", WorkloadType: "vm", NodeName: "pve-1",
				Name: "media", Status: "running", Source: models.InfrastructureWorkloadSourceScriptImport,
			},
			Interfaces: []models.InfrastructureWorkloadInterface{{
				Name: "net0", Mac: "AA:BB:CC:DD:EE:19", Bridge: "vmbr0", VLANTag: "20",
				ConfiguredAddress: "10.4.1.19/24", ConfiguredNetwork: "10.4.1.0/24",
			}},
		}},
	}

	preview, err := BuildPreview("AA:BB:CC:DD:EE:10", snapshot, current)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	if !preview.ApplyAllowed || preview.Summary.Conflicts != 0 || preview.Summary.Updated != 1 {
		t.Fatalf("API transition preview = %+v", preview)
	}
	if len(preview.Workloads) != 1 || !slices.Contains(preview.Workloads[0].Changes, "source") {
		t.Fatalf("API transition workload diff = %+v", preview.Workloads)
	}

	inputs, err := SnapshotWorkloadInputs(snapshot)
	if err != nil {
		t.Fatalf("SnapshotWorkloadInputs: %v", err)
	}
	if len(inputs) != 1 || inputs[0].Source != models.InfrastructureWorkloadSourceProxmoxAPI ||
		inputs[0].NodeName != "pve-1" {
		t.Fatalf("API workload inputs = %+v", inputs)
	}
}

func TestScriptPreviewDoesNotRetireAPIScopeWorkloads(t *testing.T) {
	snapshot := validSnapshot()
	snapshot.Workloads[0].NodeName = "pve-1"

	current := CurrentState{
		Workloads: []models.InfrastructureWorkloadRecord{
			{
				Workload: models.InfrastructureWorkload{
					ID: 1, NativeID: "119", WorkloadType: "vm", NodeName: "pve-1",
					Name: "media", Status: "running", Source: models.InfrastructureWorkloadSourceProxmoxAPI,
				},
				Interfaces: []models.InfrastructureWorkloadInterface{{
					Name: "net0", Mac: "AA:BB:CC:DD:EE:19", Bridge: "vmbr0", VLANTag: "20",
					ConfiguredAddress: "10.4.1.19/24", ConfiguredNetwork: "10.4.1.0/24",
				}},
			},
			{
				Workload: models.InfrastructureWorkload{
					ID: 2, NativeID: "220", WorkloadType: "vm", NodeName: "pve-2",
					Name: "remote-node-vm", Status: "running", Source: models.InfrastructureWorkloadSourceProxmoxAPI,
				},
			},
		},
	}

	preview, err := BuildPreview("AA:BB:CC:DD:EE:10", snapshot, current)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	if preview.Summary.Updated != 1 || preview.Summary.Retired != 0 || preview.Summary.Conflicts != 0 {
		t.Fatalf("script scope preview = %+v", preview.Summary)
	}
	for _, diff := range preview.Workloads {
		if diff.Key == "vm:220" && diff.Action == "retire" {
			t.Fatalf("node-local script preview retired API-managed cluster workload: %+v", diff)
		}
	}
}

func TestAPIPreviewRetiresMissingScriptWorkloadDuringMigration(t *testing.T) {
	snapshot := validSnapshot()
	snapshot.Source = proxmoxsnapshot.SourceProxmoxAPI
	snapshot.Workloads[0].NodeName = "pve-1"

	current := CurrentState{
		Workloads: []models.InfrastructureWorkloadRecord{
			{
				Workload: models.InfrastructureWorkload{
					ID: 1, NativeID: "119", WorkloadType: "vm",
					Name: "media", Status: "running", Source: models.InfrastructureWorkloadSourceScriptImport,
				},
				Interfaces: []models.InfrastructureWorkloadInterface{{
					Name: "net0", Mac: "AA:BB:CC:DD:EE:19", Bridge: "vmbr0", VLANTag: "20",
					ConfiguredAddress: "10.4.1.19/24", ConfiguredNetwork: "10.4.1.0/24",
				}},
			},
			{
				Workload: models.InfrastructureWorkload{
					ID: 2, NativeID: "109", WorkloadType: "container",
					Name: "removed-lxc", Status: "stopped", Source: models.InfrastructureWorkloadSourceScriptImport,
				},
			},
		},
	}

	preview, err := BuildPreview("AA:BB:CC:DD:EE:10", snapshot, current)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	if preview.Summary.Updated != 1 || preview.Summary.Retired != 1 || preview.Summary.Conflicts != 0 {
		t.Fatalf("API migration preview = %+v", preview.Summary)
	}
	foundRetire := false
	for _, diff := range preview.Workloads {
		if diff.Key == "container:109" && diff.Action == "retire" {
			foundRetire = true
		}
	}
	if !foundRetire {
		t.Fatalf("API migration did not retire genuinely missing script workload: %+v", preview.Workloads)
	}
}

func TestManagedVersionComparisonIgnoresPVEManagerPrefix(t *testing.T) {
	snapshot := validSnapshot()
	current := CurrentState{
		ManagedHypervisor: &models.HypervisorProfile{
			Mac:      "AA:BB:CC:DD:EE:10",
			Platform: "proxmox-ve",
			Version:  "9.2.10",
		},
	}

	preview, err := BuildPreview("AA:BB:CC:DD:EE:10", snapshot, current)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	for _, conflict := range preview.ManagedConflicts {
		if conflict.Field == "version" {
			t.Fatalf("equivalent PVE versions reported as conflict: %+v", conflict)
		}
	}
}

func validSnapshot() proxmoxsnapshot.Snapshot {
	return proxmoxsnapshot.Snapshot{
		SchemaVersion:    proxmoxsnapshot.SchemaVersion,
		CollectorVersion: proxmoxsnapshot.CollectorVersion,
		Source:           proxmoxsnapshot.SourceScriptImport,
		CollectedAt:      "2026-09-19T09:00:00Z",
		Complete:         true,
		Node: proxmoxsnapshot.NodeSnapshot{
			Hostname:   "pve-1",
			PVEVersion: "pve-manager/9.2.10",
			Status:     "online",
		},
		Workloads: []proxmoxsnapshot.WorkloadSnapshot{{
			NativeID:     "119",
			WorkloadType: "vm",
			Name:         "media",
			Status:       "running",
			Interfaces: []proxmoxsnapshot.InterfaceSnapshot{{
				Name:              "net0",
				Mac:               "AA:BB:CC:DD:EE:19",
				Bridge:            "vmbr0",
				VLANTag:           "20",
				ConfiguredAddress: "10.4.1.19/24",
				ConfiguredNetwork: "10.4.1.0/24",
			}},
		}},
	}
}
