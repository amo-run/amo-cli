package cmd

import (
	"fmt"
	"os"

	"amo/pkg/workflow"

	"github.com/spf13/cobra"
)

// NewWorkflowCmd creates the workflow subcommand with subcommands
func NewWorkflowCmd() *cobra.Command {
	workflowCmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflow files",
		Long:  "Manage workflow files: list available workflows or download new ones from remote sources.",
		RunE:  listAllWorkflows,
	}

	// Add subcommands
	workflowCmd.AddCommand(NewWorkflowGetCmd())
	workflowCmd.AddCommand(NewWorkflowListCmd())

	return workflowCmd
}

// NewWorkflowListCmd creates the workflow list subcommand
func NewWorkflowListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available workflow files",
		Long:  "List all available workflow files from user directory and embedded assets.",
		RunE:  listAllWorkflows,
	}
}

// NewWorkflowGetCmd creates the workflow get subcommand
func NewWorkflowGetCmd() *cobra.Command {
	var filename string

	getCmd := &cobra.Command{
		Use:   "get <url>",
		Short: "Download a workflow from a remote source",
		Long: `Download a workflow script from an allowed remote source.

Supported domains (whitelist):
- github.com (automatically converts to raw.githubusercontent.com)
- gitlab.com (automatically converts to raw content URL)
- bitbucket.org
- sourceforge.net

The downloaded workflow will be saved to the user config directory (~/.amo/workflows/).

Examples:
  amo workflow get https://github.com/user/repo/blob/main/workflow.js
  amo workflow get https://gitlab.com/user/repo/-/blob/main/workflow.js --filename my-workflow.js
  amo workflow get https://raw.githubusercontent.com/user/repo/main/workflow.js`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return downloadWorkflow(args[0], filename)
		},
	}

	getCmd.Flags().StringVar(&filename, "filename", "", "Custom filename for the downloaded workflow (optional)")

	return getCmd
}

// listAllWorkflows lists both user and embedded workflows
func listAllWorkflows(cmd *cobra.Command, args []string) error {
	// List user workflows
	downloader, err := workflow.NewWorkflowDownloader()
	if err == nil {
		userWorkflows, err := downloader.ListUserWorkflows()
		if err == nil && len(userWorkflows) > 0 {
			fmt.Println("User workflows (in ~/.amo/workflows/):")
			for _, workflow := range userWorkflows {
				fmt.Printf("  %s\n", workflow)
			}
			fmt.Println()
		}
	}

	// List embedded workflows
	if AssetManager == nil {
		fmt.Println("No embedded workflows available")
		return nil
	}

	workflows, err := AssetManager.GetWorkflowFileNames()
	if err != nil {
		return fmt.Errorf("failed to list embedded workflows: %w", err)
	}

	if len(workflows) == 0 {
		fmt.Println("No embedded workflows found")
		return nil
	}

	fmt.Println("Embedded workflows:")
	for _, workflow := range workflows {
		fmt.Printf("  %s\n", workflow)
	}

	fmt.Printf("\nUsage: amo run <workflow-name>\n")
	if len(workflows) > 0 {
		fmt.Printf("Example: amo run %s\n", workflows[0])
	}

	return nil
}

// downloadWorkflow downloads a workflow from the given URL
func downloadWorkflow(url, filename string) error {
	downloader, err := workflow.NewWorkflowDownloader()
	if err != nil {
		return fmt.Errorf("failed to initialize workflow downloader: %w", err)
	}

	fmt.Printf("Downloading workflow from: %s\n", url)

	if filename != "" {
		fmt.Printf("Saving as: %s\n", filename)
	}

	err = downloader.DownloadWorkflow(url, filename)
	if err != nil {
		return fmt.Errorf("failed to download workflow: %w", err)
	}

	// Determine the actual filename used
	actualFilename := filename
	if actualFilename == "" {
		actualFilename, err = downloader.ExtractFilename(url)
		if err != nil {
			actualFilename = "workflow.js" // fallback
		}
	}

	workflowPath := downloader.GetWorkflowsDir() + string(os.PathSeparator) + actualFilename
	fmt.Printf("✅ Workflow downloaded successfully to: %s\n", workflowPath)
	fmt.Printf("Run with: amo run %s\n", actualFilename)

	return nil
}
