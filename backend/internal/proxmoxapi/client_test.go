package proxmoxapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientUsesTokenHeaderAndNeverIncludesSecretInErrors(t *testing.T) {
	const tokenID = "lannventory@pve!inventory"
	const secret = "super-secret-proxmox-token"

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "PVEAPIToken="+tokenID+"="+secret {
			t.Fatalf("Authorization header = %q", got)
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL: server.URL, TokenID: tokenID, TokenSecret: secret, VerifyTLS: false, Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = client.Version(context.Background())
	if err == nil || KindOf(err) != ErrorAuthentication {
		t.Fatalf("Version error = %v kind=%s", err, KindOf(err))
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), tokenID) {
		t.Fatalf("error leaked credentials: %q", err.Error())
	}
}

func TestClientTLSVerificationDefaultsToCallerChoiceAndFailsClosed(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"version":"9.2.10"}}`))
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL: server.URL, TokenID: "user@pve!token", TokenSecret: "secret", VerifyTLS: true, Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = client.Version(context.Background())
	if err == nil || KindOf(err) != ErrorTLS {
		t.Fatalf("TLS verification error = %v kind=%s", err, KindOf(err))
	}

	client, err = New(Config{
		BaseURL: server.URL, TokenID: "user@pve!token", TokenSecret: "secret", VerifyTLS: false, Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New insecure explicit: %v", err)
	}
	version, err := client.Version(context.Background())
	if err != nil || version.Version != "9.2.10" {
		t.Fatalf("explicit TLS bypass version=%+v err=%v", version, err)
	}
}

func TestClientDoesNotFollowRedirectsWithCredentials(t *testing.T) {
	var redirectedAuthorization string
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"version":"9.2.10"}}`))
	}))
	defer target.Close()

	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/api2/json/version", http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	client, err := New(Config{
		BaseURL: source.URL, TokenID: "user@pve!token", TokenSecret: "secret", VerifyTLS: false, Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = client.Version(context.Background())
	if err == nil || KindOf(err) != ErrorAPI {
		t.Fatalf("redirect error = %v kind=%s", err, KindOf(err))
	}
	if redirectedAuthorization != "" {
		t.Fatalf("credentials were forwarded across redirect")
	}
}

func TestClientRejectsNonHTTPSConfiguration(t *testing.T) {
	_, err := New(Config{BaseURL: "http://127.0.0.1:8006", TokenID: "u@pve!t", TokenSecret: "secret", VerifyTLS: true})
	if err == nil || KindOf(err) != ErrorInvalidConfig {
		t.Fatalf("New non-HTTPS error=%v kind=%s", err, KindOf(err))
	}
}
