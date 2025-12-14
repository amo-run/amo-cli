package config

import (
	"fmt"
	"os"
	"path/filepath"

	"amo/pkg/env"

	"github.com/spf13/viper"
)

const (
	ConfigFileName = "config.yaml"
)

const (
	KeyWorkflowDir                        = "workflows"
	KeyNetworkDialTimeoutSeconds          = "network_dial_timeout_seconds"
	KeyNetworkTLSHandshakeTimeoutSeconds  = "network_tls_handshake_timeout_seconds"
	KeyNetworkResponseHeaderTimeoutSecond = "network_response_header_timeout_seconds"
	KeyNetworkIdleTimeoutSeconds          = "network_idle_timeout_seconds"
)

var DefaultConfig = map[string]interface{}{
	KeyWorkflowDir:                        "",
	KeyNetworkDialTimeoutSeconds:          15,
	KeyNetworkTLSHandshakeTimeoutSeconds:  15,
	KeyNetworkResponseHeaderTimeoutSecond: 60,
	KeyNetworkIdleTimeoutSeconds:          300,
}

type Manager struct {
	viper         *viper.Viper
	environment   *env.Environment
	configDir     string
	configFile    string
	isInitialized bool
}

func NewManager() (*Manager, error) {
	environment, err := env.NewEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize environment: %w", err)
	}

	configDir := environment.GetUserConfigDir()
	configFile := filepath.Join(configDir, ConfigFileName)

	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	manager := &Manager{
		viper:       v,
		environment: environment,
		configDir:   configDir,
		configFile:  configFile,
	}

	return manager, nil
}

func (m *Manager) Initialize() error {
	if m.isInitialized {
		return nil
	}

	// Ensure config directory exists
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Set default values
	for key, value := range DefaultConfig {
		m.viper.SetDefault(key, value)
	}

	// Check if config file exists
	if _, err := os.Stat(m.configFile); os.IsNotExist(err) {
		// When config doesn't exist, we need to create an empty one first
		if err := os.WriteFile(m.configFile, []byte{}, 0644); err != nil {
			return fmt.Errorf("failed to create empty config file: %w", err)
		}
	}

	// Always set the config file path
	m.viper.SetConfigFile(m.configFile)
	m.viper.SetConfigType("yaml")

	// Load config file
	if err := m.viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Save to ensure defaults are written
	if err := m.viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	m.isInitialized = true
	return nil
}

func (m *Manager) GetConfigFile() string {
	return m.configFile
}

func (m *Manager) Set(key string, value interface{}) error {
	if err := m.Initialize(); err != nil {
		return err
	}

	m.viper.Set(key, value)
	return m.viper.WriteConfig()
}

func (m *Manager) Get(key string) interface{} {
	if err := m.Initialize(); err != nil {
		return nil
	}

	return m.viper.Get(key)
}

func (m *Manager) GetString(key string) string {
	if err := m.Initialize(); err != nil {
		return ""
	}

	return m.viper.GetString(key)
}

func (m *Manager) GetBool(key string) bool {
	if err := m.Initialize(); err != nil {
		return false
	}

	return m.viper.GetBool(key)
}

func (m *Manager) GetInt(key string) int {
	if err := m.Initialize(); err != nil {
		return 0
	}

	return m.viper.GetInt(key)
}

func (m *Manager) Unset(key string) error {
	if err := m.Initialize(); err != nil {
		return err
	}

	// Get default value
	defaultValue, exists := DefaultConfig[key]
	if !exists {
		return fmt.Errorf("no default value for key: %s", key)
	}

	// Set to default value
	m.viper.Set(key, defaultValue)
	return m.viper.WriteConfig()
}

// GetAll returns all configuration values
func (m *Manager) GetAll() map[string]interface{} {
	if err := m.Initialize(); err != nil {
		return map[string]interface{}{}
	}

	return m.viper.AllSettings()
}

// IsValidKey checks if a key is valid
func (m *Manager) IsValidKey(key string) bool {
	_, exists := DefaultConfig[key]
	return exists
}

// GetValidKeys returns a list of all valid configuration keys
func (m *Manager) GetValidKeys() []string {
	keys := make([]string, 0, len(DefaultConfig))
	for key := range DefaultConfig {
		keys = append(keys, key)
	}
	return keys
}

// GetWorkflowsDir returns the configured workflows directory
// If not set, returns the default workflows directory
func (m *Manager) GetWorkflowsDir() string {
	if err := m.Initialize(); err != nil {
		// Fallback to default
		downloader, err := NewWorkflowDownloader()
		if err != nil {
			return ""
		}
		return downloader.GetWorkflowsDir()
	}

	configuredDir := m.GetString(KeyWorkflowDir)
	if configuredDir == "" {
		// Use default location
		downloader, err := NewWorkflowDownloader()
		if err != nil {
			return ""
		}
		return downloader.GetWorkflowsDir()
	}

	// Normalize path
	return m.environment.GetCrossPlatformUtils().NormalizePath(configuredDir)
}

// NewWorkflowDownloader creates a new workflow downloader
// This is a temporary function to avoid circular imports
func NewWorkflowDownloader() (*struct{ GetWorkflowsDir func() string }, error) {
	environment, err := env.NewEnvironment()
	if err != nil {
		return nil, err
	}

	return &struct{ GetWorkflowsDir func() string }{
		GetWorkflowsDir: func() string {
			configDir := environment.GetUserConfigDir()
			return filepath.Join(configDir, "workflows")
		},
	}, nil
}
