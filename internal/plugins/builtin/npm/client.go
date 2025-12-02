package npm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client handles communication with NPM API
type Client struct {
	mu         sync.RWMutex
	baseURL    string
	email      string
	password   string
	token      string
	tokenExp   time.Time
	httpClient *http.Client
}

// NewClient creates a new NPM API client
func NewClient(baseURL, email, password string) *Client {
	return &Client{
		baseURL:  baseURL,
		email:    email,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TokenResponse represents the NPM token API response
type TokenResponse struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

// ProxyHost represents an NPM proxy host
type ProxyHost struct {
	ID             int      `json:"id"`
	CreatedOn      string   `json:"created_on"`
	ModifiedOn     string   `json:"modified_on"`
	DomainNames    []string `json:"domain_names"`
	ForwardScheme  string   `json:"forward_scheme"`
	ForwardHost    string   `json:"forward_host"`
	ForwardPort    int      `json:"forward_port"`
	CertificateID  int      `json:"certificate_id"`
	SSLForced      bool     `json:"ssl_forced"`
	HSTSEnabled    bool     `json:"hsts_enabled"`
	HSTSSubdomains bool     `json:"hsts_subdomains"`
	HTTP2Support   bool     `json:"http2_support"`
	BlockExploits  bool     `json:"block_exploits"`
	CachingEnabled bool     `json:"caching_enabled"`
	AllowWebsocket bool     `json:"allow_websocket_upgrade"`
	AccessListID   int      `json:"access_list_id"`
	Enabled        bool     `json:"enabled"`
	Meta           struct {
		LetsencryptAgree bool   `json:"letsencrypt_agree"`
		DNSChallenge     bool   `json:"dns_challenge"`
		LetsencryptEmail string `json:"letsencrypt_email"`
		NginxOnline      bool   `json:"nginx_online"`
		NginxErr         string `json:"nginx_err"`
	} `json:"meta"`
}

// authenticate gets a new token from NPM
func (c *Client) authenticate() error {
	payload := map[string]string{
		"identity": c.email,
		"secret":   c.password,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/tokens", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	c.mu.Lock()
	c.token = tokenResp.Token
	c.tokenExp = tokenResp.Expires
	c.mu.Unlock()

	return nil
}

// getToken returns a valid token, refreshing if necessary
func (c *Client) getToken() (string, error) {
	c.mu.RLock()
	token := c.token
	exp := c.tokenExp
	c.mu.RUnlock()

	// Refresh if token is empty or expired (with 5 minute buffer)
	if token == "" || time.Now().Add(5*time.Minute).After(exp) {
		if err := c.authenticate(); err != nil {
			return "", err
		}
		c.mu.RLock()
		token = c.token
		c.mu.RUnlock()
	}

	return token, nil
}

// doRequest performs an authenticated request to NPM API
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// If we get a 401, try to re-authenticate once
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if err := c.authenticate(); err != nil {
			return nil, fmt.Errorf("re-authentication failed: %w", err)
		}

		token, _ = c.getToken()
		req, err = http.NewRequest(method, c.baseURL+path, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// GetProxyHosts retrieves all proxy hosts from NPM
func (c *Client) GetProxyHosts() ([]ProxyHost, error) {
	resp, err := c.doRequest("GET", "/api/nginx/proxy-hosts", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get proxy hosts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get proxy hosts failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var hosts []ProxyHost
	if err := json.NewDecoder(resp.Body).Decode(&hosts); err != nil {
		return nil, fmt.Errorf("failed to decode proxy hosts: %w", err)
	}

	return hosts, nil
}

// TestConnection tests if the NPM instance is reachable and credentials are valid
func (c *Client) TestConnection() error {
	_, err := c.getToken()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Try to get proxy hosts to verify full access
	_, err = c.GetProxyHosts()
	if err != nil {
		return fmt.Errorf("failed to access proxy hosts: %w", err)
	}

	return nil
}
