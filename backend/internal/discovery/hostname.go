package discovery

import (
	"context"
	"net"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/models"
)

// HostnameObservation is one set of locally discovered hostnames with provenance.
type HostnameObservation struct {
	Source string
	Values []string
}

var (
	hostnameSourceTimeout = 800 * time.Millisecond
	reverseLookup         = func(ctx context.Context, address string) ([]string, error) {
		return net.DefaultResolver.LookupAddr(ctx, address)
	}
	localCommand = runLocalCommand
)

type hostnameLookup struct {
	source string
	lookup func(context.Context, string) []string
}

// LocalHostnames performs only local/system discovery: reverse DNS through the
// configured resolver, the system NSS resolver via getent, and optional Avahi
// address resolution when avahi-resolve-address is installed. Sources run in
// parallel under individual hard timeouts. Missing tools and lookup failures are
// normal best-effort outcomes and are not returned as scanner failures.
func LocalHostnames(parent context.Context, address string) []HostnameObservation {
	address = strings.TrimSpace(address)
	if net.ParseIP(address) == nil {
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}

	lookups := []hostnameLookup{
		{source: models.DiscoverySourceReverseDNS, lookup: lookupReverseDNS},
		{source: models.DiscoverySourceSystemResolver, lookup: lookupGetent},
		{source: models.DiscoverySourceMDNS, lookup: lookupAvahi},
	}

	results := make(chan HostnameObservation, len(lookups))
	for _, item := range lookups {
		item := item
		go func() {
			ctx, cancel := context.WithTimeout(parent, hostnameSourceTimeout)
			defer cancel()
			results <- HostnameObservation{
				Source: item.source,
				Values: cleanHostnames(item.lookup(ctx, address)),
			}
		}()
	}

	bySource := make(map[string][]string, len(lookups))
	for range lookups {
		result := <-results
		if len(result.Values) > 0 {
			bySource[result.Source] = result.Values
		}
	}

	ordered := make([]HostnameObservation, 0, len(bySource))
	for _, item := range lookups {
		if values := bySource[item.source]; len(values) > 0 {
			ordered = append(ordered, HostnameObservation{Source: item.source, Values: values})
		}
	}
	return ordered
}

// PreferredHostname returns the first hostname from the configured source
// priority: reverse DNS, system resolver, then mDNS/Avahi.
func PreferredHostname(observations []HostnameObservation) string {
	for _, observation := range observations {
		if len(observation.Values) > 0 {
			return observation.Values[0]
		}
	}
	return ""
}

// ValuesForSource returns a defensive copy of values from one discovery source.
func ValuesForSource(observations []HostnameObservation, source string) []string {
	for _, observation := range observations {
		if observation.Source == source {
			return append([]string(nil), observation.Values...)
		}
	}
	return nil
}

func lookupReverseDNS(ctx context.Context, address string) []string {
	values, err := reverseLookup(ctx, address)
	if err != nil {
		return nil
	}
	return values
}

func lookupGetent(ctx context.Context, address string) []string {
	output, err := localCommand(ctx, "getent", "hosts", address)
	if err != nil {
		return nil
	}
	return parseGetentHostnames(output)
}

func lookupAvahi(ctx context.Context, address string) []string {
	output, err := localCommand(ctx, "avahi-resolve-address", address)
	if err != nil {
		return nil
	}
	return parseAvahiHostnames(output)
}

func runLocalCommand(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

func parseGetentHostnames(output string) []string {
	var values []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		values = append(values, fields[1:]...)
	}
	return values
}

func parseAvahiHostnames(output string) []string {
	var values []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		values = append(values, fields[1:]...)
	}
	return values
}

func cleanHostnames(values []string) []string {
	unique := make(map[string]string, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		value = strings.TrimSuffix(value, ".")
		if value == "" || net.ParseIP(value) != nil {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := unique[key]; !exists {
			unique[key] = value
		}
	}

	cleaned := make([]string, 0, len(unique))
	for _, value := range unique {
		cleaned = append(cleaned, value)
	}
	sort.Slice(cleaned, func(i, j int) bool {
		return strings.ToLower(cleaned[i]) < strings.ToLower(cleaned[j])
	})
	return cleaned
}
