package env

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CrossPlatformUtils provides utilities for cross-platform compatibility
type CrossPlatformUtils struct{}

// NewCrossPlatformUtils creates a new CrossPlatformUtils instance
func NewCrossPlatformUtils() *CrossPlatformUtils {
	return &CrossPlatformUtils{}
}

// GetExecutableExtension returns the appropriate executable extension for the current OS
func (cpu *CrossPlatformUtils) GetExecutableExtension() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// AddExecutableExtensionIfNeeded adds .exe extension if running on Windows and not already present
func (cpu *CrossPlatformUtils) AddExecutableExtensionIfNeeded(filename string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(filename), ".exe") {
		return filename + ".exe"
	}
	return filename
}

// GetDefaultFilePermissions returns appropriate file permissions for the current OS
func (cpu *CrossPlatformUtils) GetDefaultFilePermissions() os.FileMode {
	// 0644 is suitable for all platforms
	// On Windows, this will be ignored and default permissions will be used
	return 0644
}

// GetDefaultDirPermissions returns appropriate directory permissions for the current OS
func (cpu *CrossPlatformUtils) GetDefaultDirPermissions() os.FileMode {
	// 0755 is suitable for all platforms
	// On Windows, this will be ignored and default permissions will be used
	return 0755
}

// GetExecutableFilePermissions returns appropriate permissions for executable files
func (cpu *CrossPlatformUtils) GetExecutableFilePermissions() os.FileMode {
	if runtime.GOOS == "windows" {
		// On Windows, return standard file permissions
		return 0644
	}
	// On Unix-like systems, add execute permissions
	return 0755
}

// NormalizePath normalizes a path for cross-platform consistency
// Converts backslashes to forward slashes and cleans the path
func (cpu *CrossPlatformUtils) NormalizePath(path string) string {
	// Convert to forward slashes for consistency
	normalized := filepath.ToSlash(path)
	// Clean the path to remove redundant separators
	normalized = filepath.Clean(normalized)
	// Convert back to platform-specific separators
	return filepath.FromSlash(normalized)
}

// JoinPath joins path elements in a cross-platform compatible way
func (cpu *CrossPlatformUtils) JoinPath(elements ...string) string {
	return filepath.Join(elements...)
}

// IsAbsolutePath checks if a path is absolute in a cross-platform way
func (cpu *CrossPlatformUtils) IsAbsolutePath(path string) bool {
	return filepath.IsAbs(path)
}

// GetPathSeparator returns the appropriate path separator for the current OS
func (cpu *CrossPlatformUtils) GetPathSeparator() string {
	return string(filepath.Separator)
}

// GetPathListSeparator returns the path list separator for the current OS
func (cpu *CrossPlatformUtils) GetPathListSeparator() string {
	return string(filepath.ListSeparator)
}

// GetHomeDir returns the user's home directory in a cross-platform way
func (cpu *CrossPlatformUtils) GetHomeDir() (string, error) {
	return os.UserHomeDir()
}

// GetTempDir returns the system's temporary directory
func (cpu *CrossPlatformUtils) GetTempDir() string {
	return os.TempDir()
}

// GetConfigDir returns the appropriate configuration directory for the current OS
func (cpu *CrossPlatformUtils) GetConfigDir() (string, error) {
	homeDir, err := cpu.GetHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "windows":
		// On Windows, use AppData\Roaming if available, otherwise fallback to home
		if appData := os.Getenv("APPDATA"); appData != "" {
			return appData, nil
		}
		return homeDir, nil
	case "darwin":
		// On macOS, use ~/Library/Application Support
		return filepath.Join(homeDir, "Library", "Application Support"), nil
	default:
		// On Linux and other Unix-like systems, use ~/.config if XDG_CONFIG_HOME is not set
		if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
			return configHome, nil
		}
		return filepath.Join(homeDir, ".config"), nil
	}
}

