// Package service provides RemoteConnector for managing remote agent-based servers.
package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cofedish/3x-UI-agents/database/model"
	"github.com/cofedish/3x-UI-agents/logger"
	"github.com/cofedish/3x-UI-agents/xray"
)

// RemoteConnector implements ServerConnector for remote agent-managed servers.
type RemoteConnector struct {
	serverId   int
	endpoint   string
	authType   string
	jwtToken   string // JWT bearer token (empty for mTLS)
	httpClient *http.Client
}

// AgentResponse is the standard response format from agent API.
type AgentResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *AgentError     `json:"error,omitempty"`
	TraceId string          `json:"trace_id,omitempty"`
}

// AgentError represents an error from the agent API.
type AgentError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// NewRemoteConnector creates a new RemoteConnector for a remote server.
func NewRemoteConnector(server *model.Server) (*RemoteConnector, error) {
	connector := &RemoteConnector{
		serverId: server.Id,
		endpoint: server.Endpoint,
		authType: server.AuthType,
	}

	// Initialize HTTP client and auth based on auth type
	var err error
	switch server.AuthType {
	case "mtls":
		connector.httpClient, err = createMTLSClient(server)
	case "jwt":
		connector.httpClient, connector.jwtToken, err = createJWTClient(server)
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", server.AuthType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return connector, nil
}

// createMTLSClient creates an HTTP client with mTLS authentication.
func createMTLSClient(server *model.Server) (*http.Client, error) {
	// Parse auth data. We support:
	// 1) JSON with file paths: { "certFile": "...", "keyFile": "...", "caFile": "..." }
	// 2) JSON with PEM contents: { "certPem": "...", "keyPem": "...", "caPem": "..." }
	// 3) Raw PEM bundle (cert + key + ca) pasted as a single string.
	var authData struct {
		CertFile string `json:"certFile"`
		KeyFile  string `json:"keyFile"`
		CAFile   string `json:"caFile"`
		CertPem  string `json:"certPem"`
		KeyPem   string `json:"keyPem"`
		CAPem    string `json:"caPem"`
	}

	raw := server.AuthData
	_ = json.Unmarshal([]byte(raw), &authData) // best effort

	// Try PEM contents first (inlined)
	var cert tls.Certificate
	var caCertPool *x509.CertPool

	if authData.CertPem != "" && authData.KeyPem != "" {
		c, err := tls.X509KeyPair([]byte(authData.CertPem), []byte(authData.KeyPem))
		if err != nil {
			return nil, fmt.Errorf("failed to parse inlined client certificate: %w", err)
		}
		cert = c
	}

	if authData.CAPem != "" {
		caCertPool = x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM([]byte(authData.CAPem)) {
			return nil, fmt.Errorf("failed to parse inlined CA certificate")
		}
	}

	// If no PEM provided, try file paths
	if (cert.Certificate == nil || len(cert.Certificate) == 0) && authData.CertFile != "" && authData.KeyFile != "" {
		c, err := tls.LoadX509KeyPair(authData.CertFile, authData.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		cert = c
	}

	if caCertPool == nil && authData.CAFile != "" {
		caCert, err := os.ReadFile(authData.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %w", err)
		}
		caCertPool = x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
	}

	// If still empty, attempt to parse raw PEM bundle pasted directly
	if cert.Certificate == nil || len(cert.Certificate) == 0 || caCertPool == nil {
		var certPEM, keyPEM, caPEM []byte
		certPEM, keyPEM, caPEM = splitPEMBundle([]byte(raw))

		if len(certPEM) > 0 && len(keyPEM) > 0 {
			c, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("failed to parse pasted client cert/key: %w", err)
			}
			cert = c
		}

		if len(caPEM) > 0 {
			caCertPool = x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caPEM) {
				return nil, fmt.Errorf("failed to parse pasted CA certificate")
			}
		}
	}

	if cert.Certificate == nil || len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("invalid mTLS auth data: client cert/key not provided")
	}
	if caCertPool == nil {
		return nil, fmt.Errorf("invalid mTLS auth data: CA certificate not provided")
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS13,
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return client, nil
}

