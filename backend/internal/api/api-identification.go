package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/godlev/LANnventory/internal/gdb"
	"github.com/godlev/LANnventory/internal/identification"
	"github.com/godlev/LANnventory/internal/identity"
	"github.com/godlev/LANnventory/internal/models"
)

const (
	identificationMaxDiscoveryEvidence = 64
	identificationMaxServices          = 64
	identificationMaxWorkloads         = 16
	identificationMaxEvidenceRunes     = 512
)

type IdentificationSuggestionResponse struct {
	Value      string                    `json:"value"`
	Confidence identification.Confidence `json:"confidence"`
	Source     string                    `json:"source"`
	Reasons    []string                  `json:"reasons"`
}

type IdentificationEvidenceResponse struct {
	Category  string                   `json:"category"`
	Source    string                   `json:"source"`
	Kind      string                   `json:"kind"`
	Value     string                   `json:"value"`
	Freshness identification.Freshness `json:"freshness"`
}

type IdentificationAssessmentResponse struct {
	State               string                            `json:"state"`
	SuggestedName       *IdentificationSuggestionResponse `json:"suggestedName,omitempty"`
	SuggestedDeviceType *IdentificationSuggestionResponse `json:"suggestedDeviceType,omitempty"`
	ClueCount           int                               `json:"clueCount"`
	Reasons             []string                          `json:"reasons"`
	Cautions            []string                          `json:"cautions"`
	Conflicts           []string                          `json:"conflicts"`
	Evidence            []IdentificationEvidenceResponse  `json:"evidence"`
}

type IdentificationSourceStatus struct {
	Source    string `json:"source"`
	Available bool   `json:"available"`
	Total     int    `json:"total"`
	Included  int    `json:"included"`
	Truncated bool   `json:"truncated"`
	Message   string `json:"message,omitempty"`
}

