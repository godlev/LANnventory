package proxmoxapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxResponseBytes int64 = 4 * 1024 * 1024
	apiPrefix              = "/api2/json"
)

type ErrorKind string

const (
	ErrorInvalidConfig ErrorKind = "invalid-config"
	ErrorTLS           ErrorKind = "tls"
	ErrorAuthentication ErrorKind = "authentication"
	ErrorPermission    ErrorKind = "permission"
	ErrorTimeout       ErrorKind = "timeout"
	ErrorUnreachable   ErrorKind = "unreachable"
	ErrorMalformed     ErrorKind = "malformed-response"
	ErrorAPI           ErrorKind = "api-error"
)

type APIError struct {
	Kind       ErrorKind
	StatusCode int
	Operation  string
}

func (e *APIError) Error() string {
	switch e.Kind {
	case ErrorTLS:
		return "TLS validation failed"
	case ErrorAuthentication:
		return "authentication failed"
	case ErrorPermission:
		return "token lacks required permission"
	case ErrorTimeout:
		return "Proxmox API request timed out"
	case ErrorUnreachable:
		return "Proxmox API is unreachable"
	case ErrorMalformed:
		return "Proxmox API returned malformed data"
	case ErrorInvalidConfig:
		return "invalid Proxmox API configuration"
	default:
		return "Proxmox API request failed"
	}
}

func KindOf(err error) ErrorKind {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Kind
	}
	return ErrorAPI
}

type Config struct {
	BaseURL     string
	TokenID     string
	TokenSecret string
	VerifyTLS   bool
	Timeout     time.Duration
}

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	authHeader string
}

type VersionInfo struct {
	Version string `json:"version"`
	Release string `json:"release"`
	RepoID  string `json:"repoid"`
}

type ClusterStatusEntry struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	NodeID any    `json:"nodeid"`
	Online int    `json:"online"`
	Local  int    `json:"local"`
	IP     string `json:"ip"`
}

type Resource struct {
	Type   string      `json:"type"`
	VMID   json.Number `json:"vmid"`
	Node   string      `json:"node"`
	Name   string      `json:"name"`
	Status string      `json:"status"`
}

func New(config Config) (*Client, error) {
	rawURL := strings.TrimSpace(config.BaseURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, &APIError{Kind: ErrorInvalidConfig, Operation: "configure"}
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Trim(parsed.EscapedPath(), "/") != "" {
		return nil, &APIError{Kind: ErrorInvalidConfig, Operation: "configure"}
	}
	tokenID := strings.TrimSpace(config.TokenID)
	if tokenID == "" || config.TokenSecret == "" {
		return nil, &APIError{Kind: ErrorInvalidConfig, Operation: "configure"}
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: !config.VerifyTLS, // #nosec G402 -- explicit user-controlled setting; default is verify=true.
	}

	return &Client{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		authHeader: "PVEAPIToken=" + tokenID + "=" + config.TokenSecret,
	}, nil
}

func (c *Client) EndpointHost() string {
	if c == nil || c.baseURL == nil {
		return ""
	}
	return c.baseURL.Hostname()
}

func (c *Client) Version(ctx context.Context) (VersionInfo, error) {
	var result VersionInfo
	if err := c.get(ctx, "/version", &result); err != nil {
		return VersionInfo{}, err
	}
	if strings.TrimSpace(result.Version) == "" {
		return VersionInfo{}, &APIError{Kind: ErrorMalformed, Operation: "version"}
	}
	return result, nil
}

func (c *Client) ClusterStatus(ctx context.Context) ([]ClusterStatusEntry, error) {
	var result []ClusterStatusEntry
	if err := c.get(ctx, "/cluster/status", &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []ClusterStatusEntry{}
	}
	return result, nil
}

func (c *Client) ClusterResources(ctx context.Context) ([]Resource, error) {
	var result []Resource
	if err := c.get(ctx, "/cluster/resources", &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []Resource{}
	}
	return result, nil
}

func (c *Client) GuestConfig(ctx context.Context, node, workloadType string, vmid int) (map[string]any, error) {
	node = strings.TrimSpace(node)
	if node == "" || vmid < 1 {
		return nil, &APIError{Kind: ErrorInvalidConfig, Operation: "guest-config"}
	}
	var segment string
	switch workloadType {
	case "vm", "qemu":
		segment = "qemu"
	case "container", "lxc":
		segment = "lxc"
	default:
		return nil, &APIError{Kind: ErrorInvalidConfig, Operation: "guest-config"}
	}
	endpoint := "/nodes/" + url.PathEscape(node) + "/" + segment + "/" + strconv.Itoa(vmid) + "/config"
	result := map[string]any{}
	if err := c.get(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) get(ctx context.Context, endpoint string, target any) error {
	if c == nil || c.baseURL == nil || c.httpClient == nil {
		return &APIError{Kind: ErrorInvalidConfig, Operation: "request"}
	}
	if !strings.HasPrefix(endpoint, "/") || strings.Contains(endpoint, "..") {
		return &APIError{Kind: ErrorInvalidConfig, Operation: "request"}
	}

	requestURL := strings.TrimRight(c.baseURL.String(), "/") + apiPrefix + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return &APIError{Kind: ErrorInvalidConfig, Operation: endpoint}
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return classifyRequestError(err, endpoint)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &APIError{Kind: ErrorAuthentication, StatusCode: resp.StatusCode, Operation: endpoint}
	case http.StatusForbidden:
		return &APIError{Kind: ErrorPermission, StatusCode: resp.StatusCode, Operation: endpoint}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{Kind: ErrorAPI, StatusCode: resp.StatusCode, Operation: endpoint}
	}

	reader := io.LimitReader(resp.Body, maxResponseBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return &APIError{Kind: ErrorMalformed, StatusCode: resp.StatusCode, Operation: endpoint}
	}
	if int64(len(data)) > maxResponseBytes {
		return &APIError{Kind: ErrorMalformed, StatusCode: resp.StatusCode, Operation: endpoint}
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return &APIError{Kind: ErrorMalformed, StatusCode: resp.StatusCode, Operation: endpoint}
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return &APIError{Kind: ErrorMalformed, StatusCode: resp.StatusCode, Operation: endpoint}
	}
	return nil
}

func classifyRequestError(err error, operation string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &APIError{Kind: ErrorTimeout, Operation: operation}
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var unknownAuthority x509.UnknownAuthorityError
		var certificateInvalid x509.CertificateInvalidError
		var hostnameError x509.HostnameError
		if errors.As(urlErr.Err, &unknownAuthority) ||
			errors.As(urlErr.Err, &certificateInvalid) ||
			errors.As(urlErr.Err, &hostnameError) {
			return &APIError{Kind: ErrorTLS, Operation: operation}
		}
		var netErr net.Error
		if errors.As(urlErr.Err, &netErr) && netErr.Timeout() {
			return &APIError{Kind: ErrorTimeout, Operation: operation}
		}
		if strings.Contains(strings.ToLower(urlErr.Err.Error()), "tls") ||
			strings.Contains(strings.ToLower(urlErr.Err.Error()), "certificate") {
			return &APIError{Kind: ErrorTLS, Operation: operation}
		}
		return &APIError{Kind: ErrorUnreachable, Operation: operation}
	}

	return &APIError{Kind: ErrorUnreachable, Operation: operation}
}

func NormalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "pve-manager/") {
		return value
	}
	return fmt.Sprintf("pve-manager/%s", value)
}