// splitPEMBundle best-effort splits a combined PEM string into cert, key, and CA blocks.
func splitPEMBundle(data []byte) (certPEM, keyPEM, caPEM []byte) {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch block.Type {
		case "CERTIFICATE":
			if certPEM == nil {
				certPEM = pem.EncodeToMemory(block)
			} else {
				caPEM = append(caPEM, pem.EncodeToMemory(block)...)
			}
		case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "ED25519 PRIVATE KEY":
			keyPEM = pem.EncodeToMemory(block)
		default:
			// ignore
		}
	}
	return
}

// createJWTClient creates an HTTP client with JWT authentication.
// Returns the HTTP client and the JWT token to be used in Authorization header.
func createJWTClient(server *model.Server) (*http.Client, string, error) {
	// Parse auth data (JSON with JWT token or raw token string)
	var token string
	var authData struct {
		Token string `json:"token"`
	}

	// Try parsing as JSON first
	if err := json.Unmarshal([]byte(server.AuthData), &authData); err == nil && authData.Token != "" {
		token = authData.Token
	} else {
		// If not JSON, treat the whole AuthData as the token
		token = server.AuthData
	}

	if token == "" {
		return nil, "", fmt.Errorf("JWT token is required in auth data")
	}

	// Create standard HTTPS client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return client, token, nil
}

// doRequest performs an HTTP request to the agent API.
func (c *RemoteConnector) doRequest(ctx context.Context, method, path string, body interface{}) (*AgentResponse, error) {
	url := c.endpoint + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// For JWT auth, add Authorization header
	if c.authType == "jwt" && c.jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwtToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var agentResp AgentResponse
	if err := json.Unmarshal(respData, &agentResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !agentResp.Success {
		if agentResp.Error != nil {
			return nil, fmt.Errorf("agent error: %s - %s", agentResp.Error.Code, agentResp.Error.Message)
		}
		return nil, fmt.Errorf("agent request failed")
	}

	return &agentResp, nil
}

// GetServerInfo returns server information from the agent.
func (c *RemoteConnector) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/info", nil)
	if err != nil {
		return nil, err
	}

	var info ServerInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse server info: %w", err)
	}

	info.ServerId = c.serverId
	return &info, nil
}

// GetHealth returns health status from the agent.
func (c *RemoteConnector) GetHealth(ctx context.Context) (*HealthStatus, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/health", nil)
	if err != nil {
		return nil, err
	}

	var health HealthStatus
	if err := json.Unmarshal(resp.Data, &health); err != nil {
		return nil, fmt.Errorf("failed to parse health status: %w", err)
	}

	return &health, nil
}

// ListInbounds retrieves inbounds from the agent.
func (c *RemoteConnector) ListInbounds(ctx context.Context) ([]*model.Inbound, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/inbounds", nil)
	if err != nil {
		return nil, err
	}

	var inbounds []*model.Inbound
	if err := json.Unmarshal(resp.Data, &inbounds); err != nil {
		return nil, fmt.Errorf("failed to parse inbounds: %w", err)
	}

	// Set server_id for all inbounds
	for _, inbound := range inbounds {
		inbound.ServerId = c.serverId
	}

	return inbounds, nil
}

// GetInbound retrieves a specific inbound from the agent.
func (c *RemoteConnector) GetInbound(ctx context.Context, id int) (*model.Inbound, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/inbounds/%d", id), nil)
	if err != nil {
		return nil, err
	}

	var inbound model.Inbound
	if err := json.Unmarshal(resp.Data, &inbound); err != nil {
		return nil, fmt.Errorf("failed to parse inbound: %w", err)
	}

	inbound.ServerId = c.serverId
	return &inbound, nil
}

// AddInbound adds a new inbound via the agent.
func (c *RemoteConnector) AddInbound(ctx context.Context, inbound *model.Inbound) error {
	_, err := c.doRequest(ctx, "POST", "/api/v1/inbounds", inbound)
	return err
}

