package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"amo/pkg/env"
)

// Manager handles tool management operations
type Manager struct {
	config         *ToolConfig
	environment    *env.Environment
	pathCache      *ToolPathCache
	workflowEngine WorkflowEngine
}

// InstallOptions represents optional parameters to override installation behavior
type InstallOptions struct {
	// URL overrides the download or installer URL. When provided,
	// the manager will install directly from this URL regardless of
	// the method defined in assets/tools.json.
	URL string
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

// SetWorkflowEngine sets the workflow engine for the manager
func (m *Manager) SetWorkflowEngine(engine WorkflowEngine) {
	m.workflowEngine = engine
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

	// Try system PATH for primary command first
	if path, err := exec.LookPath(tool.Check.Command); err == nil {
		m.setCachedToolPath(tool.Check.Command, path)
		return path
	}
	// Windows-specific fallback via 'where'
	if runtime.GOOS == "windows" {
		if wherePath, err := exec.LookPath("where"); err == nil {
			out, err := exec.Command(wherePath, tool.Check.Command).CombinedOutput()
			if err == nil {
				lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
				for _, l := range lines {
					p := strings.TrimSpace(l)
					if p == "" {
						continue
					}
					if _, err := os.Stat(p); err == nil {
						m.setCachedToolPath(tool.Check.Command, p)
						return p
					}
				}
			}
		}
	}

	// Check for fallback commands
	if len(tool.Check.FallbackCommands) > 0 {
		for _, fallbackCmd := range tool.Check.FallbackCommands {
			if runtime.GOOS == "windows" && strings.EqualFold(fallbackCmd, "convert") {
				continue
			}
			fallbackPath := filepath.Join(installDir, fallbackCmd)
			if runtime.GOOS == "windows" && !strings.HasSuffix(fallbackPath, ".exe") {
				fallbackPath += ".exe"
			}
			if _, err := os.Stat(fallbackPath); err == nil {
				m.setCachedToolPath(tool.Check.Command, fallbackPath)
				return fallbackPath
			}
			if path, err := exec.LookPath(fallbackCmd); err == nil {
				m.setCachedToolPath(tool.Check.Command, path)
				return path
			}
			if runtime.GOOS == "windows" {
				if wherePath, err := exec.LookPath("where"); err == nil {
					out, err := exec.Command(wherePath, fallbackCmd).CombinedOutput()
					if err == nil {
						lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
						for _, l := range lines {
							p := strings.TrimSpace(l)
							if p == "" {
								continue
							}
							if _, err := os.Stat(p); err == nil {
								m.setCachedToolPath(tool.Check.Command, p)
								return p
							}
						}
					}
				}
			}
		}
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
		// If primary command failed, try fallback commands
		if len(tool.Check.FallbackCommands) > 0 {
			for _, fallbackCmd := range tool.Check.FallbackCommands {
				if runtime.GOOS == "windows" && strings.EqualFold(fallbackCmd, "convert") {
					continue
				}
				fallbackCommand := m.findToolExecutable(Tool{Check: CheckConfig{Command: fallbackCmd, FallbackCommands: []string{}}})
				fallbackArgs := tool.Check.Args
				if len(fallbackArgs) == 0 {
					fallbackArgs = []string{"--version"}
				}

				fallbackCmd := exec.Command(fallbackCommand, fallbackArgs...)
				if fallbackOutput, fallbackErr := fallbackCmd.CombinedOutput(); fallbackErr == nil {
					// Fallback command succeeded, use its output
					command = fallbackCommand
					output = fallbackOutput
					err = nil
					break
				}
			}
		}

		// If still no success, return error
		if err != nil {
			status.Error = fmt.Sprintf("command failed: %v", err)
			// Clear cached path if command failed
			m.clearCachedToolPath(tool.Check.Command)
			return status
		}
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

// InstallTool installs a specific tool
func (m *Manager) InstallTool(toolName string, forceReinstall bool) error {
	return m.InstallToolWithOptions(toolName, forceReinstall, nil)
}

// InstallToolWithOptions installs a specific tool with optional overrides
func (m *Manager) InstallToolWithOptions(toolName string, forceReinstall bool, opts *InstallOptions) error {
	if m.config == nil {
		return fmt.Errorf("tool configuration not loaded")
	}

	tool, exists := m.config.Tools[toolName]
	if !exists {
		return fmt.Errorf("unknown tool: %s", toolName)
	}

	// Check if already installed and not forcing reinstall
	if !forceReinstall {
		status := m.checkToolStatus(toolName, tool)
		if status.Installed {
			fmt.Printf("✅ %s is already installed (version: %s)\n", tool.Name, status.Version)
			// Even if already installed, try to ensure it's in PATH
			if err := m.ensureToolsInPath(); err != nil {
				fmt.Printf("⚠️  Warning: Failed to ensure tools directory in PATH: %v\n", err)
			}
			return nil
		}
	}

	fmt.Printf("📦 Installing %s...\n", tool.Name)

	// Get platform-specific install info
	osName := m.environment.GetOperatingSystem()
	installInfo, exists := tool.Install[osName]
	if !exists {
		return fmt.Errorf("installation not supported for platform: %s", osName)
	}

	// If a URL override is provided, install directly from that URL
	if opts != nil && strings.TrimSpace(opts.URL) != "" {
		ovr := InstallInfo{URL: strings.TrimSpace(opts.URL), Target: installInfo.Target}
		if err := m.installViaDownload(toolName, ovr); err != nil {
			return fmt.Errorf("failed to install %s from provided URL: %w", toolName, err)
		}
		// Clear cache and verify like normal flow below
		m.clearCachedToolPath(toolName)
		status := m.checkToolStatus(toolName, tool)
		if status.Installed {
			fmt.Printf("✅ Successfully installed %s (version: %s)\n", tool.Name, status.Version)
			if err := m.ensureToolsInPath(); err != nil {
				fmt.Printf("⚠️  Warning: Failed to configure PATH: %v\n", err)
			}
			return nil
		}
		return fmt.Errorf("installation verification failed for %s: %s", toolName, status.Error)
	}

	// Install based on method
	var err error
	switch installInfo.Method {
	case "homebrew":
		err = m.installViaHomebrew(installInfo.Package)
	case "package":
		err = m.installViaPackageManager(installInfo.Packages)
	case "pip":
		err = m.installViaPip(installInfo.Package)
	case "github":
		err = m.installViaGitHub(toolName, installInfo)
	case "download":
		err = m.installViaDownload(toolName, installInfo)
	case "installer":
		err = m.installViaInstaller(installInfo)
	case "workflow":
		err = m.installViaWorkflow(toolName, installInfo)
	default:
		m.printManualInstallInstructions(toolName, installInfo)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to install %s: %w", toolName, err)
	}

	// Clear path cache for this tool to force re-detection
	m.clearCachedToolPath(toolName)

	// Verify installation
	status := m.checkToolStatus(toolName, tool)
	if status.Installed {
		fmt.Printf("✅ Successfully installed %s (version: %s)\n", tool.Name, status.Version)

		// Try to ensure tools directory is in PATH after successful installation
		if err := m.ensureToolsInPath(); err != nil {
			fmt.Printf("⚠️  Warning: Failed to configure PATH: %v\n", err)
		}
	} else {
		return fmt.Errorf("installation verification failed for %s: %s", toolName, status.Error)
	}

	return nil
}

// getInstallDir returns the installation directory for tools
func (m *Manager) getInstallDir() string {
	config := m.config.Config
	installDirConfig, ok := config["install_dir"].(map[string]interface{})
	if !ok {
		// Fallback to default
		homeDir, err := m.environment.GetCrossPlatformUtils().GetHomeDir()
		if err != nil {
			return filepath.Join(".", ".amo", "tools")
		}
		return filepath.Join(homeDir, ".amo", "tools")
	}

	osName := m.environment.GetOperatingSystem()
	installDir, ok := installDirConfig[osName].(string)
	if !ok {
		// Fallback to default
		homeDir, err := m.environment.GetCrossPlatformUtils().GetHomeDir()
		if err != nil {
			return filepath.Join(".", ".amo", "tools")
		}
		return filepath.Join(homeDir, ".amo", "tools")
	}

	// Expand environment variables
	crossPlatform := m.environment.GetCrossPlatformUtils()
	installDir = os.ExpandEnv(installDir)

	// Handle platform-specific path expansion
	if strings.Contains(installDir, "$HOME") {
		homeDir, err := crossPlatform.GetHomeDir()
		if err == nil {
			installDir = strings.ReplaceAll(installDir, "$HOME", homeDir)
		}
	}

	return crossPlatform.NormalizePath(installDir)
}

// GetInstallDir returns the installation directory for tools (exported version)
func (m *Manager) GetInstallDir() string {
	return m.getInstallDir()
}

// ensureToolsInPath ensures the tools directory is in the system PATH
func (m *Manager) ensureToolsInPath() error {
	toolsDir := m.getInstallDir()

	// Only attempt PATH configuration if the tools directory exists and contains files
	if _, err := os.Stat(toolsDir); os.IsNotExist(err) {
		return nil // Tools directory doesn't exist yet, skip PATH configuration
	}

	// Check if directory has any executable files
	files, err := os.ReadDir(toolsDir)
	if err != nil {
		return nil // Can't read directory, skip PATH configuration
	}

	hasExecutables := false
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if runtime.GOOS == "windows" {
			if strings.HasSuffix(strings.ToLower(file.Name()), ".exe") {
				hasExecutables = true
				break
			}
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.Mode().Perm()&0111 != 0 {
			hasExecutables = true
			break
		}
	}

	if !hasExecutables {
		return nil // No executable files found, skip PATH configuration
	}

	return m.environment.EnsureToolsDirInPath(toolsDir)
}

// EnsureToolsInPath ensures the tools directory is in the system PATH (exported version)
func (m *Manager) EnsureToolsInPath() error {
	return m.ensureToolsInPath()
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

// CheckToolsWithCallback checks all tools status and calls the callback function
// after each tool check for immediate feedback
func (m *Manager) CheckToolsWithCallback(callback func(ToolStatus)) error {
	if m.config == nil {
		return fmt.Errorf("tool configuration not loaded")
	}

	for toolName, tool := range m.config.Tools {
		status := m.checkToolStatus(toolName, tool)
		callback(status)
	}

	// Save path cache after checking all tools
	if err := m.savePathCache(); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: failed to save tool path cache: %v\n", err)
	}

	return nil
}
