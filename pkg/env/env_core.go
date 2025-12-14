package env

import (
	"crypto/rand"
	"fmt"
	"os"
	"runtime"
	"strings"
)

const AppName = "amo"

type Environment struct {
	userConfigDir string
	crossPlatform *CrossPlatformUtils
}

func NewEnvironment() (*Environment, error) {
	crossPlatform := NewCrossPlatformUtils()

	userConfigDir, err := getUserConfigDir(crossPlatform)
	if err != nil {
		return nil, fmt.Errorf("failed to determine user config directory: %w", err)
	}

	if err := crossPlatform.CreateDirWithPermissions(userConfigDir); err != nil {
		return nil, fmt.Errorf("failed to create user config directory: %w", err)
	}

	return &Environment{
		userConfigDir: userConfigDir,
		crossPlatform: crossPlatform,
	}, nil
}

func (e *Environment) GetCurrentWorkingDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return e.crossPlatform.NormalizePath(cwd), nil
}

func (e *Environment) GetUserConfigDir() string {
	return e.userConfigDir
}

func (e *Environment) GetTempPath() (string, error) {
	randomName, err := generateRandomName(16)
	if err != nil {
		return "", fmt.Errorf("failed to generate random name: %w", err)
	}

	tempPath := e.crossPlatform.JoinPath(e.userConfigDir, "temp", randomName)

	if err := e.crossPlatform.CreateDirWithPermissions(tempPath); err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	return tempPath, nil
}

func (e *Environment) GetSystemLanguage() string {
	langVars := []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"}

	for _, langVar := range langVars {
		if lang := e.crossPlatform.GetEnvironmentVariable(langVar); lang != "" {
			if idx := strings.Index(lang, "."); idx != -1 {
				lang = lang[:idx]
			}
			return lang
		}
	}

	return "en_US"
}

func (e *Environment) GetOperatingSystem() string {
	return runtime.GOOS
}

func (e *Environment) GetArchitecture() string {
	return runtime.GOARCH
}

func (e *Environment) DetectRegion() string {
	if value := strings.TrimSpace(e.crossPlatform.GetEnvironmentVariable("AMO_REGION")); value != "" {
		return strings.ToLower(value)
	}

	detector := NewRegionDetector()
	return detector.DetectRegion()
}

func (e *Environment) GetSystemInfo() (map[string]interface{}, error) {
	cwd, err := e.GetCurrentWorkingDir()
	if err != nil {
		return nil, err
	}

	tempPath, err := e.GetTempPath()
	if err != nil {
		return nil, err
	}

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

func (e *Environment) CleanupTempPath(tempPath string) error {
	normalizedTempPath := e.crossPlatform.NormalizePath(tempPath)
	normalizedConfigDir := e.crossPlatform.NormalizePath(e.userConfigDir)

	if !strings.HasPrefix(normalizedTempPath, normalizedConfigDir) {
		return fmt.Errorf("temp path is not under app data directory: %s", tempPath)
	}

	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(tempPath)
}

func getUserConfigDir(crossPlatform *CrossPlatformUtils) (string, error) {
	homeDir, err := crossPlatform.GetHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	userConfigDir := crossPlatform.JoinPath(homeDir, "."+strings.ToLower(AppName))

	return userConfigDir, nil
}

func getUserConfigDirXDG(crossPlatform *CrossPlatformUtils) (string, error) {
	switch runtime.GOOS {
	case "windows":
		if appData := crossPlatform.GetEnvironmentVariable("APPDATA"); appData != "" {
			return crossPlatform.JoinPath(appData, AppName), nil
		}
		homeDir, err := crossPlatform.GetHomeDir()
		if err != nil {
			return "", err
		}
		return crossPlatform.JoinPath(homeDir, AppName), nil

	case "darwin":
		homeDir, err := crossPlatform.GetHomeDir()
		if err != nil {
			return "", err
		}
		return crossPlatform.JoinPath(homeDir, "Library", "Application Support", AppName), nil

	default:
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

func (e *Environment) GetCrossPlatformUtils() *CrossPlatformUtils {
	return e.crossPlatform
}

func (e *Environment) IsValidPath(path string) bool {
	pathComponents := strings.Split(e.crossPlatform.NormalizePath(path), e.crossPlatform.GetPathSeparator())

	for _, component := range pathComponents {
		if component != "" && !e.crossPlatform.IsValidFilename(component) {
			return false
		}
	}

	return true
}

func (e *Environment) NormalizePath(path string) string {
	return e.crossPlatform.NormalizePath(path)
}

func (e *Environment) JoinPath(elements ...string) string {
	return e.crossPlatform.JoinPath(elements...)
}
