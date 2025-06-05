package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"amo/pkg/cli"
	"amo/pkg/workflow"

	"github.com/spf13/cobra"
)

// Command line flags for run command
var (
	runVarSpecs    []string
	runInputPath   string
	runOutputPath  string
	runHelp        bool
	runDebug       bool
	runTimeoutSecs int
)

// NewRunCmd creates the run subcommand for executing workflows
func NewRunCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run <workflow-file>",
		Short: "Run a JavaScript workflow file",
		Long: `Execute a JavaScript workflow file with optional variables and parameters.

The workflow file can be:
- An embedded workflow (e.g., file-organizer.js)
- An external file path (e.g., /path/to/my-workflow.js)

Examples:
  amo run file-organizer.js --var source_dir=/Downloads --var target_dir=/Organized
  amo run /path/to/custom-workflow.js --input /data --output /results
  amo run video-to-audio.js --var input=/videos --var format=mp3 --debug
  amo run workflow.js --timeout 3600  # With 1 hour timeout limit`,
		Args: cobra.ExactArgs(1),
		RunE: runWorkflowCommand,
	}

	// Add flags
	runCmd.Flags().StringSliceVar(&runVarSpecs, "var", []string{}, "Runtime variables (key=value)")
	runCmd.Flags().StringVar(&runInputPath, "input", "", "Input path (same as --var input=...)")
	runCmd.Flags().StringVar(&runOutputPath, "output", "", "Output path (same as --var output=...)")
	runCmd.Flags().BoolVar(&runHelp, "workflow-help", false, "Show workflow help message")
	runCmd.Flags().BoolVar(&runDebug, "debug", false, "Enable debug mode")
	runCmd.Flags().IntVar(&runTimeoutSecs, "timeout", 0, "Timeout in seconds (0 = no timeout)")

	return runCmd
}

func runWorkflowCommand(cmd *cobra.Command, args []string) error {
	workflowFile := args[0]

	if runDebug {
		fmt.Fprintf(os.Stderr, "🚀 Amo Workflow Engine\n")
		fmt.Fprintf(os.Stderr, "======================\n")
		fmt.Fprintf(os.Stderr, "Executing workflow: %s\n", workflowFile)
		fmt.Fprintf(os.Stderr, "Debug mode: enabled\n")
		if runTimeoutSecs > 0 {
			fmt.Fprintf(os.Stderr, "Timeout: %d seconds\n", runTimeoutSecs)
		} else {
			fmt.Fprintf(os.Stderr, "Timeout: unlimited\n")
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Create context with optional timeout
	var ctx context.Context
	if runTimeoutSecs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(runTimeoutSecs)*time.Second)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	engine := workflow.NewEngine(ctx)

	// Set asset reader if available
	if AssetManager != nil {
		engine.SetAssetReader(AssetManager)
	}

	// Parse variables
	vars := cli.ParseVars(runVarSpecs)

	// Add convenience variables (if not already set)
	if runHelp {
		vars["help"] = "true"
	}
	if runInputPath != "" && vars["input"] == "" {
		vars["input"] = runInputPath
	}
	if runOutputPath != "" && vars["output"] == "" {
		vars["output"] = runOutputPath
	}

	// Set variables in engine
	if len(vars) > 0 {
		engine.SetVars(vars)

		if runDebug {
			fmt.Fprintf(os.Stderr, "📋 Runtime Variables:\n")
			for key, value := range vars {
				fmt.Fprintf(os.Stderr, "  %s = %s\n", key, value)
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
	}

	// Execute workflow
	if runDebug {
		fmt.Fprintf(os.Stderr, "▶️  Starting workflow execution...\n")
		fmt.Fprintf(os.Stderr, "\n")
	}

	if err := engine.RunWorkflow(workflowFile); err != nil {
		if runDebug {
			fmt.Fprintf(os.Stderr, "\n❌ Workflow execution failed: %v\n", err)
		}
		return fmt.Errorf("failed to execute workflow %s: %w", workflowFile, err)
	}

	if runDebug {
		fmt.Fprintf(os.Stderr, "\n✅ Workflow completed successfully\n")
	}

	return nil
}
