package tool

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"amo/pkg/env"
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
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Pattern string   `json:"pattern,omitempty"`
}

// InstallInfo represents installation information for a platform
type InstallInfo struct {
	Method   string            `json:"method"`
	Package  string            `json:"package,omitempty"`
	Packages map[string]string `json:"packages,omitempty"`
	URL      string            `json:"url,omitempty"`
	Python   string            `json:"python,omitempty"`
	Repo     string            `json:"repo,omitempty"`    // GitHub repository (e.g., "owner/repo")
	Pattern  string            `json:"pattern,omitempty"` // Asset filename pattern with placeholders
	Target   string            `json:"target,omitempty"`  // Target executable name after extraction
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

// Manager handles tool management operations
type Manager struct {
	config      *ToolConfig
	environment *env.Environment
	pathCache   *ToolPathCache
}

// NewManager creates a new tool manager
func NewManager() (*Manager, error) {
	env, err := env.NewEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize environment: %w", err)
	}

	manager := &Manager{
		environment: env,
	}

	// Load tool path cache
	if err := manager.loadPathCache(); err != nil {
		// If cache loading fails, create a new empty cache
		manager.pathCache = &ToolPathCache{
			Version:   "1.0.0",
			Timestamp: time.Now().Unix(),
			Paths:     make(map[string]string),
		}
	}

	return manager, nil
}

// LoadConfig loads tool configuration from embedded assets
func (m *Manager) LoadConfig(configData []byte) error {
	var config ToolConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse tool configuration: %w", err)
	}

	m.config = &config
	return nil
}

// getToolPathCacheFile returns the path to the tool path cache file
func (m *Manager) getToolPathCacheFile() string {
	return m.environment.GetCrossPlatformUtils().JoinPath(m.environment.GetUserConfigDir(), "tool_paths.json")
}

// loadPathCache loads the tool path cache from file
func (m *Manager) loadPathCache() error {
	cacheFile := m.getToolPathCacheFile()

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cache file does not exist")
		}
		return fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache ToolPathCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return fmt.Errorf("failed to parse cache file: %w", err)
	}

	m.pathCache = &cache
	return nil
}

// savePathCache saves the tool path cache to file
func (m *Manager) savePathCache() error {
	if m.pathCache == nil {
		return fmt.Errorf("path cache is nil")
	}

	cacheFile := m.getToolPathCacheFile()

	// Update timestamp
	m.pathCache.Timestamp = time.Now().Unix()

	data, err := json.MarshalIndent(m.pathCache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// getCachedToolPath returns the cached path for a tool
func (m *Manager) getCachedToolPath(toolName string) (string, bool) {
	if m.pathCache == nil {
		return "", false
	}

	path, exists := m.pathCache.Paths[toolName]
	return path, exists
}

// GetCachedToolPath returns the cached path for a tool (public method)
func (m *Manager) GetCachedToolPath(toolName string) (string, bool) {
	return m.getCachedToolPath(toolName)
}

// setCachedToolPath sets the cached path for a tool
func (m *Manager) setCachedToolPath(toolName, path string) {
	if m.pathCache == nil {
		m.pathCache = &ToolPathCache{
			Version:   "1.0.0",
			Timestamp: time.Now().Unix(),
			Paths:     make(map[string]string),
		}
	}

	m.pathCache.Paths[toolName] = path
}

// clearCachedToolPath removes the cached path for a tool
func (m *Manager) clearCachedToolPath(toolName string) {
	if m.pathCache != nil {
		delete(m.pathCache.Paths, toolName)
	}
}

// findToolExecutable searches for tool executable in common locations
func (m *Manager) findToolExecutable(tool Tool) string {
	// First check cached path
	if cachedPath, exists := m.getCachedToolPath(tool.Check.Command); exists {
		if _, err := os.Stat(cachedPath); err == nil {
			return cachedPath
		}
		// Remove invalid cached path
		m.clearCachedToolPath(tool.Check.Command)
	}

	// Check custom install directory first
	installDir := m.getInstallDir()
	customPath := filepath.Join(installDir, tool.Check.Command)
	if runtime.GOOS == "windows" && !strings.HasSuffix(customPath, ".exe") {
		customPath += ".exe"
	}
	if _, err := os.Stat(customPath); err == nil {
		m.setCachedToolPath(tool.Check.Command, customPath)
		return customPath
	}

	// Special handling for macOS binary paths
	if tool.DarwinBinary != "" && runtime.GOOS == "darwin" {
		if _, err := os.Stat(tool.DarwinBinary); err == nil {
			m.setCachedToolPath(tool.Check.Command, tool.DarwinBinary)
			return tool.DarwinBinary
		}
	}

	// Use exec.LookPath to find the executable
	if path, err := exec.LookPath(tool.Check.Command); err == nil {
		m.setCachedToolPath(tool.Check.Command, path)
		return path
	}

	return tool.Check.Command // fallback to original command
}

// ListTools returns all available tools with their status
func (m *Manager) ListTools() ([]ToolStatus, error) {
	if m.config == nil {
		return nil, fmt.Errorf("tool configuration not loaded")
	}

	var tools []ToolStatus
	for toolName, tool := range m.config.Tools {
		status := m.checkToolStatus(toolName, tool)
		tools = append(tools, status)
	}

	// Save path cache after checking all tools
	if err := m.savePathCache(); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: failed to save tool path cache: %v\n", err)
	}

	return tools, nil
}

// CheckTool checks the status of a specific tool
func (m *Manager) CheckTool(toolName string) (*ToolStatus, error) {
	if m.config == nil {
		return nil, fmt.Errorf("tool configuration not loaded")
	}

	tool, exists := m.config.Tools[toolName]
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found", toolName)
	}

	status := m.checkToolStatus(toolName, tool)

	// Save path cache after checking
	if err := m.savePathCache(); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: failed to save tool path cache: %v\n", err)
	}

	return &status, nil
}

