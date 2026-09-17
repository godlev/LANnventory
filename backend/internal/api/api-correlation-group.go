package api

import (
	"log/slog"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/godlev/LANnventory/internal/correlation"
	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identity"
)

// ConfirmedIdentityGroupMember is one MAC identity in the user-confirmed
// logical-device projection. Original host observations remain independent.
type ConfirmedIdentityGroupMember struct {
	Mac        string   `json:"mac"`
	HostID     int      `json:"hostId"`
	Name       string   `json:"name"`
	DeviceType string   `json:"deviceType"`
	Exists     bool     `json:"exists"`
	Active     bool     `json:"active"`
	Addresses  []string `json:"addresses"`
	FirstSeen  string   `json:"firstSeen"`
	LastSeen   string   `json:"lastSeen"`
}

// HostIdentityGroupResponse is a read-only confirmed identity group. It does
// not create a new persistent Device row or rewrite Events/Presence/history.
type HostIdentityGroupResponse struct {
	Mac       string                         `json:"mac"`
	Confirmed bool                           `json:"confirmed"`
	Members   []ConfirmedIdentityGroupMember `json:"members"`
}

// getHostIdentityGroup godoc
// @Summary      Get confirmed logical identity group
// @Description  Return the transitive group of MAC identities explicitly confirmed by the user as the same physical device. The projection is read only and does not rewrite Hosts, Events, Presence, or historical observations.
// @Tags         hosts
// @Produce      json
// @Param        id   path      string  true  "Host ID"
// @Success      200  {object}  HostIdentityGroupResponse
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /host/{id}/identity/group [get]
func getHostIdentityGroup(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	decisionRows, err := gdb.SelectAllIdentityCorrelationDecisions()
	if err != nil {
		slog.Error("Failed to load confirmed identity decisions", "id", host.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load confirmed identity group"})
		return
	}
	addressRows, err := gdb.SelectAllHostAddresses()
	if err != nil {
		slog.Error("Failed to load identity addresses for confirmed group", "id", host.ID, "err", err)
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load confirmed identity group"})
		return
	}
	currentHosts, ok := gdb.Select("now")
	if !ok {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": "failed to load current identities"})
		return
	}

	pairs := make([]correlation.DecisionPair, 0, len(decisionRows))
	for _, row := range decisionRows {
		pairs = append(pairs, correlation.DecisionPair{
			MacA:     row.MacA,
			MacB:     row.MacB,
			Decision: row.Decision,
		})
	}
	targetMAC := identity.MACKey(host.Mac)
	groupMACs := correlation.ConfirmedGroup(targetMAC, pairs)
	memberSet := make(map[string]struct{}, len(groupMACs))
	for _, mac := range groupMACs {
		memberSet[mac] = struct{}{}
	}

	currentByMAC := currentHostsByMAC(currentHosts)
	type addressSummary struct {
		values    map[string]struct{}
		firstSeen string
		lastSeen  string
	}
	addressByMAC := make(map[string]*addressSummary, len(groupMACs))
	for _, row := range addressRows {
		mac := identity.MACKey(row.Mac)
		if _, included := memberSet[mac]; !included {
			continue
		}
		summary := addressByMAC[mac]
		if summary == nil {
			summary = &addressSummary{values: make(map[string]struct{})}
			addressByMAC[mac] = summary
		}
		if row.Address != "" {
			summary.values[row.Address] = struct{}{}
		}
		if row.FirstSeen != "" && (summary.firstSeen == "" || row.FirstSeen < summary.firstSeen) {
			summary.firstSeen = row.FirstSeen
		}
		if row.LastSeen != "" && (summary.lastSeen == "" || row.LastSeen > summary.lastSeen) {
			summary.lastSeen = row.LastSeen
		}
	}

	members := make([]ConfirmedIdentityGroupMember, 0, len(groupMACs))
	for _, mac := range groupMACs {
		member := ConfirmedIdentityGroupMember{Mac: mac}
		if summary := addressByMAC[mac]; summary != nil {
			member.FirstSeen = summary.firstSeen
			member.LastSeen = summary.lastSeen
			member.Addresses = make([]string, 0, len(summary.values))
			for address := range summary.values {
				member.Addresses = append(member.Addresses, address)
			}
			sort.Strings(member.Addresses)
		} else {
			member.Addresses = []string{}
		}
		if current, exists := currentByMAC[mac]; exists {
			member.HostID = current.ID
			member.Name = current.Name
			member.DeviceType = current.DeviceType
			member.Exists = true
			member.Active = current.Now == 1
			if current.IP != "" && !containsString(member.Addresses, current.IP) {
				member.Addresses = append(member.Addresses, current.IP)
				sort.Strings(member.Addresses)
			}
			if member.LastSeen == "" {
				member.LastSeen = current.Date
			}
		}
		members = append(members, member)
	}

	// Put the identity currently being viewed first; retain deterministic MAC
	// ordering for the remaining confirmed identities.
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].Mac == targetMAC {
			return true
		}
		if members[j].Mac == targetMAC {
			return false
		}
		return members[i].Mac < members[j].Mac
	})

	c.IndentedJSON(http.StatusOK, HostIdentityGroupResponse{
		Mac:       targetMAC,
		Confirmed: len(members) > 1,
		Members:   members,
	})
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