type IdentificationWarning struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type IdentificationAction struct {
	Key       string `json:"key"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type HostIdentificationResponse struct {
	HostID         int                              `json:"hostId"`
	Mac            string                           `json:"mac"`
	CurrentAddress string                           `json:"currentAddress"`
	Known          bool                             `json:"known"`
	Assessment     IdentificationAssessmentResponse `json:"assessment"`
	Sources        []IdentificationSourceStatus     `json:"sources"`
	Warnings       []IdentificationWarning          `json:"warnings"`
	Actions        []IdentificationAction           `json:"actions"`
}

// getHostIdentification returns retained identification evidence only; it does not run live discovery.\nfunc getHostIdentification(c *gin.Context) {
	host, err := getHostByID(c.Param("id"))
	if err != nil || host.ID < 1 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": errInvalidHostID.Error()})
		return
	}

	now := time.Now().UTC()
	input := identification.Input{
		Now:            now,
		CurrentAddress: strings.TrimSpace(host.IP),
		FreshWithin:    identification.DefaultFreshWithin,
	}
	sources := make([]IdentificationSourceStatus, 0, 4)

	discoveryInput, discoveryStatus := loadIdentificationDiscovery(host)
	input.Discovery = discoveryInput
	sources = append(sources, discoveryStatus)

	serviceInput, serviceStatus := loadIdentificationServices(host)
	input.Services = serviceInput
	sources = append(sources, serviceStatus)

	workloadInput, workloadStatus := loadIdentificationWorkloads(host, now)
	input.Workloads = workloadInput
	sources = append(sources, workloadStatus)

	warnings, addressStatus := loadIdentificationAddressWarnings(host)
	sources = append(sources, addressStatus)

	assessment := identification.Assess(input)
	response := HostIdentificationResponse{
		HostID:         host.ID,
		Mac:            canonicalIdentificationMAC(host.Mac),
		CurrentAddress: strings.TrimSpace(host.IP),
		Known:          host.Known == 1,
		Assessment:     identificationAssessmentResponse(host, assessment),
		Sources:        sources,
		Warnings:       warnings,
		Actions:        identificationActions(host),
	}
	if response.Warnings == nil {
		response.Warnings = []IdentificationWarning{}
	}
	if response.Actions == nil {
		response.Actions = []IdentificationAction{}
	}
	c.IndentedJSON(http.StatusOK, response)
}

func loadIdentificationDiscovery(host models.Host) ([]identification.DiscoveryEvidence, IdentificationSourceStatus) {
	status := IdentificationSourceStatus{Source: "discovery", Available: true}
	rows, err := gdb.SelectHostDiscoveryEvidenceByMAC(host.Mac)
	if err != nil {
		slog.Error("Failed to load retained discovery evidence for identification", "hostID", host.ID, "err", err)
		status.Available = false
		status.Message = "retained discovery evidence is temporarily unavailable"
		return nil, status
	}
	status.Total = len(rows)
	rows, status.Truncated = boundSlice(rows, identificationMaxDiscoveryEvidence)
	status.Included = len(rows)

	result := make([]identification.DiscoveryEvidence, 0, len(rows))
	for _, row := range rows {
		result = append(result, identification.DiscoveryEvidence{
			Source:   boundedIdentificationString(row.Source),
			Kind:     boundedIdentificationString(row.Kind),
			Value:    boundedIdentificationString(row.Value),
			Address:  boundedIdentificationString(row.Address),
			Active:   row.Active,
			LastSeen: parseIdentificationTime(row.LastSeen),
		})
	}
	return result, status
}

func loadIdentificationServices(host models.Host) ([]identification.ServiceEvidence, IdentificationSourceStatus) {
	status := IdentificationSourceStatus{Source: "services", Available: true}
	rows, err := gdb.SelectServicesByMAC(host.Mac)
	if err != nil {
		slog.Error("Failed to load retained services for identification", "hostID", host.ID, "err", err)
		status.Available = false
		status.Message = "retained service evidence is temporarily unavailable"
		return nil, status
	}
	status.Total = len(rows)
	rows, status.Truncated = boundSlice(rows, identificationMaxServices)
	status.Included = len(rows)

	result := make([]identification.ServiceEvidence, 0, len(rows))
	for _, row := range rows {
		result = append(result, identification.ServiceEvidence{
			Address:     boundedIdentificationString(row.Address),
			Protocol:    boundedIdentificationString(row.Protocol),
			Port:        row.Port,
			State:       boundedIdentificationString(row.State),
			Hint:        boundedIdentificationString(row.ServiceHint),
			LastChecked: parseIdentificationTime(row.LastChecked),
		})
	}
	return result, status
}

func loadIdentificationWorkloads(host models.Host, now time.Time) ([]identification.WorkloadEvidence, IdentificationSourceStatus) {
	status := IdentificationSourceStatus{Source: "workloads", Available: true}
	records, err := gdb.SelectInfrastructureWorkloadsByInterfaceMAC(host.Mac)
	if err != nil {
		slog.Error("Failed to load exact workload interface evidence for identification", "hostID", host.ID, "err", err)
		status.Available = false
		status.Message = "workload identity evidence is temporarily unavailable"
		return nil, status
	}
	status.Total = len(records)
	records, status.Truncated = boundSlice(records, identificationMaxWorkloads)
	status.Included = len(records)

	result := make([]identification.WorkloadEvidence, 0, len(records))
	for _, record := range records {
		workload := record.Workload
		result = append(result, identification.WorkloadEvidence{
			Name:         boundedIdentificationString(workload.Name),
			WorkloadType: boundedIdentificationString(workload.WorkloadType),
			Status:       boundedIdentificationString(workload.Status),
			Source:       boundedIdentificationString(workload.Source),
			ExactMAC:     workloadHasExactInterfaceMAC(record, host.Mac),
			Current:      identificationWorkloadCurrent(workload, now),
		})
	}
	return result, status
}

func loadIdentificationAddressWarnings(host models.Host) ([]IdentificationWarning, IdentificationSourceStatus) {
	status := IdentificationSourceStatus{Source: "address-history", Available: true}
	address := strings.TrimSpace(host.IP)
	if address == "" {
		return []IdentificationWarning{}, status
	}

	rows, err := gdb.SelectHostAddressesByAddress(address)
	if err != nil {
		slog.Error("Failed to load address reuse history for identification", "hostID", host.ID, "address", address, "err", err)
		status.Available = false
		status.Message = "address reuse history is temporarily unavailable"
		return []IdentificationWarning{}, status
	}

	hostMAC := canonicalIdentificationMAC(host.Mac)
	other := make(map[string]models.HostAddress)
	for _, row := range rows {
		mac := canonicalIdentificationMAC(row.Mac)
		if mac == "" || strings.EqualFold(mac, hostMAC) {
			continue
		}
		existing, exists := other[mac]
		if !exists || (!existing.Active && row.Active) || row.LastSeen > existing.LastSeen {
			other[mac] = row
		}
	}
	status.Total = len(other)

	activeCount := 0
	historicalCount := 0
	for _, row := range other {
		if row.Active {
			activeCount++
		} else {
			historicalCount++
		}
	}

	warnings := make([]IdentificationWarning, 0, 2)
	if activeCount > 0 {
		warnings = append(warnings, IdentificationWarning{
			Code:     "active-ip-reuse",
			Severity: "caution",
			Message:  fmt.Sprintf("The current address is also active for %d other retained MAC identity/identities; address-based clues must not be treated as identity proof.", activeCount),
		})
	}
	if historicalCount > 0 {
		warnings = append(warnings, IdentificationWarning{
			Code:     "historical-ip-reuse",
			Severity: "info",
			Message:  fmt.Sprintf("The current address was previously observed with %d other MAC identity/identities.", historicalCount),
		})
	}
	status.Included = activeCount + historicalCount
	return warnings, status
}

func identificationAssessmentResponse(host models.Host, assessment identification.Assessment) IdentificationAssessmentResponse {
	evidence := make([]IdentificationEvidenceResponse, 0, len(assessment.Evidence))
	for _, item := range assessment.Evidence {
		evidence = append(evidence, IdentificationEvidenceResponse{
			Category:  item.Category,
			Source:    item.Source,
			Kind:      item.Kind,
			Value:     boundedIdentificationString(item.Value),
			Freshness: item.Freshness,
		})
	}

	return IdentificationAssessmentResponse{
		State:               identificationAssessmentState(host, assessment),
		SuggestedName:       identificationSuggestionResponse(assessment.SuggestedName),
		SuggestedDeviceType: identificationSuggestionResponse(assessment.SuggestedDeviceType),
		ClueCount:           assessment.ClueCount,
		Reasons:             nonNilStrings(assessment.Reasons),
		Cautions:            nonNilStrings(assessment.Cautions),
		Conflicts:           nonNilStrings(assessment.Conflicts),
		Evidence:            evidence,
	}
}

func identificationSuggestionResponse(value *identification.Suggestion) *IdentificationSuggestionResponse {
	if value == nil {
		return nil
	}
	return &IdentificationSuggestionResponse{
		Value:      boundedIdentificationString(value.Value),
		Confidence: value.Confidence,
		Source:     boundedIdentificationString(value.Source),
		Reasons:    nonNilStrings(value.Reasons),
	}
}

func identificationAssessmentState(host models.Host, assessment identification.Assessment) string {
	if host.Known == 1 {
		return "known"
	}
	if len(assessment.Conflicts) > 0 {
		return "conflict"
	}
	if assessment.SuggestedName != nil || assessment.SuggestedDeviceType != nil {
		return "suggested"
	}
	return "needs-investigation"
}

func identificationActions(host models.Host) []IdentificationAction {
	actions := []IdentificationAction{
		{Key: "identity-history", Available: true},
		{Key: "service-scan", Available: strings.TrimSpace(host.IP) != ""},
	}
	if strings.TrimSpace(host.IP) == "" {
		actions[1].Reason = "host has no current address"
	}
	return actions
}

func identificationWorkloadCurrent(workload models.InfrastructureWorkload, now time.Time) bool {
	if strings.TrimSpace(workload.RetiredAt) != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(workload.Source), models.InfrastructureWorkloadSourceManual) {
		return true
	}
	lastSeen := parseIdentificationTime(workload.LastSeen)
	if now.IsZero() || lastSeen.IsZero() || lastSeen.After(now) {
		return false
	}
	return now.Sub(lastSeen) <= identification.DefaultFreshWithin
}

func workloadHasExactInterfaceMAC(record models.InfrastructureWorkloadRecord, hostMAC string) bool {
	target := canonicalIdentificationMAC(hostMAC)
	if target == "" {
		return false
	}
	for _, iface := range record.Interfaces {
		if strings.EqualFold(canonicalIdentificationMAC(iface.Mac), target) {
			return true
		}
	}
	return false
}

func parseIdentificationTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func boundedIdentificationString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) <= identificationMaxEvidenceRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:identificationMaxEvidenceRunes])
}

func canonicalIdentificationMAC(value string) string {
	if normalized, err := identity.NormalizeMAC(strings.TrimSpace(value)); err == nil {
		return normalized
	}
	return strings.TrimSpace(value)
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func boundSlice[T any](values []T, limit int) ([]T, bool) {
	if limit < 0 {
		limit = 0
	}
	if len(values) <= limit {
		return values, false
	}
	return values[:limit], true
}
