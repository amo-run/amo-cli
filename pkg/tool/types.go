package tool

import (
	"fmt"
	"strings"
)

// ToolConfig represents the configuration for all tools
type ToolConfig struct {
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Tools       map[string]Tool        `json:"tools"`
	Config      map[string]interface{} `json:"config"`
}

// Tool represents a single tool configuration
type Tool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Category     string                 `json:"category"`
	Website      string                 `json:"website"`
	Check        CheckConfig            `json:"check"`
	Install      map[string]InstallInfo `json:"install"`
	DarwinBinary string                 `json:"darwin_binary,omitempty"`
}

// CheckConfig represents tool verification configuration
type CheckConfig struct {
	Command          string   `json:"command"`
	Args             []string `json:"args"`
	Pattern          string   `json:"pattern,omitempty"`
	FallbackCommands []string `json:"fallback_commands,omitempty"`
}

// InstallInfo represents installation information for a platform
type InstallInfo struct {
	Method      string            `json:"method"`
	Package     string            `json:"package,omitempty"`
	Packages    map[string]string `json:"packages,omitempty"`
	URL         string            `json:"url,omitempty"`
	Python      string            `json:"python,omitempty"`
	Repo        string            `json:"repo,omitempty"`         // GitHub repository (e.g., "owner/repo")
	Pattern     string            `json:"pattern,omitempty"`      // Asset filename pattern with placeholders
	Target      string            `json:"target,omitempty"`       // Target executable name after extraction
	Workflow    string            `json:"workflow,omitempty"`     // Workflow name for workflow-based installation
	PortableURL string            `json:"portable_url,omitempty"` // URL for portable versions
	MirrorURL   string            `json:"mirror_url,omitempty"`   // Mirror URL for downloads
}

// ToolStatus represents the status of a tool
type ToolStatus struct {
	Name      string `json:"name"`
	Command   string `json:"command"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Error     string `json:"error,omitempty"`
}

// ToolPathCache represents cached tool paths
type ToolPathCache struct {
	Version   string            `json:"version"`
	Timestamp int64             `json:"timestamp"`
	Paths     map[string]string `json:"paths"`
}

// FormatToolStatus formats tool status for display
func FormatToolStatus(status ToolStatus) string {
	if status.Installed {
		version := status.Version
		if version == "" {
			version = "unknown"
		}
		return fmt.Sprintf("✅ %s (%s) - installed (%s)", status.Command, status.Name, version)
	}

	if status.Error != "" {
		if strings.Contains(status.Error, "command failed") {
			return fmt.Sprintf("❌ %s (%s) - not installed", status.Command, status.Name)
		}
		return fmt.Sprintf("❌ %s (%s) - %s", status.Command, status.Name, status.Error)
	}

	return fmt.Sprintf("❌ %s (%s) - not installed", status.Command, status.Name)
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []GitHubReleaseAsset `json:"assets"`
}

// GitHubReleaseAsset represents a GitHub release asset
type GitHubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}
