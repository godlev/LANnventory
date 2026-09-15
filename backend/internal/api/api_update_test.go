package api

import "testing"

func TestUpdateHealthURL(t *testing.T) {
	tests := map[string]struct {
		host string
		port string
		want string
	}{
		"configured IPv4": {host: "10.4.1.29", port: "8840", want: "http://10.4.1.29:8840/api/health"},
		"wildcard IPv4": {host: "0.0.0.0", port: "8840", want: "http://127.0.0.1:8840/api/health"},
		"wildcard IPv6": {host: "::", port: "8840", want: "http://[::1]:8840/api/health"},
		"default port": {host: "127.0.0.1", port: "", want: "http://127.0.0.1:8840/api/health"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := updateHealthURL(tc.host, tc.port); got != tc.want {
				t.Fatalf("updateHealthURL(%q, %q) = %q, want %q", tc.host, tc.port, got, tc.want)
			}
		})
	}
}
