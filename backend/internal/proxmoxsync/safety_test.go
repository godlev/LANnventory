package proxmoxsync

import (
	"testing"

	"github.com/godlev/LANnventory/internal/proxmoximport"
	"github.com/godlev/LANnventory/internal/proxmoxsnapshot"
	"github.com/godlev/LANnventory/internal/workloadmatch"
)

func TestAutomaticSafetyBlocksIncompleteAndPreviewConflicts(t *testing.T) {
	snapshot := proxmoxsnapshot.Snapshot{Complete: false}
	preview := proxmoximport.Preview{ApplyAllowed: false, BlockedReasons: []string{"source conflict"}}
	got := assessAutomaticSafety(snapshot, preview, nil)
	if got.Safe || len(got.Reasons) < 2 {
		t.Fatalf("assessment = %+v", got)
	}
}

func TestAutomaticSafetyBlocksAmbiguousExactMAC(t *testing.T) {
	got := assessAutomaticSafety(
		proxmoxsnapshot.Snapshot{Complete: true},
		proxmoximport.Preview{ApplyAllowed: true},
		[]automaticWorkloadMatch{{
			NativeID: "119", WorkloadType: "vm",
			Result: workloadmatch.Result{ExactAmbiguous: true},
		}},
	)
	if got.Safe {
		t.Fatalf("ambiguous exact match unexpectedly safe: %+v", got)
	}
}

func TestAutomaticSafetyBlocksUnresolvedIPConflictButHonorsRejection(t *testing.T) {
	conflict := workloadmatch.Candidate{HostID: 42, PossibleIPConflict: true}
	got := assessAutomaticSafety(
		proxmoxsnapshot.Snapshot{Complete: true},
		proxmoximport.Preview{ApplyAllowed: true},
		[]automaticWorkloadMatch{{
			NativeID: "119", WorkloadType: "vm",
			Result: workloadmatch.Result{Candidates: []workloadmatch.Candidate{conflict}},
		}},
	)
	if got.Safe {
		t.Fatalf("unresolved IP conflict unexpectedly safe: %+v", got)
	}

	conflict.Rejected = true
	got = assessAutomaticSafety(
		proxmoxsnapshot.Snapshot{Complete: true},
		proxmoximport.Preview{ApplyAllowed: true},
		[]automaticWorkloadMatch{{
			NativeID: "119", WorkloadType: "vm",
			Result: workloadmatch.Result{Candidates: []workloadmatch.Candidate{conflict}},
		}},
	)
	if !got.Safe {
		t.Fatalf("reviewed rejection should make weak conflict non-blocking: %+v", got)
	}
}

func TestAutomaticSafetyProtectsExistingExactLink(t *testing.T) {
	got := assessAutomaticSafety(
		proxmoxsnapshot.Snapshot{Complete: true},
		proxmoximport.Preview{ApplyAllowed: true},
		[]automaticWorkloadMatch{{
			NativeID: "127", WorkloadType: "container", ExistingExactHostID: 12,
			Result: workloadmatch.Result{},
		}},
	)
	if got.Safe {
		t.Fatalf("loss of deterministic exact link unexpectedly safe: %+v", got)
	}
}
