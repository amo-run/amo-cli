package workflow

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"amo/pkg/env"

	"github.com/spf13/viper"
)

// AllowedDomains defines the whitelist of allowed domains for workflow downloads
// Uses domain suffix matching pattern:
// - "github.com" matches github.com itself and any subdomain like api.github.com
// - "raw.githubusercontent.com" matches itself and subdomains like cdn.raw.githubusercontent.com
var AllowedDomains = []string{
	"github.com",
	"raw.githubusercontent.com",
	"gitlab.com",
	"bitbucket.org",
	"sourceforge.net",
	"toolchains.mirror.toulan.fun",
}

// WorkflowDownloader handles downloading workflow scripts from allowed sources
type WorkflowDownloader struct {
	env    *env.Environment
	client *http.Client
}

// NewWorkflowDownloader creates a new workflow downloader
func NewWorkflowDownloader() (*WorkflowDownloader, error) {
	environment, err := env.NewEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize environment: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &WorkflowDownloader{
		env:    environment,
		client: client,
	}, nil
}

// GetWorkflowsDir returns the default workflows directory
func (wd *WorkflowDownloader) GetWorkflowsDir() string {
	return wd.env.GetCrossPlatformUtils().JoinPath(wd.env.GetUserConfigDir(), "workflows")
}

// EnsureWorkflowsDir ensures the workflows directory exists
func (wd *WorkflowDownloader) EnsureWorkflowsDir() error {
	workflowsDir := wd.GetWorkflowsDir()
	return wd.env.GetCrossPlatformUtils().CreateDirWithPermissions(workflowsDir)
}

// GetConfiguredWorkflowsDir attempts to get the configured workflows directory
// without directly importing the config package (to avoid circular imports)
// If no configured directory is found, returns an empty string
func (wd *WorkflowDownloader) GetConfiguredWorkflowsDir() string {
	// We can't directly import the config package due to circular references,
	// so we'll directly use viper to read from the config file

	// Try environment variable first (useful for testing)
	configuredDir := os.Getenv("AMO_WORKFLOWS_DIR")
	if configuredDir != "" {
		return wd.env.GetCrossPlatformUtils().NormalizePath(configuredDir)
	}

	// Use viper to read directly from the config file
	configDir := wd.env.GetUserConfigDir()
	configFile := filepath.Join(configDir, "config.yaml")

	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	// If config file doesn't exist, return empty string
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return ""
	}

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		// If there's an error reading, just return empty
		return ""
	}

	// Get the workflows directory setting
	configuredDir = v.GetString("workflows")
	if configuredDir != "" {
		return wd.env.GetCrossPlatformUtils().NormalizePath(configuredDir)
	}

	return ""
}

// IsValidURL checks if the URL is from an allowed domain
func (wd *WorkflowDownloader) IsValidURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTP and HTTPS URLs are allowed")
	}

	hostname := strings.ToLower(parsedURL.Hostname())

	// Check against allowed domains using domain and path matching pattern
	urlPath := parsedURL.Path

	for _, allowedEntry := range AllowedDomains {
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
		if hostname == hostPart {
			hostnameMatches = true
		} else if strings.HasSuffix(hostname, "."+hostPart) {
			// Domain suffix match (e.g., "github.com" matches "api.github.com")
			hostnameMatches = true
		}

		// If hostname matches, check path if necessary
		if hostnameMatches {
			if pathPart == "" {
				// No path restriction in this entry, allow access
				return nil
			} else {
				// Path restriction exists, check if URL path starts with the allowed path
				// Make sure we match exact paths or subdirectories, not partial path segments
				if strings.HasPrefix(urlPath, pathPart) &&
					(len(urlPath) == len(pathPart) || urlPath[len(pathPart)] == '/' || pathPart[len(pathPart)-1] == '/') {
					return nil
				}
				// Path doesn't match, continue checking other entries
			}
		}
	}

	return fmt.Errorf("URL with domain %s and path %s is not in the allowed list", hostname, urlPath)
}

