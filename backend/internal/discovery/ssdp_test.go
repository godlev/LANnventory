package discovery

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/godlev/LANnventory/internal/models"
)

func TestValidateSSDPLocationAcceptsOnlyObservedLiteralIP(t *testing.T) {
	allowed := canonicalAddressSet([]string{"192.168.1.50", "2001:db8::50"})

	tests := []struct {
		name    string
		value   string
		wantOK  bool
		wantIP  string
	}{
		{name: "observed ipv4", value: "http://192.168.1.50:1400/xml/device.xml", wantOK: true, wantIP: "192.168.1.50"},
		{name: "observed ipv6", value: "http://[2001:db8::50]:8080/device.xml", wantOK: true, wantIP: "2001:db8::50"},
		{name: "unobserved literal", value: "http://192.168.1.99/device.xml"},
		{name: "hostname rejected", value: "http://device.local/device.xml"},
		{name: "internet hostname rejected", value: "https://example.com/device.xml"},
		{name: "unsupported scheme", value: "ftp://192.168.1.50/device.xml"},
		{name: "userinfo rejected", value: "http://user:pass@192.168.1.50/device.xml"},
		{name: "invalid port", value: "http://192.168.1.50:notaport/device.xml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, address, ok := validateSSDPLocation(tc.value, allowed)
			if ok != tc.wantOK || address != tc.wantIP {
				t.Fatalf("validateSSDPLocation(%q) = address %q ok %v, want %q %v", tc.value, address, ok, tc.wantIP, tc.wantOK)
			}
		})
	}
}

func TestParseSSDPLocationHeaderIsCaseInsensitive(t *testing.T) {
	response := "HTTP/1.1 200 OK\r\nCACHE-CONTROL: max-age=1800\r\nloCaTiOn: http://192.168.1.20/device.xml\r\nST: upnp:rootdevice\r\n\r\n"
	if got := parseSSDPLocationHeader(response); got != "http://192.168.1.20/device.xml" {
		t.Fatalf("parseSSDPLocationHeader = %q", got)
	}
}

func TestDiscoverSSDPFiltersLocationsAndReturnsDeterministicEvidence(t *testing.T) {
	oldSearch := ssdpLocationSearch
	oldFetch := ssdpDescriptorFetch
	ssdpLocationSearch = func(context.Context) []string {
		return []string{
			"http://192.168.1.50:1400/device.xml",
			"http://192.168.1.50:1400/device.xml",
			"http://192.168.1.99/device.xml",
			"http://device.local/device.xml",
		}
	}
	fetchCalls := 0
	ssdpDescriptorFetch = func(_ context.Context, location *url.URL) (ssdpDescriptor, error) {
		fetchCalls++
		if location.Hostname() != "192.168.1.50" {
			t.Fatalf("descriptor fetch escaped observed target: %s", location)
		}
		var descriptor ssdpDescriptor
		descriptor.Device.FriendlyName = "Sony BRAVIA XR"
		descriptor.Device.Manufacturer = "Sony"
		descriptor.Device.ModelName = "BRAVIA XR"
		descriptor.Device.ModelNumber = "XR-55A95L"
		return descriptor, nil
	}
	t.Cleanup(func() {
		ssdpLocationSearch = oldSearch
		ssdpDescriptorFetch = oldFetch
	})

	got := DiscoverSSDP(context.Background(), []string{"192.168.1.50"})
	if fetchCalls != 1 {
		t.Fatalf("descriptor fetch calls = %d, want 1", fetchCalls)
	}
	if len(got) != 4 {
		t.Fatalf("observations = %+v, want four identity values", got)
	}

	want := []SSDPObservation{
		{Address: "192.168.1.50", Kind: models.DiscoveryKindFriendlyName, Values: []string{"Sony BRAVIA XR"}},
		{Address: "192.168.1.50", Kind: models.DiscoveryKindManufacturer, Values: []string{"Sony"}},
		{Address: "192.168.1.50", Kind: models.DiscoveryKindModel, Values: []string{"BRAVIA XR"}},
		{Address: "192.168.1.50", Kind: models.DiscoveryKindModelNumber, Values: []string{"XR-55A95L"}},
	}
	for i := range want {
		if got[i].Address != want[i].Address || got[i].Kind != want[i].Kind || strings.Join(got[i].Values, "|") != strings.Join(want[i].Values, "|") {
			t.Fatalf("observation[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestFetchSSDPDescriptorParsesIdentityAndRejectsRedirect(t *testing.T) {
	xmlBody := `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <friendlyName>Living Room TV</friendlyName>
    <manufacturer>Sony</manufacturer>
    <modelName>BRAVIA XR</modelName>
    <modelNumber>XR-55A95L</modelNumber>
  </device>
</root>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/device.xml", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(xmlBody))
	}))
	defer server.Close()

	location, err := url.Parse(server.URL + "/device.xml")
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := fetchSSDPDescriptor(context.Background(), location)
	if err != nil {
		t.Fatalf("fetchSSDPDescriptor: %v", err)
	}
	if descriptor.Device.FriendlyName != "Living Room TV" || descriptor.Device.Manufacturer != "Sony" || descriptor.Device.ModelNumber != "XR-55A95L" {
		t.Fatalf("descriptor = %+v", descriptor)
	}

	redirect, _ := url.Parse(server.URL + "/redirect")
	if _, err := fetchSSDPDescriptor(context.Background(), redirect); err == nil {
		t.Fatal("descriptor redirect was followed, want rejection")
	}
}

func TestFetchSSDPDescriptorRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, strings.Repeat("x", ssdpDescriptorMaxBytes+1))
	}))
	defer server.Close()

	location, _ := url.Parse(server.URL)
	if _, err := fetchSSDPDescriptor(context.Background(), location); err == nil {
		t.Fatal("oversized SSDP descriptor was accepted")
	}
}