// checkToolStatus performs the actual tool status check
func (m *Manager) checkToolStatus(toolName string, tool Tool) ToolStatus {
	status := ToolStatus{
		Name:      tool.Name,
		Command:   tool.Check.Command,
		Installed: false,
		Version:   "",
		Error:     "",
	}

	// NOTE: Tool management bypasses workflow CLI whitelist for security isolation
	// Tool management operates independently of workflow command restrictions

	// Find tool executable (with caching)
	command := m.findToolExecutable(tool)

	// Execute check command
	args := tool.Check.Args
	if len(args) == 0 {
		args = []string{"--version"}
	}

	// No timeout restriction for tool management operations
	cmd := exec.Command(command, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		status.Error = fmt.Sprintf("command failed: %v", err)
		// Clear cached path if command failed
		m.clearCachedToolPath(tool.Check.Command)
		return status
	}

	outputStr := string(output)

	// Extract version if pattern is provided
	if tool.Check.Pattern != "" {
		// Special case for simple text matching (like "Usage:" for surya_ocr)
		if tool.Check.Pattern == "Usage:" {
			if strings.Contains(outputStr, "Usage:") {
				status.Installed = true
				status.Version = "available"
			}
		} else {
			// Regular regex pattern matching
			re, err := regexp.Compile(tool.Check.Pattern)
			if err != nil {
				status.Error = fmt.Sprintf("invalid version pattern: %v", err)
				return status
			}

			matches := re.FindStringSubmatch(outputStr)
			if len(matches) >= 2 {
				status.Version = matches[1]
				status.Installed = true
			}
		}
	} else if len(outputStr) > 0 {
		// If no pattern but command succeeded with output
		status.Installed = true
		status.Version = "unknown"
	}

	return status
}

// InstallTool installs a specific tool (automatic installation by default)
func (m *Manager) InstallTool(toolName string, forceReinstall bool) error {
	if m.config == nil {
		return fmt.Errorf("tool configuration not loaded")
	}

	tool, exists := m.config.Tools[toolName]
	if !exists {
		return fmt.Errorf("tool '%s' not found", toolName)
	}

	// Check if already installed
	status := m.checkToolStatus(toolName, tool)
	if status.Installed && !forceReinstall {
		return fmt.Errorf("tool '%s' is already installed (version %s)", tool.Name, status.Version)
	}

	// Get installation info for current platform
	osName := runtime.GOOS
	installInfo, exists := tool.Install[osName]
	if !exists {
		return fmt.Errorf("no installation method available for %s", osName)
	}

	// Perform installation based on method (automatic by default)
	switch installInfo.Method {
	case "homebrew":
		return m.installViaHomebrew(installInfo.Package)
	case "package":
		return m.installViaPackageManager(installInfo.Packages)
	case "pip":
		return m.installViaPip(installInfo.Package)
	case "github":
		return m.installViaGitHub(toolName, installInfo)
	case "download":
		return m.installViaDownload(toolName, installInfo)
	case "installer":
		return m.installViaInstaller(installInfo)
	default:
		return fmt.Errorf("installation method '%s' not implemented yet", installInfo.Method)
	}
}