// UpdateInbound updates an existing inbound via the agent.
func (c *RemoteConnector) UpdateInbound(ctx context.Context, inbound *model.Inbound) error {
	_, err := c.doRequest(ctx, "PUT", fmt.Sprintf("/api/v1/inbounds/%d", inbound.Id), inbound)
	return err
}

// DeleteInbound deletes an inbound via the agent.
func (c *RemoteConnector) DeleteInbound(ctx context.Context, id int) error {
	_, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/inbounds/%d", id), nil)
	return err
}

// AddClient adds a client to an inbound via the agent.
func (c *RemoteConnector) AddClient(ctx context.Context, inbound *model.Inbound) error {
	_, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/inbounds/%d/clients", inbound.Id), inbound)
	return err
}

// UpdateClient updates a client via the agent.
func (c *RemoteConnector) UpdateClient(ctx context.Context, inbound *model.Inbound, clientIndex int) error {
	_, err := c.doRequest(ctx, "PUT", fmt.Sprintf("/api/v1/inbounds/%d/clients/%d", inbound.Id, clientIndex), inbound)
	return err
}

// DeleteClient deletes a client from an inbound via the agent.
func (c *RemoteConnector) DeleteClient(ctx context.Context, inboundId int, clientEmail string) error {
	_, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/inbounds/%d/clients/%s", inboundId, clientEmail), nil)
	return err
}

// ResetClientTraffic resets client traffic via the agent.
func (c *RemoteConnector) ResetClientTraffic(ctx context.Context, inboundId int, email string) error {
	_, err := c.doRequest(ctx, "POST", fmt.Sprintf("/api/v1/inbounds/%d/clients/%s/reset-traffic", inboundId, email), nil)
	return err
}

// GetOnlineClients retrieves online clients from the agent.
func (c *RemoteConnector) GetOnlineClients(ctx context.Context) ([]string, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/clients/online", nil)
	if err != nil {
		return nil, err
	}

	var emails []string
	if err := json.Unmarshal(resp.Data, &emails); err != nil {
		return nil, fmt.Errorf("failed to parse online clients: %w", err)
	}

	return emails, nil
}

// GetTraffic retrieves traffic statistics from the agent.
func (c *RemoteConnector) GetTraffic(ctx context.Context, reset bool) (*xray.Traffic, error) {
	path := "/api/v1/traffic"
	if reset {
		path += "?reset=true"
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var traffic xray.Traffic
	if err := json.Unmarshal(resp.Data, &traffic); err != nil {
		return nil, fmt.Errorf("failed to parse traffic: %w", err)
	}

	return &traffic, nil
}

// GetClientTraffics retrieves client traffic statistics from the agent.
func (c *RemoteConnector) GetClientTraffics(ctx context.Context) ([]*xray.ClientTraffic, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/traffic/clients", nil)
	if err != nil {
		return nil, err
	}

	var traffics []*xray.ClientTraffic
	if err := json.Unmarshal(resp.Data, &traffics); err != nil {
		return nil, fmt.Errorf("failed to parse client traffics: %w", err)
	}

	// Set server_id
	for _, traffic := range traffics {
		traffic.ServerId = c.serverId
	}

	return traffics, nil
}

// StartXray starts Xray on the agent.
func (c *RemoteConnector) StartXray(ctx context.Context) error {
	_, err := c.doRequest(ctx, "POST", "/api/v1/xray/start", nil)
	return err
}

// StopXray stops Xray on the agent.
func (c *RemoteConnector) StopXray(ctx context.Context) error {
	_, err := c.doRequest(ctx, "POST", "/api/v1/xray/stop", nil)
	return err
}

// RestartXray restarts Xray on the agent.
func (c *RemoteConnector) RestartXray(ctx context.Context) error {
	_, err := c.doRequest(ctx, "POST", "/api/v1/xray/restart", nil)
	return err
}

// GetXrayVersion retrieves Xray version from the agent.
func (c *RemoteConnector) GetXrayVersion(ctx context.Context) (string, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/xray/version", nil)
	if err != nil {
		return "", err
	}

	var versionResp struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(resp.Data, &versionResp); err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}

	return versionResp.Version, nil
}

