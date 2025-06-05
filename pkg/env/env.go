package env

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// AppName defines the application name for directory creation
	AppName = "amo"
)

// Environment provides system environment information and utilities
type Environment struct {
	userConfigDir string
	crossPlatform *CrossPlatformUtils
}

// NewEnvironment creates a new Environment instance
func NewEnvironment() (*Environment, error) {
	crossPlatform := NewCrossPlatformUtils()

	userConfigDir, err := getUserConfigDir(crossPlatform)
	if err != nil {
		return nil, fmt.Errorf("failed to determine user config directory: %w", err)
	}

	// Ensure user config directory exists with appropriate permissions
	if err := crossPlatform.CreateDirWithPermissions(userConfigDir); err != nil {
		return nil, fmt.Errorf("failed to create user config directory: %w", err)
	}

	return &Environment{
		userConfigDir: userConfigDir,
		crossPlatform: crossPlatform,
	}, nil
}

// GetCurrentWorkingDir returns the current working directory
func (e *Environment) GetCurrentWorkingDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return e.crossPlatform.NormalizePath(cwd), nil
}

// GetUserConfigDir returns the user config directory path
func (e *Environment) GetUserConfigDir() string {
	return e.userConfigDir
}

// GetTempPath returns a temporary random path under the app data directory
func (e *Environment) GetTempPath() (string, error) {
	// Generate random directory name
	randomName, err := generateRandomName(16)
	if err != nil {
		return "", fmt.Errorf("failed to generate random name: %w", err)
	}

	tempPath := e.crossPlatform.JoinPath(e.userConfigDir, "temp", randomName)

	// Create the temporary directory with appropriate permissions
	if err := e.crossPlatform.CreateDirWithPermissions(tempPath); err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	return tempPath, nil
}

// GetSystemLanguage returns the current system language
func (e *Environment) GetSystemLanguage() string {
	// Try different environment variables in order of preference
	langVars := []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"}

	for _, langVar := range langVars {
		if lang := e.crossPlatform.GetEnvironmentVariable(langVar); lang != "" {
			// Extract language code (e.g., "en_US.UTF-8" -> "en_US")
			if idx := strings.Index(lang, "."); idx != -1 {
				lang = lang[:idx]
			}
			return lang
		}
	}

	// Default fallback
	return "en_US"
}

// GetOperatingSystem returns the current operating system type
func (e *Environment) GetOperatingSystem() string {
	return runtime.GOOS
}

// GetArchitecture returns the current system architecture
func (e *Environment) GetArchitecture() string {
	return runtime.GOARCH
}

// GetSystemInfo returns comprehensive system information
func (e *Environment) GetSystemInfo() (map[string]interface{}, error) {
	cwd, err := e.GetCurrentWorkingDir()
	if err != nil {
		return nil, err
	}

	tempPath, err := e.GetTempPath()
	if err != nil {
		return nil, err
	}

	// Get system directories
	homeDir, _ := e.crossPlatform.GetHomeDir()
	configDir, _ := e.crossPlatform.GetConfigDir()
	dataDir, _ := e.crossPlatform.GetDataDir()
	cacheDir, _ := e.crossPlatform.GetCacheDir()

	info := map[string]interface{}{
		"current_working_dir":  cwd,
		"user_config_dir":      e.GetUserConfigDir(),
		"temp_path":            tempPath,
		"system_language":      e.GetSystemLanguage(),
		"operating_system":     e.GetOperatingSystem(),
		"arch":                 e.GetArchitecture(),
		"go_version":           runtime.Version(),
		"home_dir":             homeDir,
		"system_config_dir":    configDir,
		"system_data_dir":      dataDir,
		"system_cache_dir":     cacheDir,
		"path_separator":       e.crossPlatform.GetPathSeparator(),
		"path_list_separator":  e.crossPlatform.GetPathListSeparator(),
		"executable_extension": e.crossPlatform.GetExecutableExtension(),
	}

	return info, nil
}

