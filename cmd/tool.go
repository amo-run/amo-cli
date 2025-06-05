package cmd

import (
	"fmt"
	"os"
	"strings"

	"amo/pkg/env"
	"amo/pkg/tool"

	"github.com/spf13/cobra"
)

// Tool command flags
var (
	forceReinstall bool
	showDetails    bool
)

// NewToolCmd creates and returns the tool management command
func NewToolCmd() *cobra.Command {
	toolCmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage external tools and applications",
		Long: `Manage external tools and applications required by workflows.
This includes checking installation status, automatic installation, 
and updating of tools like ffmpeg, imagemagick, pandoc, etc.`,
	}

	// List subcommand
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all supported tools and their status",
		Long:    "Display a list of all supported tools with their installation status and versions.",
		RunE:    runToolListCommand,
	}
	listCmd.Flags().BoolVar(&showDetails, "details", false, "Show detailed information for each tool")

	// Install subcommand
	installCmd := &cobra.Command{
		Use:   "install <tool-name|all>",
		Short: "Install a specific tool or all supported tools",
		Long:  "Install a specific tool or all supported tools automatically (no confirmation required).",
		Args:  cobra.ExactArgs(1),
		RunE:  runToolInstallCommand,
	}
	installCmd.Flags().BoolVar(&forceReinstall, "force", false, "Force reinstall even if tool is already installed")

	// Permission subcommand
	permissionCmd := &cobra.Command{
		Use:   "permission",
		Short: "Manage workflow CLI command whitelist",
		Long:  "Manage the workflow CLI command whitelist for security control.",
		RunE:  runToolPermissionCommand,
	}

	// Permission list subcommand
	permissionListCmd := &cobra.Command{
		Use:   "list",
		Short: "List allowed CLI commands",
		Long:  "Display all commands in the whitelist.",
		RunE:  runToolPermissionListCommand,
	}

	// Permission add subcommand
	permissionAddCmd := &cobra.Command{
		Use:   "add <command>",
		Short: "Add command to whitelist",
		Long:  "Add a CLI command to the workflow whitelist.",
		Args:  cobra.ExactArgs(1),
		RunE:  runToolPermissionAddCommand,
	}

	// Permission remove subcommand
	permissionRemoveCmd := &cobra.Command{
		Use:   "remove <command>",
		Short: "Remove command from whitelist",
		Long:  "Remove a CLI command from the workflow whitelist.",
		Args:  cobra.ExactArgs(1),
		RunE:  runToolPermissionRemoveCommand,
	}

	// Add permission subcommands
	permissionCmd.AddCommand(permissionListCmd)
	permissionCmd.AddCommand(permissionAddCmd)
	permissionCmd.AddCommand(permissionRemoveCmd)

	// Cache subcommand
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage tool path cache",
		Long:  "View and manage the tool path cache file that stores discovered tool locations.",
		RunE:  runToolCacheInfoCommand, // Default to info command
	}

	// Cache info subcommand
	cacheInfoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show tool path cache information",
		Long:  "Display information about the tool path cache file.",
		RunE:  runToolCacheInfoCommand,
	}

	// Cache clear subcommand
	cacheClearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear tool path cache",
		Long:  "Clear all cached tool paths to force re-detection.",
		RunE:  runToolCacheClearCommand,
	}

	// Add cache subcommands
	cacheCmd.AddCommand(cacheInfoCmd)
	cacheCmd.AddCommand(cacheClearCmd)

	// Add subcommands
	toolCmd.AddCommand(listCmd)
	toolCmd.AddCommand(installCmd)
	toolCmd.AddCommand(permissionCmd)
	toolCmd.AddCommand(cacheCmd)

	return toolCmd
}

// createToolManager creates and initializes a tool manager with configuration
func createToolManager() (*tool.Manager, error) {
	// Create tool manager
	manager, err := tool.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create tool manager: %w", err)
	}

	// Load configuration from embedded assets
	if AssetManager == nil {
		return nil, fmt.Errorf("asset manager not available")
	}

	configStr, err := AssetManager.ReadFileAsString("tools.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read tool configuration: %w", err)
	}

	if err := manager.LoadConfig([]byte(configStr)); err != nil {
		return nil, fmt.Errorf("failed to load tool configuration: %w", err)
	}

	return manager, nil
}

