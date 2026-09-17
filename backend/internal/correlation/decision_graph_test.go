package correlation

import (
	"errors"
	"testing"
)

func TestValidateDecisionGraphAllowsConsistentConfirmedGroup(t *testing.T) {
	decisions := []DecisionPair{
		{MacA: "A", MacB: "B", Decision: DecisionConfirmed},
		{MacA: "B", MacB: "C", Decision: DecisionConfirmed},
	}
	if err := ValidateDecisionGraph(decisions); err != nil {
		t.Fatalf("ValidateDecisionGraph() error = %v", err)
	}
}

func TestValidateDecisionGraphRejectsTransitiveContradiction(t *testing.T) {
	decisions := []DecisionPair{
		{MacA: "A", MacB: "B", Decision: DecisionConfirmed},
		{MacA: "B", MacB: "C", Decision: DecisionConfirmed},
		{MacA: "A", MacB: "C", Decision: DecisionRejected},
	}
	if err := ValidateDecisionGraph(decisions); !errors.Is(err, ErrDecisionConflict) {
		t.Fatalf("ValidateDecisionGraph() error = %v, want ErrDecisionConflict", err)
	}
}

func TestValidateDecisionGraphRejectsMergeAcrossRejectedBoundary(t *testing.T) {
	decisions := []DecisionPair{
		{MacA: "A", MacB: "B", Decision: DecisionRejected},
		{MacA: "B", MacB: "C", Decision: DecisionConfirmed},
		{MacA: "A", MacB: "C", Decision: DecisionConfirmed},
	}
	if err := ValidateDecisionGraph(decisions); !errors.Is(err, ErrDecisionConflict) {
		t.Fatalf("ValidateDecisionGraph() error = %v, want ErrDecisionConflict", err)
	}
}

func TestValidateDecisionGraphAllowsRejectedPairAcrossSeparateGroups(t *testing.T) {
	decisions := []DecisionPair{
		{MacA: "A", MacB: "B", Decision: DecisionConfirmed},
		{MacA: "C", MacB: "D", Decision: DecisionConfirmed},
		{MacA: "A", MacB: "C", Decision: DecisionRejected},
	}
	if err := ValidateDecisionGraph(decisions); err != nil {
		t.Fatalf("ValidateDecisionGraph() error = %v", err)
	}
}
