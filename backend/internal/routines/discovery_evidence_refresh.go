package routines

import (
	"log/slog"
	"strings"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

type discoveryEvidenceScope struct {
	source string
	kind   string
}

var refreshedDiscoveryEvidenceScopes = []discoveryEvidenceScope{
	{source: models.DiscoverySourceScanner, kind: models.DiscoveryKindVendor},
	{source: models.DiscoverySourceReverseDNS, kind: models.DiscoveryKindHostname},
	{source: models.DiscoverySourceSystemResolver, kind: models.DiscoveryKindHostname},
	{source: models.DiscoverySourceMDNS, kind: models.DiscoveryKindHostname},
	{source: models.DiscoverySourceSSDP, kind: models.DiscoveryKindFriendlyName},
	{source: models.DiscoverySourceSSDP, kind: models.DiscoveryKindManufacturer},
	{source: models.DiscoverySourceSSDP, kind: models.DiscoveryKindModel},
	{source: models.DiscoverySourceSSDP, kind: models.DiscoveryKindModelNumber},
}

// refreshDiscoveryEvidenceScopes marks the previously current evidence for every
// MAC/address pair observed by this successful scan as historical. The discovery
// stages that follow reactivate only values that are actually observed again.
// This keeps Current/Previous evidence truthful without affecting host state or
// scanner success semantics when best-effort enrichment returns no data.
func refreshDiscoveryEvidenceScopes(hosts []models.Host) {
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		observedAt := strings.TrimSpace(host.Date)
		if observedAt == "" {
			continue
		}
		mac, err := identity.NormalizeMAC(host.Mac)
		if err != nil {
			continue
		}
		address, ok := canonicalObservedIP(host.IP)
		if !ok {
			continue
		}

		key := mac + "\x00" + address
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		for _, scope := range refreshedDiscoveryEvidenceScopes {
			if err := gdb.RecordHostDiscoveryEvidence(mac, address, scope.source, scope.kind, nil, observedAt); err != nil {
				slog.Error(
					"Failed to refresh discovery evidence scope",
					"mac", mac,
					"ip", address,
					"source", scope.source,
					"kind", scope.kind,
					"err", err,
				)
			}
		}
	}
}
