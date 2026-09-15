package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestComparableVersionNormalizesSnapshots(t *testing.T) {
	tests := map[string]string{
		"0.1.0-beta.2":                  "v0.1.0-beta.2",
		"v0.1.0-beta.2":                 "v0.1.0-beta.2",
		"0.1.0-beta.2-SNAPSHOT-deadbee": "v0.1.0-beta.2",
		"not-a-version":                 "",
	}
	for input, want := range tests {
		if got := comparableVersion(input); got != want {
			t.Fatalf("comparableVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSelectLatestReleaseHonorsChannel(t *testing.T) {
	releases := []release{
		{TagName: "v0.2.0-beta.1", Prerelease: true},
		{TagName: "v0.1.5", Prerelease: false},
		{TagName: "v0.1.0-beta.2", Prerelease: true},
	}
	stable, ok := selectLatestRelease(releases, StableChannel)
	if !ok || stable.TagName != "v0.1.5" {
		t.Fatalf("stable = %+v, ok=%v", stable, ok)
	}
	beta, ok := selectLatestRelease(releases, BetaChannel)
	if !ok || beta.TagName != "v0.2.0-beta.1" {
		t.Fatalf("beta = %+v, ok=%v", beta, ok)
	}
}

func TestCheckDoesNotOfferSameSnapshotBaseRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"v0.1.0-beta.2","prerelease":true,"draft":false,"published_at":"2026-09-01T00:00:00Z","assets":[]}]`))
	}))
	defer server.Close()

	service := NewServiceWithURL(server.Client(), server.URL)
	status, err := service.Check(context.Background(), "0.1.0-beta.2-SNAPSHOT-deadbee", BetaChannel, true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if status.Available {
		t.Fatalf("snapshot base should compare equal to released beta: %+v", status)
	}
	if status.LatestVersion != "0.1.0-beta.2" {
		t.Fatalf("LatestVersion = %q", status.LatestVersion)
	}
}

func TestCheckFindsNewerBetaRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"tag_name":"v0.1.0-beta.3","prerelease":true,"draft":false,"published_at":"2026-09-15T00:00:00Z","assets":[]},
			{"tag_name":"v0.1.0-beta.2","prerelease":true,"draft":false,"assets":[]}
		]`))
	}))
	defer server.Close()

	service := NewServiceWithURL(server.Client(), server.URL)
	status, err := service.Check(context.Background(), "0.1.0-beta.2-SNAPSHOT-deadbee", BetaChannel, true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !status.Available || status.LatestVersion != "0.1.0-beta.3" {
		t.Fatalf("status = %+v", status)
	}
}

func TestChecksumForFile(t *testing.T) {
	hash := strings.Repeat("a", 64)
	data := []byte(hash + "  lannventory_0.2.0_linux_amd64.deb\n")
	got, err := checksumForFile(data, "lannventory_0.2.0_linux_amd64.deb")
	if err != nil {
		t.Fatalf("checksumForFile: %v", err)
	}
	if got != hash {
		t.Fatalf("hash = %q, want %q", got, hash)
	}
}

func TestSafePathComponent(t *testing.T) {
	tests := map[string]string{
		"0.1.0-beta.2-SNAPSHOT-0251fa4": "0.1.0-beta.2-SNAPSHOT-0251fa4",
		" v0.1.0 beta/3 ":               "v0.1.0_beta_3",
		"":                              "unknown",
	}
	for input, want := range tests {
		if got := safePathComponent(input); got != want {
			t.Fatalf("safePathComponent(%q) = %q, want %q", input, got, want)
		}
	}
}