// GetToolPathCacheInfo returns information about the tool path cache
func (m *Manager) GetToolPathCacheInfo() map[string]interface{} {
	info := map[string]interface{}{
		"cache_file": m.getToolPathCacheFile(),
		"version":    "unknown",
		"timestamp":  "unknown",
		"tool_count": 0,
	}

	if m.pathCache != nil {
		info["version"] = m.pathCache.Version
		info["timestamp"] = time.Unix(m.pathCache.Timestamp, 0).Format("2006-01-02 15:04:05")
		info["tool_count"] = len(m.pathCache.Paths)
	}

	return info
}

// GetCachedToolPaths returns all cached tool command-to-path mappings
func (m *Manager) GetCachedToolPaths() map[string]string {
	if m.pathCache == nil {
		return make(map[string]string)
	}

	// Return a copy to avoid external modification
	paths := make(map[string]string)
	for command, path := range m.pathCache.Paths {
		paths[command] = path
	}

	return paths
}

// installViaHomebrew installs a tool using Homebrew
func (m *Manager) installViaHomebrew(packageName string) error {
	// NOTE: Tool management bypasses workflow CLI whitelist restrictions

	// Check if brew exists
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("Homebrew not found. Install from: https://brew.sh/")
	}

	// Install package - no timeout restriction for tool management
	cmd := exec.Command("brew", "install", packageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("homebrew installation failed: %v", err)
	}

	return nil
}

// installViaPackageManager installs a tool using system package manager
func (m *Manager) installViaPackageManager(packages map[string]string) error {
	packageManagers := []string{"apt", "yum", "pacman"}

	for _, pm := range packageManagers {
		packageName, exists := packages[pm]
		if !exists {
			continue
		}

		// NOTE: Tool management bypasses workflow CLI whitelist restrictions
		// Check if package manager exists
		if _, err := exec.LookPath(pm); err != nil {
			continue
		}

		// Install using this package manager - no timeout restriction for tool management
		var cmd *exec.Cmd
		switch pm {
		case "apt":
			cmd = exec.Command("sudo", "apt", "update")
			cmd.Run() // Update first, ignore errors
			cmd = exec.Command("sudo", "apt", "install", "-y", packageName)
		case "yum":
			cmd = exec.Command("sudo", "yum", "install", "-y", packageName)
		case "pacman":
			cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", packageName)
		default:
			continue
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s installation failed: %v", pm, err)
		}

		return nil
	}

	return fmt.Errorf("no suitable package manager found or allowed")
}

// installViaPip installs a tool using pip
func (m *Manager) installViaPip(packageName string) error {
	pipCommands := []string{"pip3", "pip"}

	for _, pip := range pipCommands {
		// NOTE: Tool management bypasses workflow CLI whitelist restrictions
		// Check if pip exists
		if _, err := exec.LookPath(pip); err != nil {
			continue
		}

		// Install package - no timeout restriction for tool management
		cmd := exec.Command(pip, "install", packageName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pip installation failed: %v", err)
		}

		return nil
	}

	return fmt.Errorf("pip not found")
}

// GetToolNames returns a list of all available tool names
func (m *Manager) GetToolNames() []string {
	if m.config == nil {
		return nil
	}

	var names []string
	for name := range m.config.Tools {
		names = append(names, name)
	}

	return names
}

// GetConfigVersion returns the configuration version
func (m *Manager) GetConfigVersion() string {
	if m.config == nil {
		return "unknown"
	}
	return m.config.Version
}

