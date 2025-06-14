package cmd

import (
	"fmt"

	"amo/pkg/workflow"

	"github.com/spf13/cobra"
)

// Version information set by build flags
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
	Debug     bool
)

// Global asset manager
var AssetManager workflow.AssetReader

// No global flags for root command anymore - workflow execution moved to run subcommand

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		SilenceUsage: true,
		Use:          "amo",
		Short:        "A CLI tool for managing tools and running JavaScript-based workflows",
		Long: `amo is a command-line tool that manages tools and executes JavaScript-based workflows.
It supports variable management and system command execution through a JavaScript runtime.

Use 'amo run <workflow-file>' to execute workflows.
Use 'amo tool' to manage tools.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime),
	}

	// Add subcommands
	rootCmd.AddCommand(NewRunCmd())
	rootCmd.AddCommand(NewWorkflowCmd())
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewToolCmd())
	rootCmd.AddCommand(NewConfigCmd())

	return rootCmd
}