// GetXrayConfig retrieves Xray configuration from the agent.
func (c *RemoteConnector) GetXrayConfig(ctx context.Context) (string, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/xray/config", nil)
	if err != nil {
		return "", err
	}

	var configResp struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(resp.Data, &configResp); err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}

	return configResp.Config, nil
}

// GetSystemStats retrieves system statistics from the agent.
func (c *RemoteConnector) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/system/stats", nil)
	if err != nil {
		return nil, err
	}

	var stats SystemStats
	if err := json.Unmarshal(resp.Data, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse system stats: %w", err)
	}

	return &stats, nil
}

// GetLogs retrieves logs from the agent.
func (c *RemoteConnector) GetLogs(ctx context.Context, count int) ([]string, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v1/logs?count=%d", count), nil)
	if err != nil {
		return nil, err
	}

	var logs []string
	if err := json.Unmarshal(resp.Data, &logs); err != nil {
		return nil, fmt.Errorf("failed to parse logs: %w", err)
	}

	return logs, nil
}

// UpdateGeoFiles triggers geo file update on the agent.
func (c *RemoteConnector) UpdateGeoFiles(ctx context.Context) error {
	_, err := c.doRequest(ctx, "POST", "/api/v1/geofiles/update", nil)
	return err
}

// InstallXray installs Xray on the agent.
func (c *RemoteConnector) InstallXray(ctx context.Context, version string) error {
	body := map[string]string{"version": version}
	_, err := c.doRequest(ctx, "POST", "/api/v1/xray/install", body)
	return err
}

// GenerateCert generates a certificate on the agent.
func (c *RemoteConnector) GenerateCert(ctx context.Context, domain string) (*CertInfo, error) {
	body := map[string]string{"domain": domain}
	resp, err := c.doRequest(ctx, "POST", "/api/v1/certificates/generate", body)
	if err != nil {
		return nil, err
	}

	var cert CertInfo
	if err := json.Unmarshal(resp.Data, &cert); err != nil {
		return nil, fmt.Errorf("failed to parse cert info: %w", err)
	}

	return &cert, nil
}

// GetCerts retrieves certificate information from the agent.
func (c *RemoteConnector) GetCerts(ctx context.Context) ([]*CertInfo, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/certificates", nil)
	if err != nil {
		return nil, err
	}

	var certs []*CertInfo
	if err := json.Unmarshal(resp.Data, &certs); err != nil {
		return nil, fmt.Errorf("failed to parse certificates: %w", err)
	}

	return certs, nil
}

// BackupDatabase creates a database backup on the agent.
func (c *RemoteConnector) BackupDatabase(ctx context.Context) ([]byte, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/backup", nil)
	if err != nil {
		return nil, err
	}

	var backupResp struct {
		Data string `json:"data"` // Base64 encoded
	}
	if err := json.Unmarshal(resp.Data, &backupResp); err != nil {
		return nil, fmt.Errorf("failed to parse backup: %w", err)
	}

	// Decode base64 data
	backupData, err := base64.StdEncoding.DecodeString(backupResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode backup data: %w", err)
	}

	return backupData, nil
}

// RestoreDatabase restores database on the agent.
func (c *RemoteConnector) RestoreDatabase(ctx context.Context, data []byte) error {
	// Encode database data to base64
	encodedData := base64.StdEncoding.EncodeToString(data)

	// Prepare request payload
	payload := map[string]string{
		"data": encodedData,
	}

	// Send restore request to agent
	_, err := c.doRequest(ctx, "POST", "/api/v1/restore", payload)
	if err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	logger.Info(fmt.Sprintf("Successfully restored database on server %d", c.serverId))
	return nil
}