// ConvertToRawURL converts GitHub/GitLab URLs to raw content URLs
func (wd *WorkflowDownloader) ConvertToRawURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	hostname := strings.ToLower(parsedURL.Hostname())

	// Convert GitHub URLs to raw.githubusercontent.com
	if hostname == "github.com" {
		// Pattern: https://github.com/owner/repo/blob/branch/path/to/file.js
		// Convert to: https://raw.githubusercontent.com/owner/repo/branch/path/to/file.js
		path := parsedURL.Path
		if strings.Contains(path, "/blob/") {
			path = strings.Replace(path, "/blob/", "/", 1)
			return fmt.Sprintf("https://raw.githubusercontent.com%s", path), nil
		}
		return urlStr, nil
	}

	// Convert GitLab URLs to raw content
	if hostname == "gitlab.com" || strings.HasSuffix(hostname, ".gitlab.com") {
		// Pattern: https://gitlab.com/owner/repo/-/blob/branch/path/to/file.js
		// Convert to: https://gitlab.com/owner/repo/-/raw/branch/path/to/file.js
		path := parsedURL.Path
		if strings.Contains(path, "/-/blob/") {
			path = strings.Replace(path, "/-/blob/", "/-/raw/", 1)
			return fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, path), nil
		}
		return urlStr, nil
	}

	// For other domains, return as-is
	return urlStr, nil
}

// ExtractFilename extracts the filename from URL
func (wd *WorkflowDownloader) ExtractFilename(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	filename := filepath.Base(parsedURL.Path)
	if filename == "." || filename == "/" {
		return "", fmt.Errorf("could not extract filename from URL")
	}

	// Ensure .js extension
	if !strings.HasSuffix(strings.ToLower(filename), ".js") {
		filename += ".js"
	}

	// Sanitize filename
	filename = wd.sanitizeFilename(filename)

	return filename, nil
}

// sanitizeFilename removes invalid characters from filename
func (wd *WorkflowDownloader) sanitizeFilename(filename string) string {
	// Remove or replace invalid characters
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	filename = reg.ReplaceAllString(filename, "_")

	// Remove leading/trailing spaces and dots
	filename = strings.Trim(filename, " .")

	// Ensure filename is not empty
	if filename == "" {
		filename = "workflow.js"
	}

	return filename
}

// DownloadWorkflow downloads a workflow script from the given URL with mirror fallback
func (wd *WorkflowDownloader) DownloadWorkflow(urlStr string, filename string) error {
	// Validate URL
	if err := wd.IsValidURL(urlStr); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	// Convert to raw URL if needed
	rawURL, err := wd.ConvertToRawURL(urlStr)
	if err != nil {
		return fmt.Errorf("failed to convert URL: %w", err)
	}

	// Extract filename if not provided
	if filename == "" {
		filename, err = wd.ExtractFilename(rawURL)
		if err != nil {
			return fmt.Errorf("failed to extract filename: %w", err)
		}
	} else {
		// Sanitize provided filename
		filename = wd.sanitizeFilename(filename)
		if !strings.HasSuffix(strings.ToLower(filename), ".js") {
			filename += ".js"
		}
	}

	// Always use the default workflows directory for downloads
	// regardless of whether a custom workflows directory is configured
	workflowsDir := wd.GetWorkflowsDir()

	// Ensure workflows directory exists
	if err := wd.EnsureWorkflowsDir(); err != nil {
		return fmt.Errorf("failed to create workflows directory: %w", err)
	}

	// Try original URL first, then mirror if it fails
	content, err := wd.downloadFromURL(rawURL)
	if err != nil {
		fmt.Printf("⚠️  Original URL failed: %v\n", err)

		// Try mirror site if original URL is from GitHub
		parsedURL, parseErr := url.Parse(rawURL)
		if parseErr == nil && wd.isGitHubURL(parsedURL) {
			fmt.Printf("🔄 Trying mirror site: toolchains.mirror.toulan.fun\n")

			mirrorURL, mirrorErr := wd.convertToMirrorURL(rawURL)
			if mirrorErr == nil {
				content, err = wd.downloadFromURL(mirrorURL)
				if err != nil {
					return fmt.Errorf("both original and mirror download failed: original=%v, mirror=%v", err, err)
				}
				fmt.Printf("✅ Successfully downloaded from mirror site\n")
			} else {
				return fmt.Errorf("original download failed and mirror URL conversion failed: original=%v, mirror=%v", err, mirrorErr)
			}
		} else {
			return fmt.Errorf("download failed: %w", err)
		}
	}

	// Validate it's a valid amo workflow
	contentStr := string(content)
	if !strings.HasPrefix(strings.TrimSpace(contentStr), "//!amo") {
		return fmt.Errorf("downloaded file is not a valid amo workflow (must start with //!amo)")
	}

	// Save to workflows directory
	workflowPath := wd.env.GetCrossPlatformUtils().JoinPath(workflowsDir, filename)

	err = wd.env.GetCrossPlatformUtils().CreateFileWithPermissions(workflowPath, content, false)
	if err != nil {
		return fmt.Errorf("failed to save workflow file: %w", err)
	}

	return nil
}

