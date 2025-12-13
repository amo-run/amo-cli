package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"amo/pkg/env"
)

// NetworkClient provides secure HTTP client functionality
type NetworkClient struct {
	client         *http.Client
	environment    *env.Environment
	allowedHosts   []string
	allowedSchemes []string
}

// HTTPResponse represents the response from an HTTP request
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// NewNetworkClient creates a new network client with security controls
func NewNetworkClient() (*NetworkClient, error) {
	environment, err := env.NewEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize environment: %w", err)
	}

	client := &http.Client{
		Timeout: 600 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Limit redirects and check allowed hosts
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	nc := &NetworkClient{
		client:         client,
		environment:    environment,
		allowedSchemes: []string{"https", "http"},
	}

	// Load allowed hosts from whitelist
	if err := nc.loadAllowedHosts(); err != nil {
		return nil, fmt.Errorf("failed to load network whitelist: %w", err)
	}

	return nc, nil
}

// Get performs an HTTP GET request
func (nc *NetworkClient) Get(urlStr string, headers map[string]string) *HTTPResponse {
	return nc.request("GET", urlStr, nil, headers)
}

// Post performs an HTTP POST request
func (nc *NetworkClient) Post(urlStr string, body string, headers map[string]string) *HTTPResponse {
	return nc.request("POST", urlStr, strings.NewReader(body), headers)
}

// GetJSON performs a GET request and parses JSON response
func (nc *NetworkClient) GetJSON(urlStr string, headers map[string]string) map[string]interface{} {
	response := nc.Get(urlStr, headers)

	result := map[string]interface{}{
		"status_code": response.StatusCode,
		"headers":     response.Headers,
	}

	if response.Error != "" {
		result["error"] = response.Error
		return result
	}

	// Parse JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(response.Body), &jsonData); err != nil {
		result["error"] = fmt.Sprintf("failed to parse JSON: %v", err)
		result["raw_body"] = response.Body
	} else {
		result["data"] = jsonData
	}

	return result
}

// request performs the actual HTTP request
func (nc *NetworkClient) request(method, urlStr string, body io.Reader, headers map[string]string) *HTTPResponse {
	// Validate URL
	if !nc.isURLAllowed(urlStr) {
		return &HTTPResponse{
			Error: fmt.Sprintf("URL not in allowed hosts whitelist: %s", urlStr),
		}
	}

	// Create request
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	// Set default headers
	req.Header.Set("User-Agent", "amo-cli/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := nc.client.Do(req)
	if err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &HTTPResponse{
			StatusCode: resp.StatusCode,
			Headers:    nc.extractHeaders(resp.Header),
			Error:      fmt.Sprintf("failed to read response body: %v", err),
		}
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    nc.extractHeaders(resp.Header),
		Body:       string(bodyBytes),
	}
}

// isURLAllowed checks if a URL is in the allowed hosts whitelist
func (nc *NetworkClient) isURLAllowed(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Check scheme
	schemeAllowed := false
	for _, allowedScheme := range nc.allowedSchemes {
		if parsedURL.Scheme == allowedScheme {
			schemeAllowed = true
			break
		}
	}
	if !schemeAllowed {
		return false
	}

	// If no hosts are configured, allow all (for initial setup)
	if len(nc.allowedHosts) == 0 {
		return true
	}

	// Check host and path using domain and path matching pattern
	host := parsedURL.Hostname()
	urlPath := parsedURL.Path

	for _, allowedEntry := range nc.allowedHosts {
		// Check if the allowed entry contains a path
		hostPart := allowedEntry
		pathPart := ""

		if strings.Contains(allowedEntry, "/") {
			parts := strings.SplitN(allowedEntry, "/", 2)
			hostPart = parts[0]
			pathPart = "/" + parts[1]
		}

		// First check if hostname matches
		hostnameMatches := false
		if host == hostPart {
			hostnameMatches = true
		} else if strings.HasSuffix(host, "."+hostPart) {
			// Domain suffix match (e.g., "github.com" matches "api.github.com" or "user.github.com")
			hostnameMatches = true
		}

		// If hostname matches, check path if necessary
		if hostnameMatches {
			if pathPart == "" {
				// No path restriction in this entry, allow access
				return true
			} else {
				// Path restriction exists, check if URL path starts with the allowed path
				// Make sure we match exact paths or subdirectories, not partial path segments
				if strings.HasPrefix(urlPath, pathPart) &&
					(len(urlPath) == len(pathPart) || urlPath[len(pathPart)] == '/' || pathPart[len(pathPart)-1] == '/') {
					return true
				}
				// Path doesn't match, continue checking other entries
			}
		}
	}

	return false
}

