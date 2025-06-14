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
var AllowedDomains = []string{
	"github.com",
	"raw.githubusercontent.com",
	"gitlab.com",
	"bitbucket.org",
	"sourceforge.net",
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

	// Check against allowed domains
	for _, allowedDomain := range AllowedDomains {
		if hostname == allowedDomain || strings.HasSuffix(hostname, "."+allowedDomain) {
			return nil
		}
	}

	return fmt.Errorf("domain %s is not in the allowed list: %v", hostname, AllowedDomains)
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

// DownloadWorkflow downloads a workflow script from the given URL
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

	// Download the file
	resp, err := wd.client.Get(rawURL)
	if err != nil {
		return fmt.Errorf("%s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Read the content
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
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