// GetDataDir returns the appropriate data directory for the current OS
func (cpu *CrossPlatformUtils) GetDataDir() (string, error) {
	homeDir, err := cpu.GetHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "windows":
		// On Windows, use AppData\Local if available, otherwise fallback to home
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return localAppData, nil
		}
		if appData := os.Getenv("APPDATA"); appData != "" {
			return appData, nil
		}
		return homeDir, nil
	case "darwin":
		// On macOS, use ~/Library/Application Support
		return filepath.Join(homeDir, "Library", "Application Support"), nil
	default:
		// On Linux and other Unix-like systems, use ~/.local/share if XDG_DATA_HOME is not set
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			return dataHome, nil
		}
		return filepath.Join(homeDir, ".local", "share"), nil
	}
}

// GetCacheDir returns the appropriate cache directory for the current OS
func (cpu *CrossPlatformUtils) GetCacheDir() (string, error) {
	homeDir, err := cpu.GetHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "windows":
		// On Windows, use AppData\Local\Temp if available, otherwise fallback to temp
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "Temp"), nil
		}
		return cpu.GetTempDir(), nil
	case "darwin":
		// On macOS, use ~/Library/Caches
		return filepath.Join(homeDir, "Library", "Caches"), nil
	default:
		// On Linux and other Unix-like systems, use ~/.cache if XDG_CACHE_HOME is not set
		if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
			return cacheHome, nil
		}
		return filepath.Join(homeDir, ".cache"), nil
	}
}

// IsValidFilename checks if a filename is valid for the current OS
func (cpu *CrossPlatformUtils) IsValidFilename(filename string) bool {
	if filename == "" || filename == "." || filename == ".." {
		return false
	}

	// Check for invalid characters based on OS
	switch runtime.GOOS {
	case "windows":
		// Windows has more restrictions
		invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
		for _, char := range invalidChars {
			if strings.Contains(filename, char) {
				return false
			}
		}
		// Check for control characters (0-31)
		for _, r := range filename {
			if r >= 0 && r <= 31 {
				return false
			}
		}
		// Check for reserved names
		reservedNames := []string{
			"CON", "PRN", "AUX", "NUL",
			"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
			"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
		}
		upperFilename := strings.ToUpper(filename)
		for _, reserved := range reservedNames {
			if upperFilename == reserved || strings.HasPrefix(upperFilename, reserved+".") {
				return false
			}
		}
	default:
		// Unix-like systems: only check for null byte and path separator
		if strings.Contains(filename, "\x00") || strings.Contains(filename, "/") {
			return false
		}
	}

	return true
}

// CreateFileWithPermissions creates a file with appropriate permissions for the current OS
func (cpu *CrossPlatformUtils) CreateFileWithPermissions(filename string, data []byte, executable bool) error {
	var perm os.FileMode
	if executable {
		perm = cpu.GetExecutableFilePermissions()
	} else {
		perm = cpu.GetDefaultFilePermissions()
	}

	return os.WriteFile(filename, data, perm)
}

// CreateDirWithPermissions creates a directory with appropriate permissions for the current OS
func (cpu *CrossPlatformUtils) CreateDirWithPermissions(dirname string) error {
	return os.MkdirAll(dirname, cpu.GetDefaultDirPermissions())
}

// GetEnvironmentVariables returns environment variables with cross-platform handling
func (cpu *CrossPlatformUtils) GetEnvironmentVariables() map[string]string {
	envMap := make(map[string]string)

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]

			// On Windows, environment variable names are case-insensitive
			// Normalize to uppercase for consistency
			if runtime.GOOS == "windows" {
				key = strings.ToUpper(key)
			}

			envMap[key] = value
		}
	}

	return envMap
}

// GetEnvironmentVariable gets an environment variable with cross-platform handling
func (cpu *CrossPlatformUtils) GetEnvironmentVariable(key string) string {
	// On Windows, try both the original case and uppercase
	if runtime.GOOS == "windows" {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return os.Getenv(strings.ToUpper(key))
	}

	return os.Getenv(key)
}

// SetEnvironmentVariable sets an environment variable
func (cpu *CrossPlatformUtils) SetEnvironmentVariable(key, value string) error {
	return os.Setenv(key, value)
}