// CleanupTempPath removes a temporary path if it exists
func (e *Environment) CleanupTempPath(tempPath string) error {
	// Normalize paths for comparison
	normalizedTempPath := e.crossPlatform.NormalizePath(tempPath)
	normalizedConfigDir := e.crossPlatform.NormalizePath(e.userConfigDir)

	// Ensure the path is under our app data directory for safety
	if !strings.HasPrefix(normalizedTempPath, normalizedConfigDir) {
		return fmt.Errorf("temp path is not under app data directory: %s", tempPath)
	}

	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		return nil // Already doesn't exist
	}

	return os.RemoveAll(tempPath)
}

// getUserConfigDir determines the appropriate user config directory based on the OS
func getUserConfigDir(crossPlatform *CrossPlatformUtils) (string, error) {
	// For this app, we use a simple approach: ~/.amo for all platforms
	// This provides consistency across platforms while being simple to understand
	homeDir, err := crossPlatform.GetHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Use ~/.amo for all platforms (simple and consistent)
	userConfigDir := crossPlatform.JoinPath(homeDir, "."+strings.ToLower(AppName))

	return userConfigDir, nil
}

// Alternative: getUserConfigDirXDG uses platform-specific directories following XDG/system conventions
func getUserConfigDirXDG(crossPlatform *CrossPlatformUtils) (string, error) {
	// This function demonstrates platform-specific directory selection
	// but we keep the simpler approach above for consistency

	switch runtime.GOOS {
	case "windows":
		// Use %APPDATA%\amo
		if appData := crossPlatform.GetEnvironmentVariable("APPDATA"); appData != "" {
			return crossPlatform.JoinPath(appData, AppName), nil
		}
		// Fallback to home directory
		homeDir, err := crossPlatform.GetHomeDir()
		if err != nil {
			return "", err
		}
		return crossPlatform.JoinPath(homeDir, AppName), nil

	case "darwin":
		// Use ~/Library/Application Support/amo
		homeDir, err := crossPlatform.GetHomeDir()
		if err != nil {
			return "", err
		}
		return crossPlatform.JoinPath(homeDir, "Library", "Application Support", AppName), nil

	default:
		// Linux and other Unix-like systems: use XDG Base Directory Specification
		if configHome := crossPlatform.GetEnvironmentVariable("XDG_CONFIG_HOME"); configHome != "" {
			return crossPlatform.JoinPath(configHome, AppName), nil
		}

		homeDir, err := crossPlatform.GetHomeDir()
		if err != nil {
			return "", err
		}
		return crossPlatform.JoinPath(homeDir, ".config", AppName), nil
	}
}

// generateRandomName generates a random string for temporary directory names
func generateRandomName(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}

	return string(bytes), nil
}

// GetAllowedCLIPath returns the path to the allowed CLI commands file
func (e *Environment) GetAllowedCLIPath() string {
	return e.crossPlatform.JoinPath(e.userConfigDir, "allowed_cli.txt")
}

// EnsureAllowedCLIFile ensures the allowed CLI commands file exists
// If it doesn't exist, creates a file with default tool commands
func (e *Environment) EnsureAllowedCLIFile() error {
	filePath := e.GetAllowedCLIPath()

	// Check if file already exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create file with default tool commands
		content := `# Allowed CLI commands for workflows - one per line
# 
# This file controls which CLI commands can be executed within JavaScript workflows.
# It is a security whitelist to prevent unauthorized system access from workflow scripts.
# 
# IMPORTANT: This is NOT for tool installation commands.
# Only add commands that workflows need to execute directly.
#
# Basic system commands (safe for workflows)
echo
#
# Default supported external tools (for workflow processing)
# Media processing
ffmpeg
#
# Image processing
magick
convert
#
# Document conversion and processing
ebook-convert
gs
pandoc
#
# OCR and text extraction
surya_ocr
doc-to-text
#
# LLM and AI tools
llm-caller
#
# Add your custom workflow commands below:
# (Only add commands that workflows need to execute)
#
`
		err := e.crossPlatform.CreateFileWithPermissions(filePath, []byte(content), false)
		if err != nil {
			return fmt.Errorf("failed to create allowed CLI file: %w", err)
		}
	}

	return nil
}

