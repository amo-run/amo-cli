package cmd

import (
	"fmt"
	"strings"

	"amo/pkg/config"

	"github.com/spf13/cobra"
)

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
  workflows                     Directory path for custom workflows
  security_cli_whitelist_enabled  Enable workflow CLI whitelist (true/false)`,
		Args: cobra.MaximumNArgs(2),
		RunE: runConfigCommand,
	}

	configCmd.AddCommand(newConfigLsCmd())
	configCmd.AddCommand(newConfigRmCmd())

	return configCmd
}

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

func runConfigCommand(cmd *cobra.Command, args []string) error {
	manager, err := config.NewManager()
	if err != nil {
		return newInfraError(fmt.Errorf("failed to initialize config manager: %w", err))
	}

	if len(args) == 0 {
		return cmd.Help()
	}

	key := args[0]

	if !manager.IsValidKey(key) {
		return newUserError("invalid configuration key: %s (valid keys: %s)", key, strings.Join(manager.GetValidKeys(), ", "))
	}

	if len(args) == 1 {
		value := manager.Get(key)
		if value == nil || value == "" {
			fmt.Printf("%s = <not set>\n", key)
			return nil
		}
		fmt.Printf("%s = %v\n", key, value)
		return nil
	}

	value := args[1]
	if err := manager.Set(key, value); err != nil {
		return newInfraError(fmt.Errorf("failed to set configuration: %w", err))
	}

	fmt.Printf("✅ Configuration set: %s = %s\n", key, value)
	return nil
}

func runConfigLsCmd(cmd *cobra.Command, args []string) error {
	manager, err := config.NewManager()
	if err != nil {
		return newInfraError(fmt.Errorf("failed to initialize config manager: %w", err))
	}

	fmt.Printf("📋 Configuration values (stored in %s):\n\n", manager.GetConfigFile())

	settings := manager.GetAll()

	validKeys := manager.GetValidKeys()

	if len(validKeys) == 0 {
		fmt.Println("No configuration items available")
		return nil
	}

	for _, key := range validKeys {
		value, exists := settings[key]

		if exists && value != nil && value != "" {
			fmt.Printf("%s = %v\n", key, value)
		} else {
			fmt.Printf("%s = <not set>\n", key)
		}
	}

	return nil
}

func runConfigRmCmd(cmd *cobra.Command, args []string) error {
	manager, err := config.NewManager()
	if err != nil {
		return newInfraError(fmt.Errorf("failed to initialize config manager: %w", err))
	}

	key := args[0]

	if !manager.IsValidKey(key) {
		return newUserError("invalid configuration key: %s (valid keys: %s)", key, strings.Join(manager.GetValidKeys(), ", "))
	}

	if err := manager.Unset(key); err != nil {
		return newInfraError(fmt.Errorf("failed to remove configuration: %w", err))
	}

	fmt.Printf("✅ Configuration reset: %s restored to default value\n", key)
	return nil
}