func runToolListCommand(cmd *cobra.Command, args []string) error {
	fmt.Println("🛠️  Tool Manager")
	fmt.Println("================")

	manager, err := createToolManager()
	if err != nil {
		return err
	}

	tools, err := manager.ListTools()
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	fmt.Printf("📊 Configuration: %s\n", manager.GetConfigVersion())
	fmt.Println()

	// Group tools by category
	categories := make(map[string][]tool.ToolStatus)
	for _, t := range tools {
		// Get category from config - this would need to be added to the tool status
		category := "tools" // Default category
		categories[category] = append(categories[category], t)
	}

	installedCount := 0
	for _, t := range tools {
		if t.Installed {
			installedCount++
		}

		status := tool.FormatToolStatus(t)
		fmt.Println(status)

		if showDetails && t.Error != "" {
			fmt.Printf("   🔍 Details: %s\n", t.Error)
		}
	}

	fmt.Println()
	fmt.Printf("📊 Summary: %d/%d tools installed\n", installedCount, len(tools))

	if installedCount < len(tools) {
		fmt.Println()
		fmt.Println("💡 Usage:")
		fmt.Println("   amo tool list                 - List all tools with status")
		fmt.Println("   amo tool install <tool>       - Install tool automatically")
		fmt.Println("   amo tool install all          - Install all supported tools")
	}

	return nil
}

func runToolInstallCommand(cmd *cobra.Command, args []string) error {
	toolName := args[0]

	manager, err := createToolManager()
	if err != nil {
		return err
	}

	// Handle "all" case for bulk installation
	if toolName == "all" {
		return runToolInstallAllCommand(manager)
	}

	// Handle individual tool installation
	return runToolInstallSingleCommand(manager, toolName)
}

func runToolInstallSingleCommand(manager *tool.Manager, toolName string) error {
	fmt.Printf("📦 Installing %s\n", toolName)
	fmt.Println(strings.Repeat("=", 20+len(toolName)))

	// Check current status first
	status, err := manager.CheckTool(toolName)
	if err != nil {
		return fmt.Errorf("failed to check tool status: %w", err)
	}

	if status.Installed && !forceReinstall {
		fmt.Printf("✅ %s is already installed (%s)\n", status.Name, status.Version)
		fmt.Println("💡 Use --force flag to reinstall")
		return nil
	}

	// Perform installation (automatic by default)
	err = manager.InstallTool(toolName, forceReinstall)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Verify installation
	fmt.Println()
	fmt.Println("🔍 Verifying installation...")

	newStatus, err := manager.CheckTool(toolName)
	if err != nil {
		fmt.Printf("⚠️  Installation completed but verification failed: %v\n", err)
		return nil
	}

	if newStatus.Installed {
		fmt.Printf("✅ %s successfully installed (%s)\n", newStatus.Name, newStatus.Version)
	} else {
		fmt.Printf("❌ Installation may have failed - tool not detected\n")
		if newStatus.Error != "" {
			fmt.Printf("   Error: %s\n", newStatus.Error)
		}
	}

	return nil
}

