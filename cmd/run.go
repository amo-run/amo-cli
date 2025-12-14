package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"amo/pkg/cli"
	"amo/pkg/config"
	"amo/pkg/tool"
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

var whitelistWarningShown bool

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
	if len(args) == 0 {
		return newUserError("workflow filename is required")
	}

	// Get script path
	scriptPath := args[0]

	// Help mode - just run the workflow with --help flag
	if workflowHelp, _ := cmd.Flags().GetBool("workflow-help"); workflowHelp {
		vars := map[string]string{
			"help": "true",
		}
		if err := executeWorkflow(scriptPath, vars, 0, false); err != nil {
			return newRuntimeError(err)
		}
		return nil
	}

	// Parse variables
	varsFlag, _ := cmd.Flags().GetStringSlice("var")
	vars := cli.ParseVars(varsFlag)

	// Get timeout parameter
	timeout, _ := cmd.Flags().GetInt("timeout")

	// Get debug parameter
	debug, _ := cmd.Flags().GetBool("debug")

	// Process special shortcuts
	input, _ := cmd.Flags().GetString("input")
	if input != "" {
		vars["input"] = input
	}

	output, _ := cmd.Flags().GetString("output")
	if output != "" {
		vars["output"] = output
	}

	// Add environment variables to vars map
	if debug {
		fmt.Fprintf(os.Stderr, "📋 Adding environment variables to vars map...\n")
	}
	for _, envVar := range os.Environ() {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 && !strings.HasPrefix(parts[0], "_") {
			// Only if not explicitly set by user
			if _, exists := vars[parts[0]]; !exists {
				vars[parts[0]] = parts[1]
				if debug {
					fmt.Fprintf(os.Stderr, "  Adding env var: %s = %s\n", parts[0], parts[1])
				}
			}
		}
	}

	// Execute workflow with variables and timeout
	if err := executeWorkflow(scriptPath, vars, timeout, debug); err != nil {
		return newRuntimeError(err)
	}
	return nil
}

func executeWorkflow(scriptPath string, vars map[string]string, timeout int, debug bool) error {
	if !whitelistWarningShown {
		if manager, err := config.NewManager(); err == nil {
			if !manager.GetBool(config.KeySecurityWhitelistEnabled) {
				fmt.Fprintln(os.Stderr, "⚠️ Workflow CLI whitelist security is currently DISABLED. Workflows can execute system commands directly.")
				fmt.Fprintln(os.Stderr, "   It is strongly recommended to enable the whitelist via `amo config security_cli_whitelist_enabled true` to improve security.")
				whitelistWarningShown = true
			}
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "🚀 Amo Workflow Engine\n")
		fmt.Fprintf(os.Stderr, "======================\n")
		fmt.Fprintf(os.Stderr, "Executing workflow: %s\n", scriptPath)
		fmt.Fprintf(os.Stderr, "Debug mode: enabled\n")
		if timeout > 0 {
			fmt.Fprintf(os.Stderr, "Timeout: %d seconds\n", timeout)
		} else {
			fmt.Fprintf(os.Stderr, "Timeout: unlimited\n")
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Create context with optional timeout
	var ctx context.Context
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	engine := workflow.NewEngine(ctx)

	// Set asset reader if available
	if AssetManager != nil {
		engine.SetAssetReader(AssetManager)
	}

	// Set up tool path provider for command resolution
	toolManager, err := createToolManager()
	if err == nil {
		toolPathProvider := (*tool.Manager)(toolManager).NewToolPathProviderAdapter()
		engine.SetToolPathProvider(toolPathProvider)
	}

	// Set variables in engine
	if len(vars) > 0 {
		engine.SetVars(vars)

		if debug {
			fmt.Fprintf(os.Stderr, "📋 Runtime Variables:\n")
			for key, value := range vars {
				fmt.Fprintf(os.Stderr, "  %s = %s\n", key, value)
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
	}

	// Execute workflow
	if debug {
		fmt.Fprintf(os.Stderr, "▶️  Starting workflow execution...\n")
		fmt.Fprintf(os.Stderr, "\n")
	}

	if err := engine.RunWorkflow(scriptPath); err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "\n❌ Workflow execution failed: %v\n", err)
		}
		return fmt.Errorf("failed to execute workflow %s: %w", scriptPath, err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "\n✅ Workflow completed successfully\n")
	}

	return nil
}