// LoadAllowedCLICommands loads the list of allowed CLI commands from the file
func (e *Environment) LoadAllowedCLICommands() ([]string, error) {
	if err := e.EnsureAllowedCLIFile(); err != nil {
		return nil, err
	}

	filePath := e.GetAllowedCLIPath()
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read allowed CLI file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var commands []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			commands = append(commands, line)
		}
	}

	return commands, nil
}

// IsCommandAllowed checks if a command is in the allowed CLI commands list
func (e *Environment) IsCommandAllowed(command string) (bool, error) {
	allowedCommands, err := e.LoadAllowedCLICommands()
	if err != nil {
		return false, err
	}

	// If the file is empty (no allowed commands), deny all commands
	if len(allowedCommands) == 0 {
		return false, nil
	}

	for _, allowedCmd := range allowedCommands {
		if allowedCmd == command {
			return true, nil
		}
	}

	return false, nil
}

// GetCrossPlatformUtils returns the cross-platform utilities instance
func (e *Environment) GetCrossPlatformUtils() *CrossPlatformUtils {
	return e.crossPlatform
}

// IsValidPath checks if a path is valid for the current operating system
func (e *Environment) IsValidPath(path string) bool {
	// Split path into components and check each one
	pathComponents := strings.Split(e.crossPlatform.NormalizePath(path), e.crossPlatform.GetPathSeparator())

	for _, component := range pathComponents {
		if component != "" && !e.crossPlatform.IsValidFilename(component) {
			return false
		}
	}

	return true
}

// NormalizePath provides access to cross-platform path normalization
func (e *Environment) NormalizePath(path string) string {
	return e.crossPlatform.NormalizePath(path)
}

// JoinPath provides access to cross-platform path joining
func (e *Environment) JoinPath(elements ...string) string {
	return e.crossPlatform.JoinPath(elements...)
}

// AddAllowedCommand adds a command to the allowed CLI commands list
func (e *Environment) AddAllowedCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	// Load current commands
	commands, err := e.LoadAllowedCLICommands()
	if err != nil {
		return fmt.Errorf("failed to load current commands: %w", err)
	}

	// Check if command already exists
	for _, cmd := range commands {
		if cmd == command {
			return fmt.Errorf("command '%s' is already in the whitelist", command)
		}
	}

	// Add new command
	commands = append(commands, command)

	// Save updated list
	return e.saveAllowedCLICommands(commands)
}

// RemoveAllowedCommand removes a command from the allowed CLI commands list
func (e *Environment) RemoveAllowedCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command name cannot be empty")
	}

	// Load current commands
	commands, err := e.LoadAllowedCLICommands()
	if err != nil {
		return fmt.Errorf("failed to load current commands: %w", err)
	}

	// Find and remove command
	var updatedCommands []string
	found := false
	for _, cmd := range commands {
		if cmd != command {
			updatedCommands = append(updatedCommands, cmd)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("command '%s' is not in the whitelist", command)
	}

	// Save updated list
	return e.saveAllowedCLICommands(updatedCommands)
}

// saveAllowedCLICommands saves the allowed CLI commands list to file
func (e *Environment) saveAllowedCLICommands(commands []string) error {
	filePath := e.GetAllowedCLIPath()

	// Create content with header
	content := `# Allowed CLI commands for workflows - one per line
# 
# This file controls which CLI commands can be executed within JavaScript workflows.
# It is a security whitelist to prevent unauthorized system access from workflow scripts.
# 
# IMPORTANT: This is NOT for tool installation commands.
# Only add commands that workflows need to execute directly.
#
# Basic system commands (safe for workflows)
# echo
#
# External tools for workflow processing:
#
`

	// Add commands
	for _, cmd := range commands {
		if cmd != "" {
			content += cmd + "\n"
		}
	}

	content += "#\n# Add your custom workflow commands above\n"

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := e.crossPlatform.CreateDirWithPermissions(dir); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write file
	return e.crossPlatform.CreateFileWithPermissions(filePath, []byte(content), false)
}