func runToolInstallAllCommand(manager *tool.Manager) error {
	fmt.Println("📦 Installing All Supported Tools")
	fmt.Println("==================================")
	fmt.Println()

	// Get all tool names
	toolNames := manager.GetToolNames()
	if len(toolNames) == 0 {
		fmt.Println("❌ No tools found in configuration")
		return nil
	}

	fmt.Printf("🔍 Found %d tools to install:\n", len(toolNames))
	for i, name := range toolNames {
		fmt.Printf("  %d. %s\n", i+1, name)
	}
	fmt.Println()

	// Track installation results
	var successfulInstalls []string
	var skippedInstalls []string
	var failedInstalls []string

	// Install each tool
	for i, toolName := range toolNames {
		fmt.Printf("📦 [%d/%d] Installing %s...\n", i+1, len(toolNames), toolName)

		// Check current status
		status, err := manager.CheckTool(toolName)
		if err != nil {
			fmt.Printf("❌ Failed to check %s status: %v\n", toolName, err)
			failedInstalls = append(failedInstalls, toolName)
			fmt.Println()
			continue
		}

		// Skip if already installed and not forcing reinstall
		if status.Installed && !forceReinstall {
			fmt.Printf("✅ %s is already installed (%s) - skipping\n", status.Name, status.Version)
			skippedInstalls = append(skippedInstalls, toolName)
			fmt.Println()
			continue
		}

		// Attempt installation
		err = manager.InstallTool(toolName, forceReinstall)
		if err != nil {
			fmt.Printf("❌ Installation of %s failed: %v\n", toolName, err)
			failedInstalls = append(failedInstalls, toolName)
			fmt.Println()
			continue
		}

		// Verify installation
		newStatus, err := manager.CheckTool(toolName)
		if err != nil || !newStatus.Installed {
			fmt.Printf("❌ Installation of %s completed but verification failed\n", toolName)
			failedInstalls = append(failedInstalls, toolName)
		} else {
			fmt.Printf("✅ %s successfully installed (%s)\n", newStatus.Name, newStatus.Version)
			successfulInstalls = append(successfulInstalls, toolName)
		}
		fmt.Println()
	}

	// Print summary
	fmt.Println("📊 Installation Summary")
	fmt.Println("=======================")
	fmt.Printf("✅ Successfully installed: %d tools\n", len(successfulInstalls))
	for _, name := range successfulInstalls {
		fmt.Printf("   • %s\n", name)
	}

	if len(skippedInstalls) > 0 {
		fmt.Printf("⏭️  Skipped (already installed): %d tools\n", len(skippedInstalls))
		for _, name := range skippedInstalls {
			fmt.Printf("   • %s\n", name)
		}
	}

	if len(failedInstalls) > 0 {
		fmt.Printf("❌ Failed to install: %d tools\n", len(failedInstalls))
		for _, name := range failedInstalls {
			fmt.Printf("   • %s\n", name)
		}
		fmt.Println()
		fmt.Println("💡 You can try installing failed tools individually:")
		for _, name := range failedInstalls {
			fmt.Printf("   amo tool install %s\n", name)
		}
	}

	fmt.Printf("\n🎯 Total: %d/%d tools successfully installed\n", len(successfulInstalls), len(toolNames))

	return nil
}

func runToolPermissionCommand(cmd *cobra.Command, args []string) error {
	fmt.Println("🔐 Workflow CLI Command Whitelist")
	fmt.Println("==================================")

	// Create environment instance to get whitelist path and commands
	environment, err := env.NewEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	fmt.Printf("📁 Configuration file: %s\n", environment.GetAllowedCLIPath())
	fmt.Println()
	fmt.Println("📝 This file controls which CLI commands can be executed within JavaScript workflows.")
	fmt.Println("   It is a security whitelist to prevent unauthorized system access from workflow scripts.")
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: This is NOT for tool installation commands.")
	fmt.Println("   Only add commands that workflows need to execute directly.")
	fmt.Println()

	// Show current whitelist
	fmt.Println("📋 Current allowed commands:")
	commands, err := environment.LoadAllowedCLICommands()
	if err != nil {
		fmt.Printf("   ❌ Failed to load commands: %v\n", err)
	} else if len(commands) == 0 {
		fmt.Println("   (No commands currently allowed)")
	} else {
		for _, cmd := range commands {
			fmt.Printf("   • %s\n", cmd)
		}
	}

	fmt.Println()
	fmt.Println("💡 Management commands:")
	fmt.Println("   amo tool permission list         - List allowed commands")
	fmt.Println("   amo tool permission add <cmd>    - Add command to whitelist")
	fmt.Println("   amo tool permission remove <cmd> - Remove command from whitelist")
	fmt.Println()
	fmt.Println("🚫 Do NOT add package managers or system commands like:")
	fmt.Println("   - brew, apt, yum, pip (these are for tool installation only)")
	fmt.Println("   - sudo, chmod (these are system administration commands)")

	return nil
}