// downloadFromURL downloads content from a given URL
func (wd *WorkflowDownloader) downloadFromURL(urlStr string) ([]byte, error) {
	resp, err := wd.client.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Read the content
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return content, nil
}

// isGitHubURL checks if the URL is from GitHub
func (wd *WorkflowDownloader) isGitHubURL(parsedURL *url.URL) bool {
	hostname := strings.ToLower(parsedURL.Hostname())
	return hostname == "github.com" || hostname == "raw.githubusercontent.com" || strings.HasSuffix(hostname, ".github.com")
}

// convertToMirrorURL converts a GitHub URL to mirror site URL
func (wd *WorkflowDownloader) convertToMirrorURL(githubURL string) (string, error) {
	parsedURL, err := url.Parse(githubURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	path := parsedURL.Path

	// Handle raw.githubusercontent.com URLs
	// Format: https://raw.githubusercontent.com/owner/repo/branch/path/to/file.js
	// Convert to: https://toolchains.mirror.toulan.fun/owner/repo/latest/file.js
	if hostname == "raw.githubusercontent.com" {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 4 {
			owner := parts[0]
			repo := parts[1]
			// Skip branch (parts[2]) and use "latest"
			filename := parts[len(parts)-1] // Get the last part as filename

			mirrorURL := fmt.Sprintf("https://toolchains.mirror.toulan.fun/%s/%s/latest/%s",
				owner, repo, filename)
			return mirrorURL, nil
		}
	}

	// Handle github.com URLs (shouldn't happen after ConvertToRawURL, but just in case)
	// Format: https://github.com/owner/repo/blob/branch/path/to/file.js
	if hostname == "github.com" && strings.Contains(path, "/blob/") {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 5 {
			owner := parts[0]
			repo := parts[1]
			// Skip "blob" (parts[2]) and branch (parts[3])
			filename := parts[len(parts)-1] // Get the last part as filename

			mirrorURL := fmt.Sprintf("https://toolchains.mirror.toulan.fun/%s/%s/latest/%s",
				owner, repo, filename)
			return mirrorURL, nil
		}
	}

	return "", fmt.Errorf("unsupported GitHub URL format: %s", githubURL)
}

// ListUserWorkflows returns a list of user-downloaded workflow files from both
// the default downloads directory and the configured workflows directory
func (wd *WorkflowDownloader) ListUserWorkflows() ([]string, error) {
	// Create a map to avoid duplicates when filenames are the same
	workflowMap := make(map[string]bool)
	var err1, err2 error

	// 1. First list workflows from the default directory
	defaultWorkflowsDir := wd.GetWorkflowsDir()
	if _, statErr := os.Stat(defaultWorkflowsDir); !os.IsNotExist(statErr) {
		entries, err := os.ReadDir(defaultWorkflowsDir)
		if err != nil {
			err1 = fmt.Errorf("failed to read default workflows directory: %w", err)
		} else {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".js") {
					workflowMap[entry.Name()] = true
				}
			}
		}
	}

	// 2. Then list workflows from the configured directory (if different)
	configuredDir := wd.GetConfiguredWorkflowsDir()
	if configuredDir != "" && configuredDir != defaultWorkflowsDir {
		if _, statErr := os.Stat(configuredDir); !os.IsNotExist(statErr) {
			entries, err := os.ReadDir(configuredDir)
			if err != nil {
				err2 = fmt.Errorf("failed to read configured workflows directory: %w", err)
			} else {
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".js") {
						workflowMap[entry.Name()] = true
					}
				}
			}
		}
	}

	// If both directories failed to read, return the errors
	if err1 != nil && err2 != nil {
		return nil, fmt.Errorf("failed to list workflows: %v; %v", err1, err2)
	} else if err1 != nil {
		return nil, err1
	} else if err2 != nil {
		return nil, err2
	}

	// Convert the map to a slice
	workflows := make([]string, 0, len(workflowMap))
	for workflow := range workflowMap {
		workflows = append(workflows, workflow)
	}

	// Sort for consistent output
	sort.Strings(workflows)

	return workflows, nil
}
