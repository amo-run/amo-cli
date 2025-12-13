package cmd

import "github.com/spf13/cobra"

// Tool command flags
var (
	forceReinstall bool
	showDetails    bool
	preferMirror   bool
	sourceURL      string
)

// NewToolCmd creates and returns the tool management command
func NewToolCmd() *cobra.Command {
	toolCmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage external tools",
		Long: `Manage external tools for amo workflows.

Subcommands:
  list       - List all supported tools and their installation status  
  install    - Install one or more tools
  permission - Manage CLI command permissions (list/add/remove)
  cache      - Manage tool path cache (info/clear)
  path       - Manage tools directory in system PATH`,
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
	installCmd.Flags().BoolVar(&preferMirror, "mirror", false, "Prefer downloading from mirror first")
	installCmd.Flags().StringVar(&sourceURL, "url", "", "Override download URL for installer or binary (advanced)")

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

	// Path subcommand
	pathCmd := &cobra.Command{
		Use:   "path",
		Short: "Manage tools directory in system PATH",
		Long: `Manage the tools directory in system PATH environment variable.

This command helps ensure that installed tools can be accessed directly from the command line.`,
	}

	// Path info subcommand
	pathInfoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show PATH configuration information",
		Long:  "Display current PATH configuration and tools directory status.",
		RunE:  runToolPathInfoCommand,
	}

	// Path setup subcommand
	pathSetupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup tools directory in system PATH",
		Long:  "Add the tools directory to system PATH environment variable.",
		RunE:  runToolPathSetupCommand,
	}

	// Add subcommands
	toolCmd.AddCommand(listCmd)
	toolCmd.AddCommand(installCmd)
	toolCmd.AddCommand(permissionCmd)
	toolCmd.AddCommand(cacheCmd)
	pathCmd.AddCommand(pathInfoCmd)
	pathCmd.AddCommand(pathSetupCmd)
	toolCmd.AddCommand(pathCmd)

	return toolCmd
}
