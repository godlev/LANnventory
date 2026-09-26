package identification

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/models"
)

// Confidence is deliberately coarse. It represents confidence in a suggested
// managed value, not a probability and never authorizes an automatic write.
type Confidence string

const (
	ConfidenceNone   Confidence = "none"
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// Freshness separates currently usable observations from retained context.
type Freshness string

const (
	FreshnessCurrent    Freshness = "current"
	FreshnessStale      Freshness = "stale"
	FreshnessHistorical Freshness = "historical"
)

const DefaultFreshWithin = 24 * time.Hour

// DiscoveryEvidence is a persistence-agnostic view of one retained discovery
// observation. Active is scope-current in storage; LastSeen and Address are
// still required so an old observation is not presented as current proof.
type DiscoveryEvidence struct {
	Source   string
	Kind     string
	Value    string
	Address  string
	Active   bool
	LastSeen time.Time
}

// ServiceEvidence represents the durable summary for one probed endpoint.
// State=open means the last definitive result was open; freshness is derived
// independently from LastChecked and the Host's current address.
type ServiceEvidence struct {
	Address     string
	Protocol    string
	Port        int
	State       string
	Hint        string
	LastChecked time.Time
}

// WorkloadEvidence is the minimal infrastructure context needed by the
// assessment engine. Current must be established by the aggregation layer from
// the current workload/source state; membership alone must never set ExactMAC.
type WorkloadEvidence struct {
	Name         string
	WorkloadType string
	Status       string
	Source       string
	ExactMAC     bool
	Current      bool
}

// Input contains retained evidence only. Assess never performs I/O or writes.
type Input struct {
	Now            time.Time
	CurrentAddress string
	FreshWithin    time.Duration
	Discovery      []DiscoveryEvidence
	Services       []ServiceEvidence
	Workloads      []WorkloadEvidence
}

// Suggestion is an explainable draft value. Callers may offer it to the user,
// but must not persist it automatically.
type Suggestion struct {
	Value      string
	Confidence Confidence
	Source     string
	Reasons    []string
}

// EvaluatedEvidence preserves provenance and the freshness decision made by the
// domain engine. It is intentionally compact so the API layer can bound and map
// it without duplicating assessment rules.
type EvaluatedEvidence struct {
	Category  string
	Source    string
	Kind      string
	Value     string
	Freshness Freshness
}

// Assessment is the complete pure-domain result for one Host.
type Assessment struct {
	SuggestedName       *Suggestion
	SuggestedDeviceType *Suggestion
	Evidence            []EvaluatedEvidence
	ClueCount           int
	Reasons             []string
	Cautions            []string
	Conflicts           []string
}

// Assess produces conservative, explainable suggestions from retained evidence.
func Assess(in Input) Assessment {
	ctx := assessmentContext{input: in}
	if ctx.input.FreshWithin <= 0 {
		ctx.input.FreshWithin = DefaultFreshWithin
	}
	ctx.currentAddress = canonicalIP(in.CurrentAddress)
	ctx.collectEvidence()

	out := Assessment{
		Evidence:  ctx.evidence,
		ClueCount: len(ctx.currentClues),
	}
	out.SuggestedName, out.Conflicts = ctx.suggestName(out.Conflicts)
	out.SuggestedDeviceType, out.Conflicts, out.Cautions = ctx.suggestDeviceType(out.Conflicts, out.Cautions)
	out.Reasons = ctx.summaryReasons(out)
	return out
}

type assessmentContext struct {
	input          Input
	currentAddress string
	evidence       []EvaluatedEvidence
	currentClues   map[string]struct{}
	currentNames   []nameCandidate
	typeClues      []typeClue
	exactWorkloads []WorkloadEvidence
}

type nameCandidate struct {
	value      string
	normalized string
	priority   int
	confidence Confidence
	source     string
}

type typeClue struct {
	deviceType string
	group      string
	detail     string
}

func (ctx *assessmentContext) collectEvidence() {
	ctx.currentClues = make(map[string]struct{})

	for _, row := range ctx.input.Discovery {
		freshness := ctx.discoveryFreshness(row)
		value := strings.TrimSpace(row.Value)
		ctx.evidence = append(ctx.evidence, EvaluatedEvidence{
			Category:  "discovery",
			Source:    normalizedToken(row.Source),
			Kind:      normalizedToken(row.Kind),
			Value:     value,
			Freshness: freshness,
		})
		if freshness != FreshnessCurrent || value == "" {
			continue
		}
		ctx.currentClues[discoveryClueKey(row)] = struct{}{}
		ctx.addNameCandidate(row)
		ctx.addDescriptorTypeClue(row)
	}

	for _, service := range ctx.input.Services {
		freshness := ctx.serviceFreshness(service)
		value := serviceLabel(service)
		ctx.evidence = append(ctx.evidence, EvaluatedEvidence{
			Category:  "service",
			Source:    "service-scan",
			Kind:      normalizedToken(service.Protocol),
			Value:     value,
			Freshness: freshness,
		})
		if freshness != FreshnessCurrent || !strings.EqualFold(strings.TrimSpace(service.State), "open") {
			continue
		}
		ctx.currentClues[fmt.Sprintf("service:%s:%d", normalizedToken(service.Protocol), service.Port)] = struct{}{}
		ctx.addServiceTypeClue(service)
	}

	for _, workload := range ctx.input.Workloads {
		freshness := FreshnessHistorical
		if workload.Current {
			freshness = FreshnessCurrent
		}
		ctx.evidence = append(ctx.evidence, EvaluatedEvidence{
			Category:  "workload",
			Source:    normalizedToken(workload.Source),
			Kind:      normalizedToken(workload.WorkloadType),
			Value:     strings.TrimSpace(workload.Name),
			Freshness: freshness,
		})
		if !workload.Current || !workload.ExactMAC {
			continue
		}
		ctx.currentClues["workload:exact-mac"] = struct{}{}
		ctx.exactWorkloads = append(ctx.exactWorkloads, workload)
		if name := usableName(workload.Name); name != "" {
			ctx.currentNames = append(ctx.currentNames, nameCandidate{
				value:      name,
				normalized: normalizeName(name),
				priority:   0,
				confidence: ConfidenceHigh,
				source:     "workload-exact-mac",
			})
		}
	}

	sort.SliceStable(ctx.evidence, func(i, j int) bool {
		left, right := ctx.evidence[i], ctx.evidence[j]
		if freshnessRank(left.Freshness) != freshnessRank(right.Freshness) {
			return freshnessRank(left.Freshness) < freshnessRank(right.Freshness)
		}
		if left.Category != right.Category {
			return left.Category < right.Category
		}
		if left.Source != right.Source {
			return left.Source < right.Source
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return strings.ToLower(left.Value) < strings.ToLower(right.Value)
	})
}

func (ctx assessmentContext) discoveryFreshness(row DiscoveryEvidence) Freshness {
	if !row.Active || !sameAddress(row.Address, ctx.currentAddress) {
		return FreshnessHistorical
	}
	if !within(ctx.input.Now, row.LastSeen, ctx.input.FreshWithin) {
		return FreshnessStale
	}
	return FreshnessCurrent
}

func (ctx assessmentContext) serviceFreshness(row ServiceEvidence) Freshness {
	if !sameAddress(row.Address, ctx.currentAddress) {
		return FreshnessHistorical
	}
	if !within(ctx.input.Now, row.LastChecked, ctx.input.FreshWithin) {
		return FreshnessStale
	}
	return FreshnessCurrent
}

func (ctx *assessmentContext) addNameCandidate(row DiscoveryEvidence) {
	name := usableName(row.Value)
	if name == "" {
		return
	}
	priority, confidence, ok := discoveryNamePriority(row.Source, row.Kind)
	if !ok {
		return
	}
	ctx.currentNames = append(ctx.currentNames, nameCandidate{
		value:      name,
		normalized: normalizeName(name),
		priority:   priority,
		confidence: confidence,
		source:     normalizedToken(row.Source),
	})
}

func (ctx *assessmentContext) addDescriptorTypeClue(row DiscoveryEvidence) {
	kind := normalizedToken(row.Kind)
	if kind != "hostname" && kind != "friendly-name" && kind != "model" && kind != "model-number" {
		return
	}
	if deviceType := descriptorDeviceType(row.Value); deviceType != "" {
		ctx.typeClues = append(ctx.typeClues, typeClue{
			deviceType: deviceType,
			group:      "descriptor",
			detail:     fmt.Sprintf("current %s %q", kind, strings.TrimSpace(row.Value)),
		})
	}
}

func (ctx *assessmentContext) addServiceTypeClue(service ServiceEvidence) {
	deviceType := ""
	switch service.Port {
	case 554, 8554:
		deviceType = string(models.DeviceTypeCamera)
	case 631, 9100:
		deviceType = string(models.DeviceTypePrinter)
	}
	if deviceType == "" {
		return
	}
	ctx.typeClues = append(ctx.typeClues, typeClue{
		deviceType: deviceType,
		group:      "service",
		detail:     serviceLabel(service),
	})
}

func (ctx assessmentContext) suggestName(conflicts []string) (*Suggestion, []string) {
	if len(ctx.currentNames) == 0 {
		return nil, conflicts
	}

	bestPriority := ctx.currentNames[0].priority
	for _, candidate := range ctx.currentNames[1:] {
		if candidate.priority < bestPriority {
			bestPriority = candidate.priority
		}
	}

	unique := make(map[string]nameCandidate)
	for _, candidate := range ctx.currentNames {
		if candidate.priority != bestPriority || candidate.normalized == "" {
			continue
		}
		if _, exists := unique[candidate.normalized]; !exists {
			unique[candidate.normalized] = candidate
		}
	}
	if len(unique) != 1 {
		values := make([]string, 0, len(unique))
		for _, candidate := range unique {
			values = append(values, candidate.value)
		}
		sort.Strings(values)
		return nil, append(conflicts, fmt.Sprintf("equally preferred current name evidence disagrees: %s", strings.Join(values, ", ")))
	}

	var selected nameCandidate
	for _, candidate := range unique {
		selected = candidate
	}
	return &Suggestion{
		Value:      selected.value,
		Confidence: selected.confidence,
		Source:     selected.source,
		Reasons:    []string{fmt.Sprintf("selected from current %s evidence", selected.source)},
	}, conflicts
}

func (ctx assessmentContext) suggestDeviceType(conflicts, cautions []string) (*Suggestion, []string, []string) {
	if len(ctx.exactWorkloads) > 1 {
		return nil, append(conflicts, "multiple current exact-MAC workloads match this Host"), cautions
	}
	if len(ctx.exactWorkloads) == 1 {
		workload := ctx.exactWorkloads[0]
		deviceType := workloadDeviceType(workload.WorkloadType)
		if deviceType == "" {
			return nil, conflicts, cautions
		}
		for _, clue := range ctx.typeClues {
			if clue.deviceType != deviceType {
				cautions = appendUnique(cautions, fmt.Sprintf("%s is application/service context; exact-MAC workload remains the infrastructure classification", clue.detail))
			}
		}
		return &Suggestion{
			Value:      deviceType,
			Confidence: ConfidenceHigh,
			Source:     "workload-exact-mac",
			Reasons:    []string{"one current workload interface MAC exactly matches the Host MAC"},
		}, conflicts, cautions
	}

	if len(ctx.typeClues) == 0 {
		return nil, conflicts, cautions
	}

	byType := make(map[string]map[string]struct{})
	details := make(map[string][]string)
	for _, clue := range ctx.typeClues {
		groups := byType[clue.deviceType]
		if groups == nil {
			groups = make(map[string]struct{})
			byType[clue.deviceType] = groups
		}
		groups[clue.group] = struct{}{}
		details[clue.deviceType] = appendUnique(details[clue.deviceType], clue.detail)
	}
	if len(byType) != 1 {
		types := make([]string, 0, len(byType))
		for deviceType := range byType {
			types = append(types, deviceType)
		}
		sort.Strings(types)
		return nil, append(conflicts, fmt.Sprintf("current type clues disagree: %s", strings.Join(types, ", "))), cautions
	}

	var deviceType string
	var groups map[string]struct{}
	for value, valueGroups := range byType {
		deviceType, groups = value, valueGroups
	}
	confidence := ConfidenceLow
	if _, service := groups["service"]; service {
		if _, descriptor := groups["descriptor"]; descriptor {
			confidence = ConfidenceMedium
		}
	}
	return &Suggestion{
		Value:      deviceType,
		Confidence: confidence,
		Source:     "conservative-clues",
		Reasons:    details[deviceType],
	}, conflicts, cautions
}

func (ctx assessmentContext) summaryReasons(out Assessment) []string {
	reasons := make([]string, 0, 3)
	if out.SuggestedName == nil {
		reasons = append(reasons, "no unambiguous current name evidence met the suggestion rules")
	}
	if out.SuggestedDeviceType == nil {
		reasons = append(reasons, "no unambiguous current device-type evidence met the suggestion rules")
	}
	if out.ClueCount == 0 {
		reasons = append(reasons, "only stale, historical, unsupported, or absent evidence is available")
	}
	return reasons
}

func discoveryNamePriority(source, kind string) (int, Confidence, bool) {
	source = normalizedToken(source)
	kind = normalizedToken(kind)
	switch {
	case source == "mdns" && kind == "hostname":
		return 10, ConfidenceMedium, true
	case source == "ssdp" && kind == "friendly-name":
		return 20, ConfidenceMedium, true
	case source == "reverse-dns" && kind == "hostname":
		return 30, ConfidenceLow, true
	case source == "system-resolver" && kind == "hostname":
		return 40, ConfidenceLow, true
	default:
		return 0, ConfidenceNone, false
	}
}

func workloadDeviceType(workloadType string) string {
	switch normalizedToken(workloadType) {
	case "vm":
		return string(models.DeviceTypeVirtualMachine)
	case "container", "lxc":
		return string(models.DeviceTypeContainer)
	default:
		return ""
	}
}

func descriptorDeviceType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	words := strings.FieldsFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	for _, word := range words {
		switch word {
		case "camera", "cam", "nvr":
			return string(models.DeviceTypeCamera)
		case "printer", "laserjet", "officejet":
			return string(models.DeviceTypePrinter)
		}
	}
	return ""
}

func discoveryClueKey(row DiscoveryEvidence) string {
	source := normalizedToken(row.Source)
	kind := normalizedToken(row.Kind)
	value := normalizeName(row.Value)
	if kind == "hostname" && (source == "reverse-dns" || source == "system-resolver") {
		return "resolver-hostname:" + value
	}
	return "discovery:" + source + ":" + kind + ":" + value
}

func serviceLabel(service ServiceEvidence) string {
	protocol := normalizedToken(service.Protocol)
	if protocol == "" {
		protocol = "tcp"
	}
	label := fmt.Sprintf("%s/%d", protocol, service.Port)
	if hint := strings.TrimSpace(service.Hint); hint != "" {
		label += " " + hint
	}
	return label
}

func usableName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ".")
	if value == "" || net.ParseIP(value) != nil {
		return ""
	}
	normalized := normalizeName(value)
	switch normalized {
	case "unknown", "device", "host", "hostname", "localhost", "local":
		return ""
	default:
		return value
	}
}

func normalizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ".")
	return strings.Join(strings.Fields(value), " ")
}

func normalizedToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func within(now, observed time.Time, window time.Duration) bool {
	if now.IsZero() || observed.IsZero() || window <= 0 || observed.After(now) {
		return false
	}
	return now.Sub(observed) <= window
}

func sameAddress(value, current string) bool {
	current = canonicalIP(current)
	if current == "" {
		return false
	}
	return canonicalIP(value) == current
}

func canonicalIP(value string) string {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil {
		return ""
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return ip.String()
}

func freshnessRank(value Freshness) int {
	switch value {
	case FreshnessCurrent:
		return 0
	case FreshnessStale:
		return 1
	default:
		return 2
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