func runToolPermissionListCommand(cmd *cobra.Command, args []string) error {
	environment, err := env.NewEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	commands, err := environment.LoadAllowedCLICommands()
	if err != nil {
		return fmt.Errorf("failed to load allowed commands: %w", err)
	}

	fmt.Println("📋 Allowed CLI Commands:")
	fmt.Println("========================")

	if len(commands) == 0 {
		fmt.Println("(No commands currently allowed)")
		fmt.Println()
		fmt.Println("💡 Add commands with: amo tool permission add <command>")
	} else {
		for i, cmd := range commands {
			fmt.Printf("%2d. %s\n", i+1, cmd)
		}
		fmt.Printf("\nTotal: %d command(s)\n", len(commands))
	}

	return nil
}

func runToolPermissionAddCommand(cmd *cobra.Command, args []string) error {
	command := args[0]

	environment, err := env.NewEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	err = environment.AddAllowedCommand(command)
	if err != nil {
		if strings.Contains(err.Error(), "already in the whitelist") {
			fmt.Printf("ℹ️  Command '%s' is already in the whitelist\n", command)
			return nil
		}
		return fmt.Errorf("failed to add command: %w", err)
	}

	fmt.Printf("✅ Command '%s' added to whitelist\n", command)
	fmt.Println("💡 Workflows can now execute this command")

	return nil
}

func runToolPermissionRemoveCommand(cmd *cobra.Command, args []string) error {
	command := args[0]

	environment, err := env.NewEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	err = environment.RemoveAllowedCommand(command)
	if err != nil {
		if strings.Contains(err.Error(), "not in the whitelist") {
			fmt.Printf("ℹ️  Command '%s' is not in the whitelist\n", command)
			return nil
		}
		return fmt.Errorf("failed to remove command: %w", err)
	}

	fmt.Printf("✅ Command '%s' removed from whitelist\n", command)
	fmt.Println("⚠️  Workflows can no longer execute this command")

	return nil
}

func runToolCacheInfoCommand(cmd *cobra.Command, args []string) error {
	fmt.Println("📁 Tool Path Cache Information")
	fmt.Println("==============================")

	manager, err := createToolManager()
	if err != nil {
		return err
	}

	cacheInfo := manager.GetToolPathCacheInfo()

	fmt.Printf("📂 Cache File: %s\n", cacheInfo["cache_file"])
	fmt.Printf("🔖 Version: %s\n", cacheInfo["version"])
	fmt.Printf("⏰ Last Updated: %s\n", cacheInfo["timestamp"])
	fmt.Printf("🔧 Cached Tools: %d\n", cacheInfo["tool_count"])
	fmt.Println()

	// Display cached tool command-to-path mappings
	cachedPaths := manager.GetCachedToolPaths()
	if len(cachedPaths) > 0 {
		fmt.Println("🗺️  Tool Command → Path Mappings:")
		fmt.Println("----------------------------------")

		// Sort commands for consistent display
		var commands []string
		for command := range cachedPaths {
			commands = append(commands, command)
		}

		// Simple sort (no need to import sort for this)
		for i := 0; i < len(commands); i++ {
			for j := i + 1; j < len(commands); j++ {
				if commands[i] > commands[j] {
					commands[i], commands[j] = commands[j], commands[i]
				}
			}
		}

		for _, command := range commands {
			path := cachedPaths[command]
			fmt.Printf("  %-15s → %s\n", command, path)
		}
		fmt.Println()
	} else {
		fmt.Println("ℹ️  No tool paths cached yet.")
		fmt.Println()
	}

	fmt.Println("💡 The cache file stores discovered tool paths for faster access.")
	fmt.Println("   You can manually edit this file to specify custom tool locations.")
	fmt.Println("   Use 'amo tool cache clear' to force re-detection of all tools.")

	return nil
}

func runToolCacheClearCommand(cmd *cobra.Command, args []string) error {
	fmt.Println("🗑️ Clearing Tool Path Cache")
	fmt.Println("============================")

	manager, err := createToolManager()
	if err != nil {
		return err
	}

	cacheInfo := manager.GetToolPathCacheInfo()
	cacheFile := cacheInfo["cache_file"].(string)

	// Remove the cache file
	if err := os.Remove(cacheFile); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("ℹ️  Cache file does not exist - nothing to clear")
			return nil
		}
		return fmt.Errorf("failed to remove cache file: %w", err)
	}

	fmt.Println("✅ Tool path cache cleared successfully")
	fmt.Println("💡 Tool paths will be re-detected on next check")

	return nil
}
