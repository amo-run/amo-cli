package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

// DownloadProgress represents download progress information
type DownloadProgress struct {
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percentage int    `json:"percentage"`
	Speed      string `json:"speed"`
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

// DownloadFile downloads a file from URL to the specified path
func (nc *NetworkClient) DownloadFile(urlStr, outputPath string, progressCallback func(DownloadProgress)) *HTTPResponse {
	// Validate URL
	if !nc.isURLAllowed(urlStr) {
		return &HTTPResponse{
			Error: fmt.Sprintf("URL not in allowed hosts whitelist: %s", urlStr),
		}
	}

	// Create HTTP request
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	// Set user agent
	req.Header.Set("User-Agent", "amo-cli/1.0")

	// Execute request
	resp, err := nc.client.Do(req)
	if err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPResponse{
			StatusCode: resp.StatusCode,
			Error:      fmt.Sprintf("HTTP error: %s", resp.Status),
		}
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := nc.environment.GetCrossPlatformUtils().CreateDirWithPermissions(outputDir); err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("failed to create output directory: %v", err),
		}
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return &HTTPResponse{
			Error: fmt.Sprintf("failed to create output file: %v", err),
		}
	}
	defer outFile.Close()

	// Get content length for progress tracking
	contentLength := resp.ContentLength

	// Copy with progress tracking
	var downloaded int64
	buffer := make([]byte, 32*1024) // 32KB buffer
	startTime := time.Now()

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := outFile.Write(buffer[:n]); writeErr != nil {
				return &HTTPResponse{
					Error: fmt.Sprintf("failed to write to file: %v", writeErr),
				}
			}
			downloaded += int64(n)

			// Report progress if callback is provided
			if progressCallback != nil && contentLength > 0 {
				elapsed := time.Since(startTime)
				speed := float64(downloaded) / elapsed.Seconds()
				percentage := int(float64(downloaded) / float64(contentLength) * 100)

				progress := DownloadProgress{
					Downloaded: downloaded,
					Total:      contentLength,
					Percentage: percentage,
					Speed:      formatBytes(int64(speed)) + "/s",
				}
				progressCallback(progress)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return &HTTPResponse{
				Error: fmt.Sprintf("failed to read response: %v", err),
			}
		}
	}

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    nc.extractHeaders(resp.Header),
		Body:       fmt.Sprintf("Downloaded %d bytes to %s", downloaded, outputPath),
	}
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

	// Check host
	host := parsedURL.Hostname()
	for _, allowedHost := range nc.allowedHosts {
		if host == allowedHost || strings.HasSuffix(host, "."+allowedHost) {
			return true
		}
	}

	return false
}

// loadAllowedHosts loads the allowed hosts from the whitelist file
func (nc *NetworkClient) loadAllowedHosts() error {
	filePath := nc.environment.JoinPath(nc.environment.GetUserConfigDir(), "allowed_hosts.txt")

	// Create file if it doesn't exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		defaultHosts := []string{
			"github.com",
			"api.github.com",
			"releases.ubuntu.com",
			"download.imagemagick.org",
			"calibre-ebook.com",
			"www.ghostscript.com",
			"github.com/jgm/pandoc",
		}

		content := "# Allowed hosts for network access - one per line\n"
		content += "# Examples:\n"
		for _, host := range defaultHosts {
			content += host + "\n"
		}

		if err := nc.environment.GetCrossPlatformUtils().CreateFileWithPermissions(filePath, []byte(content), false); err != nil {
			return fmt.Errorf("failed to create network whitelist file: %w", err)
		}
	}

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read network whitelist: %w", err)
	}

	// Parse hosts
	lines := strings.Split(string(content), "\n")
	nc.allowedHosts = []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			nc.allowedHosts = append(nc.allowedHosts, line)
		}
	}

	return nil
}

// extractHeaders extracts headers from HTTP response
func (nc *NetworkClient) extractHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// formatBytes formats bytes into human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
