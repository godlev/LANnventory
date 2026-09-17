package correlation

import (
	"sort"

	"github.com/godlev/LANnventory/internal/identity"
)

// ConfirmedGroup returns the deterministic confirmed connected component for
// one MAC identity. It is a read-only projection over explicit user decisions;
// it does not create a persistent device or rewrite host observations.
func ConfirmedGroup(target string, decisions []DecisionPair) []string {
	target = identity.MACKey(target)
	if target == "" {
		return nil
	}

	adjacency := make(map[string][]string)
	for _, decision := range decisions {
		if decision.Decision != DecisionConfirmed {
			continue
		}
		left := identity.MACKey(decision.MacA)
		right := identity.MACKey(decision.MacB)
		if left == "" || right == "" || left == right {
			continue
		}
		adjacency[left] = append(adjacency[left], right)
		adjacency[right] = append(adjacency[right], left)
	}

	seen := map[string]struct{}{target: {}}
	queue := []string{target}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if _, exists := seen[next]; exists {
				continue
			}
			seen[next] = struct{}{}
			queue = append(queue, next)
		}
	}

	members := make([]string, 0, len(seen))
	for mac := range seen {
		members = append(members, mac)
	}
	sort.Strings(members)
	return members
}
