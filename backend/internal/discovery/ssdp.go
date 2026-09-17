package discovery

import (
	"bufio"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/godlev/LANnventory/internal/models"
)

const (
	ssdpMulticastAddress      = "239.255.255.250:1900"
	ssdpDescriptorMaxBytes    = 128 * 1024
	ssdpSearchResponseMaxSize = 64 * 1024
)

var (
	ssdpSearchTimeout     = 900 * time.Millisecond
	ssdpDescriptorTimeout = 1200 * time.Millisecond
	ssdpLocationSearch    = searchSSDPLocations
	ssdpDescriptorFetch   = fetchSSDPDescriptor
)

// SSDPObservation is identity evidence learned from a UPnP device descriptor.
// Address is always one of the scanner-observed target addresses supplied to
// DiscoverSSDP; arbitrary LOCATION hosts are rejected before any HTTP request.
type SSDPObservation struct {
	Address string
	Kind    string
	Values  []string
}

type ssdpDescriptor struct {
	Device struct {
		FriendlyName string `xml:"friendlyName"`
		Manufacturer string `xml:"manufacturer"`
		ModelName    string `xml:"modelName"`
		ModelNumber  string `xml:"modelNumber"`
	} `xml:"device"`
}

// DiscoverSSDP performs one best-effort local SSDP search and returns identity
// evidence only for LOCATION URLs whose host is a literal scanner-observed IP.
// It never resolves LOCATION hostnames and therefore does not turn untrusted
// SSDP responses into arbitrary outbound requests.
func DiscoverSSDP(parent context.Context, addresses []string) []SSDPObservation {
	if parent == nil {
		parent = context.Background()
	}
	allowed := canonicalAddressSet(addresses)
	if len(allowed) == 0 {
		return nil
	}

	locations := ssdpLocationSearch(parent)
	if len(locations) == 0 {
		return nil
	}

	seenLocations := make(map[string]struct{}, len(locations))
	byKey := make(map[string]SSDPObservation)
	for _, rawLocation := range locations {
		location, address, ok := validateSSDPLocation(rawLocation, allowed)
		if !ok {
			continue
		}
		locationKey := location.String()
		if _, seen := seenLocations[locationKey]; seen {
			continue
		}
		seenLocations[locationKey] = struct{}{}

		descriptor, err := ssdpDescriptorFetch(parent, location)
		if err != nil {
			continue
		}
		appendSSDPObservation(byKey, address, models.DiscoveryKindFriendlyName, descriptor.Device.FriendlyName)
		appendSSDPObservation(byKey, address, models.DiscoveryKindManufacturer, descriptor.Device.Manufacturer)
		appendSSDPObservation(byKey, address, models.DiscoveryKindModel, descriptor.Device.ModelName)
		appendSSDPObservation(byKey, address, models.DiscoveryKindModelNumber, descriptor.Device.ModelNumber)
	}

	result := make([]SSDPObservation, 0, len(byKey))
	for _, observation := range byKey {
		result = append(result, observation)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Address != result[j].Address {
			return result[i].Address < result[j].Address
		}
		return result[i].Kind < result[j].Kind
	})
	return result
}

func appendSSDPObservation(target map[string]SSDPObservation, address, kind, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	key := address + "\x00" + kind
	observation := target[key]
	observation.Address = address
	observation.Kind = kind
	for _, existing := range observation.Values {
		if strings.EqualFold(existing, value) {
			target[key] = observation
			return
		}
	}
	observation.Values = append(observation.Values, value)
	sort.Slice(observation.Values, func(i, j int) bool {
		return strings.ToLower(observation.Values[i]) < strings.ToLower(observation.Values[j])
	})
	target[key] = observation
}

func canonicalAddressSet(addresses []string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, address := range addresses {
		if canonical, ok := canonicalIP(address); ok {
			result[canonical] = struct{}{}
		}
	}
	return result
}

func canonicalIP(value string) (string, bool) {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil {
		return "", false
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String(), true
	}
	return ip.String(), true
}

func validateSSDPLocation(raw string, allowed map[string]struct{}) (*url.URL, string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.User != nil {
		return nil, "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, "", false
	}
	if u.Hostname() == "" {
		return nil, "", false
	}
	address, ok := canonicalIP(u.Hostname())
	if !ok {
		// Deliberately reject DNS names from untrusted SSDP LOCATION headers.
		return nil, "", false
	}
	if _, ok := allowed[address]; !ok {
		return nil, "", false
	}
	if u.Port() != "" {
		if _, err := net.LookupPort("tcp", u.Port()); err != nil {
			return nil, "", false
		}
	}
	return u, address, true
}

func searchSSDPLocations(parent context.Context) []string {
	remote, err := net.ResolveUDPAddr("udp4", ssdpMulticastAddress)
	if err != nil {
		return nil
	}
	conn, err := net.DialUDP("udp4", nil, remote)
	if err != nil {
		return nil
	}
	defer conn.Close()

	deadline := time.Now().Add(ssdpSearchTimeout)
	if parentDeadline, ok := parent.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil
	}

	request := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: " + ssdpMulticastAddress + "\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 1\r\n" +
		"ST: ssdp:all\r\n\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		return nil
	}

	locations := make([]string, 0, 8)
	for {
		if parent.Err() != nil {
			break
		}
		buffer := make([]byte, ssdpSearchResponseMaxSize)
		n, err := conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			break
		}
		if location := parseSSDPLocationHeader(string(buffer[:n])); location != "" {
			locations = append(locations, location)
		}
	}
	return locations
}

func parseSSDPLocationHeader(response string) string {
	reader := textproto.NewReader(bufio.NewReader(strings.NewReader(response)))
	if _, err := reader.ReadLine(); err != nil {
		return ""
	}
	headers, err := reader.ReadMIMEHeader()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(headers.Get("Location"))
}

func fetchSSDPDescriptor(parent context.Context, location *url.URL) (ssdpDescriptor, error) {
	var descriptor ssdpDescriptor
	ctx, cancel := context.WithTimeout(parent, ssdpDescriptorTimeout)
	defer cancel()

	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: ssdpDescriptorTimeout,
		}).DialContext,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   ssdpDescriptorTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("SSDP descriptor redirects are disabled")
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), nil)
	if err != nil {
		return descriptor, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return descriptor, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return descriptor, errors.New("SSDP descriptor returned non-success status")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, ssdpDescriptorMaxBytes+1))
	if err != nil {
		return descriptor, err
	}
	if len(body) > ssdpDescriptorMaxBytes {
		return descriptor, errors.New("SSDP descriptor exceeds size limit")
	}
	if err := xml.Unmarshal(body, &descriptor); err != nil {
		return descriptor, err
	}
	return descriptor, nil
}
