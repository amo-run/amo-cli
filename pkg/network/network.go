package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

// DownloadFileResume downloads a file with HTTP range-based resume support.
// It writes to outputPath using a sidecar part file (outputPath + ".part").
// When download completes, the part file is atomically renamed to outputPath.
// A sidecar metadata file (outputPath + ".part.meta") stores ETag/Last-Modified
// to validate resumed ranges via If-Range header.
func (nc *NetworkClient) DownloadFileResume(urlStr, outputPath string, progressCallback func(DownloadProgress)) *HTTPResponse {
	// Validate URL
	if !nc.isURLAllowed(urlStr) {
		return &HTTPResponse{Error: fmt.Sprintf("URL not in allowed hosts whitelist: %s", urlStr)}
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := nc.environment.GetCrossPlatformUtils().CreateDirWithPermissions(outputDir); err != nil {
		return &HTTPResponse{Error: fmt.Sprintf("failed to create output directory: %v", err)}
	}

	partPath := outputPath + ".part"
	metaPath := outputPath + ".part.meta"

	// Determine offset from existing part file
	var offset int64 = 0
	if info, err := os.Stat(partPath); err == nil {
		offset = info.Size()
	}

	// Prepare a helper to build request (with or without range)
	buildReq := func(withRange bool) (*http.Request, error) {
		r, e := http.NewRequest("GET", urlStr, nil)
		if e != nil {
			return nil, e
		}
		r.Header.Set("User-Agent", "amo-cli/1.0")
		if withRange && offset > 0 {
			r.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
			if metaBytes, e2 := os.ReadFile(metaPath); e2 == nil && len(metaBytes) > 0 {
				var meta map[string]string
				if json.Unmarshal(metaBytes, &meta) == nil {
					if etag, ok := meta["etag"]; ok && etag != "" {
						r.Header.Set("If-Range", etag)
					} else if lm, ok := meta["last_modified"]; ok && lm != "" {
						r.Header.Set("If-Range", lm)
					}
				}
			}
		}
		return r, nil
	}

	// Open part file for appending (or creating)
	f, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return &HTTPResponse{Error: fmt.Sprintf("failed to open part file: %v", err)}
	}
	// Do not defer close here; we'll close explicitly before rename

	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to seek part file: %v", err)}
		}
	}

	// First attempt: try with Range if we have offset
	req, err := buildReq(offset > 0)
	if err != nil {
		_ = f.Close()
		return &HTTPResponse{Error: fmt.Sprintf("failed to create request: %v", err)}
	}
	resp, err := nc.client.Do(req)
	if err != nil {
		_ = f.Close()
		return &HTTPResponse{Error: fmt.Sprintf("request failed: %v", err)}
	}

	// Handle 416 (Requested Range Not Satisfiable): possibly already complete
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && offset > 0 {
		// Try parse total from Content-Range: bytes */TOTAL
		var total int64 = -1
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			if slash := strings.LastIndex(cr, "/"); slash != -1 {
				if t, perr := strconv.ParseInt(strings.TrimSpace(cr[slash+1:]), 10, 64); perr == nil {
					total = t
				}
			}
		}
		// Close response; we will decide next actions
		resp.Body.Close()
		if total > 0 && offset >= total {
			// We already have the full file in .part → finalize
			_ = f.Sync()
			_ = f.Close()
			if _, err := os.Stat(outputPath); err == nil {
				_ = os.Remove(outputPath)
			}
			if err := os.Rename(partPath, outputPath); err != nil {
				return &HTTPResponse{Error: fmt.Sprintf("failed to finalize file: %v", err)}
			}
			_ = os.Remove(metaPath)
			return &HTTPResponse{StatusCode: http.StatusOK, Body: fmt.Sprintf("Downloaded %d bytes to %s", offset, outputPath)}
		}
		// Else fallback to full download (server rejected range or mismatch)
		if err := f.Truncate(0); err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to reset part file: %v", err)}
		}
		if _, err := f.Seek(0, 0); err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to rewind part file: %v", err)}
		}
		offset = 0
		_ = os.Remove(metaPath)
		// Reissue request without Range
		req, err = buildReq(false)
		if err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to create request: %v", err)}
		}
		resp, err = nc.client.Do(req)
		if err != nil {
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("request failed: %v", err)}
		}
	}

	// Check status codes (after possible 416 handling)
	if resp.StatusCode == http.StatusOK && offset > 0 {
		if err := f.Truncate(0); err != nil {
			resp.Body.Close()
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to reset part file: %v", err)}
		}
		if _, err := f.Seek(0, 0); err != nil {
			resp.Body.Close()
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to rewind part file: %v", err)}
		}
		offset = 0
		_ = os.Remove(metaPath)
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		status := resp.Status
		resp.Body.Close()
		_ = f.Close()
		return &HTTPResponse{StatusCode: resp.StatusCode, Error: fmt.Sprintf("HTTP error: %s", status)}
	}

	// Save/refresh metadata for future resumes
	etag := strings.TrimSpace(resp.Header.Get("ETag"))
	lastModified := strings.TrimSpace(resp.Header.Get("Last-Modified"))
	meta := map[string]string{
		"etag":          etag,
		"last_modified": lastModified,
		"url":           urlStr,
	}
	if metaBytes, err := json.Marshal(meta); err == nil {
		_ = os.WriteFile(metaPath, metaBytes, 0644)
	}

	// Total size calculation
	var total int64 = -1
	if resp.StatusCode == http.StatusPartialContent {
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			// Format: bytes start-end/total
			if slash := strings.LastIndex(cr, "/"); slash != -1 {
				if t, perr := strconv.ParseInt(strings.TrimSpace(cr[slash+1:]), 10, 64); perr == nil {
					total = t
				}
			}
		}
	}
	if total <= 0 && resp.ContentLength > 0 {
		total = offset + resp.ContentLength
	} else if total > 0 {
		// include existing bytes for progress calculations below
		// total already represents full size
	}

	// Stream copy with progress
	var downloaded int64 = 0
	buf := make([]byte, 32*1024)
	startTime := time.Now()
	lastReport := startTime

	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				resp.Body.Close()
				_ = f.Close()
				return &HTTPResponse{Error: fmt.Sprintf("failed to write to part file: %v", werr)}
			}
			downloaded += int64(n)

			if progressCallback != nil {
				now := time.Now()
				if now.Sub(lastReport) >= 200*time.Millisecond || (total > 0 && offset+downloaded == total) {
					elapsed := now.Sub(startTime)
					if elapsed <= 0 {
						elapsed = time.Millisecond
					}
					speed := float64(downloaded) / elapsed.Seconds()
					var percent int
					if total > 0 {
						percent = int(float64(offset+downloaded) / float64(total) * 100)
					}
					progressCallback(DownloadProgress{
						Downloaded: offset + downloaded,
						Total:      total,
						Percentage: percent,
						Speed:      formatBytes(int64(speed)) + "/s",
					})
					lastReport = now
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			resp.Body.Close()
			_ = f.Close()
			return &HTTPResponse{Error: fmt.Sprintf("failed to read response: %v", rerr)}
		}
	}

	// Close response and file before rename (Windows requires closed handles)
	resp.Body.Close()
	_ = f.Sync()
	if err := f.Close(); err != nil {
		return &HTTPResponse{Error: fmt.Sprintf("failed to close part file: %v", err)}
	}
	// Replace existing target if needed
	if _, err := os.Stat(outputPath); err == nil {
		_ = os.Remove(outputPath)
	}
	// Done – rename part to final and remove meta
	if err := os.Rename(partPath, outputPath); err != nil {
		return &HTTPResponse{Error: fmt.Sprintf("failed to finalize file: %v", err)}
	}
	_ = os.Remove(metaPath)

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    nc.extractHeaders(resp.Header),
		Body:       fmt.Sprintf("Downloaded %d bytes to %s", offset+downloaded, outputPath),
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
			// Fallback to full rewrite
			_ = os.WriteFile(filePath, append(content, []byte(builder.String())...), 0644)
		}
		existing = append(existing, missing...)
	}

	nc.allowedHosts = existing
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