// FormatToolStatus formats tool status for display
func FormatToolStatus(status ToolStatus) string {
	// Display format: ✅ command_name (Tool Name) - status(version)

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

// installViaGitHub installs a tool from GitHub releases
func (m *Manager) installViaGitHub(toolName string, installInfo InstallInfo) error {
	fmt.Printf("📦 Installing %s from GitHub repository: %s\n", toolName, installInfo.Repo)

	// Get install directory
	installDir := m.getInstallDir()
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Get latest release info
	release, err := m.getLatestGitHubRelease(installInfo.Repo)
	if err != nil {
		fmt.Printf("❌ Failed to get GitHub release info: %v\n", err)
		fmt.Printf("💡 Manual installation steps:\n")
		m.printManualInstallInstructions(toolName, installInfo)
		return fmt.Errorf("failed to get release info: %w", err)
	}

	// Find matching asset
	assetName := m.expandPattern(installInfo.Pattern, release.TagName)
	asset := m.findMatchingAsset(release.Assets, assetName)
	if asset == nil {
		fmt.Printf("❌ No matching asset found for pattern: %s\n", assetName)
		fmt.Printf("💡 Available assets:\n")
		for _, a := range release.Assets {
			fmt.Printf("   - %s\n", a.Name)
		}
		fmt.Printf("💡 Manual installation steps:\n")
		m.printManualInstallInstructions(toolName, installInfo)
		return fmt.Errorf("no matching asset found")
	}

	fmt.Printf("📥 Downloading: %s (version %s)\n", asset.Name, release.TagName)

	// Download the asset
	tempFile, err := m.downloadFile(asset.BrowserDownloadURL)
	if err != nil {
		fmt.Printf("❌ Download failed: %v\n", err)
		fmt.Printf("💡 Manual installation steps:\n")
		m.printManualInstallInstructions(toolName, installInfo)
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tempFile)

	// Install the file
	targetName := installInfo.Target
	if targetName == "" {
		// Use tool name as target if not specified
		targetName = toolName
		if runtime.GOOS == "windows" {
			targetName += ".exe"
		}
	}

	targetPath := filepath.Join(installDir, targetName)

	if err := m.installDownloadedFile(tempFile, targetPath, asset.Name); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Cache the tool path
	m.setCachedToolPath(toolName, targetPath)
	if err := m.savePathCache(); err != nil {
		fmt.Printf("⚠️  Warning: Failed to save path cache: %v\n", err)
	}

	fmt.Printf("✅ %s installed successfully to: %s\n", toolName, targetPath)
	return nil
}

// installViaDownload installs a tool via direct download
func (m *Manager) installViaDownload(toolName string, installInfo InstallInfo) error {
	fmt.Printf("📦 Installing %s via download from: %s\n", toolName, installInfo.URL)

	// Get install directory
	installDir := m.getInstallDir()
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Download the file
	tempFile, err := m.downloadFile(installInfo.URL)
	if err != nil {
		fmt.Printf("❌ Download failed: %v\n", err)
		fmt.Printf("💡 Please download manually from: %s\n", installInfo.URL)
		fmt.Printf("   Install to: %s\n", installDir)
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tempFile)

	// Install the file
	targetName := installInfo.Target
	if targetName == "" {
		targetName = toolName
		if runtime.GOOS == "windows" {
			targetName += ".exe"
		}
	}

	targetPath := filepath.Join(installDir, targetName)
	filename := filepath.Base(installInfo.URL)

	if err := m.installDownloadedFile(tempFile, targetPath, filename); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Cache the tool path
	m.setCachedToolPath(toolName, targetPath)
	if err := m.savePathCache(); err != nil {
		fmt.Printf("⚠️  Warning: Failed to save path cache: %v\n", err)
	}

	fmt.Printf("✅ %s installed successfully to: %s\n", toolName, targetPath)
	return nil
}

// installViaInstaller handles installer downloads
func (m *Manager) installViaInstaller(installInfo InstallInfo) error {
	fmt.Printf("📦 Opening installer download page: %s\n", installInfo.URL)
	fmt.Printf("💡 Please download and run the installer manually\n")
	fmt.Printf("   After installation, the tool should be available in your PATH\n")

	// Try to open the URL in the default browser
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", installInfo.URL)
	case "linux":
		cmd = exec.Command("xdg-open", installInfo.URL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", installInfo.URL)
	default:
		fmt.Printf("   URL: %s\n", installInfo.URL)
		return nil
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("   Failed to open browser, please visit: %s\n", installInfo.URL)
	}

	return fmt.Errorf("manual installation required")
}

// getInstallDir returns the installation directory for tools
func (m *Manager) getInstallDir() string {
	if m.config != nil && m.config.Config != nil {
		if installDirConfig, exists := m.config.Config["install_dir"]; exists {
			if installDirs, ok := installDirConfig.(map[string]interface{}); ok {
				if dir, exists := installDirs[runtime.GOOS]; exists {
					if dirStr, ok := dir.(string); ok {
						// Expand environment variables
						return m.environment.GetCrossPlatformUtils().NormalizePath(os.ExpandEnv(dirStr))
					}
				}
			}
		}
	}

	// Default fallback
	homeDir, _ := m.environment.GetCrossPlatformUtils().GetHomeDir()
	return filepath.Join(homeDir, ".amo", "tools")
}

// getLatestGitHubRelease gets the latest release from a GitHub repository
func (m *Manager) getLatestGitHubRelease(repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

// expandPattern expands placeholders in asset filename patterns
func (m *Manager) expandPattern(pattern, version string) string {
	result := pattern

	// Replace version placeholder
	result = strings.ReplaceAll(result, "{version}", strings.TrimPrefix(version, "v"))

	// Replace architecture placeholder
	arch := runtime.GOARCH
	if arch == "amd64" {
		// Some projects use "x86_64" instead of "amd64"
		if strings.Contains(pattern, "x86_64") {
			arch = "x86_64"
		}
	}
	result = strings.ReplaceAll(result, "{arch}", arch)

	return result
}

// findMatchingAsset finds an asset that matches the given name pattern
func (m *Manager) findMatchingAsset(assets []GitHubReleaseAsset, pattern string) *GitHubReleaseAsset {
	// First try exact match
	for _, asset := range assets {
		if asset.Name == pattern {
			return &asset
		}
	}

	// Then try pattern matching (case-insensitive)
	pattern = strings.ToLower(pattern)
	for _, asset := range assets {
		if strings.ToLower(asset.Name) == pattern {
			return &asset
		}
	}

	// Finally try contains matching for partial patterns
	for _, asset := range assets {
		if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(pattern)) {
			return &asset
		}
	}

	return nil
}

// downloadFile downloads a file from the given URL and returns the temporary file path
func (m *Manager) downloadFile(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp("", "amo-download-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Copy response body to temp file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return tempFile.Name(), nil
}

// installDownloadedFile installs a downloaded file to the target path
func (m *Manager) installDownloadedFile(sourcePath, targetPath, originalName string) error {
	// Check if the source is a zip file
	if strings.HasSuffix(strings.ToLower(originalName), ".zip") {
		return m.extractAndInstallZip(sourcePath, targetPath, originalName)
	}

	// For non-zip files, copy directly
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Make executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(targetPath, 0755); err != nil {
			return fmt.Errorf("failed to make file executable: %w", err)
		}
	}

	return nil
}

// extractAndInstallZip extracts a zip file and installs the binary
func (m *Manager) extractAndInstallZip(zipPath, targetPath, originalName string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Find the executable file in the zip
	var executableFile *zip.File
	targetBaseName := strings.TrimSuffix(filepath.Base(targetPath), filepath.Ext(filepath.Base(targetPath)))

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		fileName := filepath.Base(file.Name)
		fileNameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		// Look for files that match the target name or are executable
		if strings.EqualFold(fileNameWithoutExt, targetBaseName) ||
			(runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(fileName), ".exe")) ||
			(runtime.GOOS != "windows" && (file.FileInfo().Mode()&0111) != 0) {
			executableFile = file
			break
		}
	}

	if executableFile == nil {
		return fmt.Errorf("no executable file found in zip archive")
	}

	// Extract the executable
	srcFile, err := executableFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open file from zip: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}

	// Make executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(targetPath, 0755); err != nil {
			return fmt.Errorf("failed to make file executable: %w", err)
		}
	}

	return nil
}

