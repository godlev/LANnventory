package correlation

import "errors"

const (
	DecisionConfirmed = "confirmed"
	DecisionRejected  = "rejected"
)

var ErrDecisionConflict = errors.New("correlation decision conflicts with existing identity relationships")

// DecisionPair is one explicit user decision between two canonical MAC identities.
// The pair is logically unordered; callers are expected to canonicalize it before
// persistence, but graph validation itself does not depend on lexical ordering.
type DecisionPair struct {
	MacA     string
	MacB     string
	Decision string
}

// ValidateDecisionGraph rejects a relationship set where a rejected pair is
// nevertheless connected through one or more confirmed edges. This prevents
// contradictory logical-device groups such as A=B, B=C, while A!=C.
func ValidateDecisionGraph(decisions []DecisionPair) error {
	parent := make(map[string]string)

	var find func(string) string
	find = func(value string) string {
		root, exists := parent[value]
		if !exists {
			parent[value] = value
			return value
		}
		if root == value {
			return value
		}
		parent[value] = find(root)
		return parent[value]
	}

	union := func(left, right string) {
		leftRoot := find(left)
		rightRoot := find(right)
		if leftRoot != rightRoot {
			parent[rightRoot] = leftRoot
		}
	}

	for _, decision := range decisions {
		if decision.MacA == "" || decision.MacB == "" || decision.MacA == decision.MacB {
			continue
		}
		find(decision.MacA)
		find(decision.MacB)
		if decision.Decision == DecisionConfirmed {
			union(decision.MacA, decision.MacB)
		}
	}

	for _, decision := range decisions {
		if decision.Decision != DecisionRejected || decision.MacA == "" || decision.MacB == "" {
			continue
		}
		if find(decision.MacA) == find(decision.MacB) {
			return ErrDecisionConflict
		}
	}

	return nil
}
