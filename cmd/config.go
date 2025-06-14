package cmd

import (
	"fmt"
	"strings"

	"amo/pkg/config"

	"github.com/spf13/cobra"
)

// NewConfigCmd creates the config subcommand
func NewConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config [<key> [<value>]]",
		Short: "Manage configuration settings",
		Long: `Manage amo configuration settings.

Configuration is stored in ~/.amo/config.yaml.

Usage:
  amo config <key>           Get a config value
  amo config <key> <value>   Set a config value
  amo config ls              List all config values
  amo config rm <key>        Remove a config key (restore default)

Examples:
  amo config workflows                  # Get workflows directory
  amo config workflows ~/my-workflows   # Set workflows directory
  amo config ls                         # List all settings
  amo config rm workflows               # Reset to default

Supported configuration keys:
  workflows    Directory path for custom workflows`,
		Args: cobra.MaximumNArgs(2),
		RunE: runConfigCommand,
	}

	// Add subcommands
	configCmd.AddCommand(newConfigLsCmd())
	configCmd.AddCommand(newConfigRmCmd())

	return configCmd
}

// newConfigLsCmd creates the config ls subcommand (renamed from list)
func newConfigLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all configuration values",
		Long: `List all configuration values.

Example:
  amo config ls`,
		Args: cobra.NoArgs,
		RunE: runConfigLsCmd,
	}
}

// newConfigRmCmd creates the config rm subcommand (renamed from unset)
func newConfigRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <key>",
		Short: "Remove a configuration value (restore default)",
		Long: `Remove a configuration value, restoring it to the default value.

Example:
  amo config rm workflows`,
		Args: cobra.ExactArgs(1),
		RunE: runConfigRmCmd,
	}
}

// runConfigCommand handles the unified config command
// - config <key> to get a value
// - config <key> <value> to set a value
func runConfigCommand(cmd *cobra.Command, args []string) error {
	// Create config manager
	manager, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize config manager: %w", err)
	}

	// No args - show help
	if len(args) == 0 {
		return cmd.Help()
	}

	key := args[0]

	// Check if key is valid
	if !manager.IsValidKey(key) {
		return fmt.Errorf("invalid configuration key: %s (valid keys: %s)", key, strings.Join(manager.GetValidKeys(), ", "))
	}

	// Case 1: config <key> - get value
	if len(args) == 1 {
		value := manager.Get(key)
		if value == nil || value == "" {
			fmt.Printf("%s = <not set>\n", key)
			return nil
		}
		fmt.Printf("%s = %v\n", key, value)
		return nil
	}

	// Case 2: config <key> <value> - set value
	value := args[1]
	if err := manager.Set(key, value); err != nil {
		return fmt.Errorf("failed to set configuration: %w", err)
	}

	fmt.Printf("✅ Configuration set: %s = %s\n", key, value)
	return nil
}

// runConfigLsCmd handles the config ls command (renamed from list)
func runConfigLsCmd(cmd *cobra.Command, args []string) error {
	manager, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize config manager: %w", err)
	}

	fmt.Printf("📋 Configuration values (stored in %s):\n\n", manager.GetConfigFile())

	settings := manager.GetAll()

	// Get all valid keys to ensure ordered display
	validKeys := manager.GetValidKeys()

	if len(validKeys) == 0 {
		fmt.Println("No configuration items available")
		return nil
	}

	// Iterate through all valid keys
	for _, key := range validKeys {
		value, exists := settings[key]

		// Check if configuration is set
		if exists && value != nil && value != "" {
			fmt.Printf("%s = %v\n", key, value)
		} else {
			fmt.Printf("%s = <not set>\n", key)
		}
	}

	return nil
}

// runConfigRmCmd handles the config rm command (renamed from unset)
func runConfigRmCmd(cmd *cobra.Command, args []string) error {
	manager, err := config.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize config manager: %w", err)
	}

	key := args[0]

	if !manager.IsValidKey(key) {
		return fmt.Errorf("invalid configuration key: %s (valid keys: %s)", key, strings.Join(manager.GetValidKeys(), ", "))
	}

	if err := manager.Unset(key); err != nil {
		return fmt.Errorf("failed to remove configuration: %w", err)
	}

	fmt.Printf("✅ Configuration reset: %s restored to default value\n", key)
	return nil
}