// printManualInstallInstructions prints manual installation instructions
func (m *Manager) printManualInstallInstructions(toolName string, installInfo InstallInfo) {
	installDir := m.getInstallDir()

	fmt.Printf("   1. Visit: https://github.com/%s/releases\n", installInfo.Repo)
	fmt.Printf("   2. Download the appropriate binary for your system:\n")

	switch runtime.GOOS {
	case "windows":
		fmt.Printf("      - Look for files containing 'windows' and 'amd64'\n")
		fmt.Printf("      - Example: %s\n", strings.ReplaceAll(installInfo.Pattern, "{arch}", "amd64"))
	case "darwin":
		fmt.Printf("      - Look for files containing 'darwin' and your architecture\n")
		if runtime.GOARCH == "arm64" {
			fmt.Printf("      - For Apple Silicon: %s\n", strings.ReplaceAll(installInfo.Pattern, "{arch}", "arm64"))
		} else {
			fmt.Printf("      - For Intel Mac: %s\n", strings.ReplaceAll(installInfo.Pattern, "{arch}", "amd64"))
		}
	case "linux":
		fmt.Printf("      - Look for files containing 'linux' and your architecture\n")
		fmt.Printf("      - Example: %s\n", strings.ReplaceAll(installInfo.Pattern, "{arch}", runtime.GOARCH))
	}

	fmt.Printf("   3. Create directory: %s\n", installDir)
	fmt.Printf("   4. Copy the downloaded binary to: %s\n", filepath.Join(installDir, toolName))
	if runtime.GOOS != "windows" {
		fmt.Printf("   5. Make it executable: chmod +x %s\n", filepath.Join(installDir, toolName))
	}
	fmt.Printf("   6. Add to PATH or run: amo tool cache clear (to re-detect)\n")
}