// loadAllowedHosts loads the allowed hosts from the whitelist file
func (nc *NetworkClient) loadAllowedHosts() error {
	filePath := nc.environment.JoinPath(nc.environment.GetUserConfigDir(), "allowed_hosts.txt")

	// Keep a single source of truth for default hosts
	defaultHosts := []string{
		"github.com",
		"raw.githubusercontent.com",
		"gitlab.com",
		"bitbucket.org",
		"sourceforge.net",
		"ffmpeg.org",
		"imagemagick.org",
		"calibre-ebook.com",
		"ghostscript.com",
		"toolchains.mirror.toulan.fun",
	}

	// Create file if it doesn't exist (bootstrap with defaults + docs)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		content := "# Allowed hosts for network access - one domain or domain/path per line\n"
		content += "# Domain and path matching rules:\n"
		content += "# - \"github.com\" matches github.com itself and any subdomain like api.github.com with any path\n"
		content += "# - \"github.com/nodewee\" matches only github.com/nodewee and any path under it (e.g., github.com/nodewee/project)\n"
		content += "# - \"api.github.com\" matches only api.github.com; subdomains are also matched by suffix rule\n"
		content += "# - \"api.github.com/v3\" matches only api.github.com/v3 and paths under it\n"
		content += "# - To restrict access to specific paths only, include the path in the entry\n"
		content += "# Example entries:\n"
		for _, host := range defaultHosts {
			content += host + "\n"
		}

		if err := nc.environment.GetCrossPlatformUtils().CreateFileWithPermissions(filePath, []byte(content), false); err != nil {
			return fmt.Errorf("failed to create network whitelist file: %w", err)
		}
	}

	// Read existing file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read network whitelist: %w", err)
	}

	// Parse existing hosts (ignore comments/blank lines)
	lines := strings.Split(string(content), "\n")
	existing := make([]string, 0, len(lines))
	seen := make(map[string]bool)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !seen[line] {
			existing = append(existing, line)
			seen[line] = true
		}
	}

	// Also merge allowed workflow download sources to honor `amo workflow source` configuration
	// File: allowed_workflow_hosts.txt (same directory)
	wfFilePath := nc.environment.JoinPath(nc.environment.GetUserConfigDir(), "allowed_workflow_hosts.txt")
	if _, err := os.Stat(wfFilePath); err == nil {
		if wfContent, rerr := os.ReadFile(wfFilePath); rerr == nil {
			wfLines := strings.Split(string(wfContent), "\n")
			for _, l := range wfLines {
				l = strings.TrimSpace(l)
				if l == "" || strings.HasPrefix(l, "#") {
					continue
				}
				if !seen[l] {
					existing = append(existing, l)
					seen[l] = true
				}
			}
		}
	}

	// Auto-heal: ensure new defaults are present even if user's file was created earlier
	missing := make([]string, 0)
	for _, host := range defaultHosts {
		if !seen[host] {
			missing = append(missing, host)
		}
	}

	if len(missing) > 0 {
		// Append missing defaults without touching user's existing entries
		builder := strings.Builder{}
		// Ensure separation from previous content
		if len(content) > 0 && content[len(content)-1] != '\n' {
			builder.WriteString("\n")
		}
		builder.WriteString("# Auto-added default hosts to keep amo-cli up-to-date\n")
		for _, host := range missing {
			builder.WriteString(host)
			builder.WriteString("\n")
		}
		// Try append mode first to preserve existing file content
		if f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644); err == nil {
			_, _ = f.WriteString(builder.String())
			_ = f.Close()
		} else {
			_ = os.WriteFile(filePath, append(content, []byte(builder.String())...), 0644)
		}
		existing = append(existing, missing...)
	}

	nc.allowedHosts = existing
	return nil
}

func (nc *NetworkClient) extractHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}
