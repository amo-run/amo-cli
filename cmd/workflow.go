package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"amo/pkg/workflow"

	"github.com/spf13/cobra"
)

// NewWorkflowCmd creates the workflow subcommand with subcommands
func NewWorkflowCmd() *cobra.Command {
	workflowCmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflow files",
		Long:  "Manage workflow files: list available workflows or download new ones from remote sources.",
	}

	// Add subcommands
	workflowCmd.AddCommand(NewWorkflowGetCmd())
	workflowCmd.AddCommand(NewWorkflowListCmd())
	workflowCmd.AddCommand(NewWorkflowSourceCmd())

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

// NewWorkflowSourceCmd creates the workflow source subcommand group
func NewWorkflowSourceCmd() *cobra.Command {
	sourceCmd := &cobra.Command{
		Use:   "source",
		Short: "Manage workflow download sources",
		Long:  "Configure allowed sources (domains or domain/path) for workflow downloads.",
	}

	sourceCmd.AddCommand(NewWorkflowSourceListCmd())
	sourceCmd.AddCommand(NewWorkflowSourceAddCmd())
	sourceCmd.AddCommand(NewWorkflowSourceRmCmd())

	return sourceCmd
}

// NewWorkflowSourceListCmd lists configured workflow sources
func NewWorkflowSourceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured workflow download sources",
		RunE:  listWorkflowSources,
	}
}

// NewWorkflowSourceAddCmd adds a new workflow source
func NewWorkflowSourceAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <domain>[/<path>]",
		Short: "Add a workflow download source (domain or domain/path)",
		Args:  cobra.ExactArgs(1),
		RunE:  addWorkflowSource,
	}
}

// NewWorkflowSourceRmCmd removes a workflow source
func NewWorkflowSourceRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <domain>[/<path>]",
		Aliases: []string{"remove", "del", "delete"},
		Short:   "Remove a workflow download source",
		Args:    cobra.ExactArgs(1),
		RunE:    removeWorkflowSource,
	}
}

// listAllWorkflows lists both user and embedded workflows
func listAllWorkflows(cmd *cobra.Command, args []string) error {
	// Get the workflow downloader
	downloader, err := workflow.NewWorkflowDownloader()
	if err != nil {
		return fmt.Errorf("failed to initialize workflow downloader: %w", err)
	}

	// Check for configured directory
	configuredDir := downloader.GetConfiguredWorkflowsDir()
	hasConfiguredDir := configuredDir != ""

	// Check default directory
	defaultWorkflowsDir := downloader.GetWorkflowsDir()

	fmt.Println("📋 Available workflow files:")
	fmt.Println("==========================")

	// A function to list workflows from a specific directory
	listWorkflowsFromDir := func(dir string, label string) error {
		// Check if directory exists
		if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
			return nil // Directory doesn't exist, nothing to list
		}

		// List workflows in the directory (including subdirectories)
		var workflows []string

		// Walk through all files recursively
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories themselves
			if info.IsDir() {
				return nil
			}

			// Check if it's a JS file
			if strings.HasSuffix(strings.ToLower(info.Name()), ".js") {
				// Get relative path from the workflows directory
				relPath, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				workflows = append(workflows, relPath)
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to walk directory %s: %w", dir, err)
		}

		if len(workflows) > 0 {
			fmt.Printf("📁 %s:\n", label)
			// Sort the workflows for consistent output
			sort.Strings(workflows)
			for _, wf := range workflows {
				// For files in subdirectories, use a different prefix
				if strings.Contains(wf, string(filepath.Separator)) {
					// Show subfolder structure with a different icon
					fmt.Printf("  - 📂 %s\n", wf)
				} else {
					fmt.Printf("  - 📄 %s\n", wf)
				}
			}
			fmt.Println()
			return nil
		}
		return nil
	}

	// 1. List from configured directory if available
	if hasConfiguredDir {
		if err := listWorkflowsFromDir(configuredDir, fmt.Sprintf("User workflows (configured: %s)", configuredDir)); err != nil {
			fmt.Printf("⚠️ %s\n\n", err)
		}
	}

	// 2. List from default downloads directory
	if !hasConfiguredDir || configuredDir != defaultWorkflowsDir {
		if err := listWorkflowsFromDir(defaultWorkflowsDir, fmt.Sprintf("Downloaded workflows (%s)", defaultWorkflowsDir)); err != nil {
			fmt.Printf("⚠️ %s\n\n", err)
		}
	}

	// 3. List embedded workflows
	if AssetManager == nil {
		fmt.Println("No embedded workflows available")
		return nil
	}

	workflows, err := AssetManager.GetWorkflowFileNames()
	if err != nil {
		return fmt.Errorf("failed to list embedded workflows: %w", err)
	}

	if len(workflows) > 0 {
		fmt.Println("📦 Embedded workflows:")
		for _, wf := range workflows {
			fmt.Printf("  - %s\n", wf)
		}
		fmt.Println()
	} else {
		fmt.Println("No embedded workflows found")
		return nil
	}

	fmt.Println("📌 Usage: amo run <workflow-name>")
	if len(workflows) > 0 {
		fmt.Printf("Example: amo run %s\n", workflows[0])
	}

	// Show tip about configuration
	if !hasConfiguredDir {
		fmt.Println("\n💡 Tip: Set a custom workflows directory with: amo config set workflows /path/to/workflows")
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

	// Always use the default workflows directory for downloads
	targetDir := downloader.GetWorkflowsDir()

	workflowPath := filepath.Join(targetDir, actualFilename)
	fmt.Printf("✅ Workflow downloaded successfully to: %s\n", workflowPath)
	fmt.Printf("Run with: amo run %s\n", actualFilename)

	return nil
}

// listWorkflowSources lists current workflow download sources
func listWorkflowSources(cmd *cobra.Command, args []string) error {
	downloader, err := workflow.NewWorkflowDownloader()
	if err != nil {
		return fmt.Errorf("failed to initialize workflow downloader: %w", err)
	}

	// Ensure file exists; if not, create with defaults
	if err := downloader.EnsureAllowedSourcesFile(); err != nil {
		return fmt.Errorf("failed to ensure sources file: %w", err)
	}

	sources, err := downloader.ListAllowedSources()
	if err != nil {
		return fmt.Errorf("failed to list workflow sources: %w", err)
	}

	fmt.Println("📋 Allowed workflow download sources:")
	fmt.Println("====================================")
	for _, s := range sources {
		fmt.Printf("- %s\n", s)
	}
	fmt.Println()
	fmt.Printf("Config file: %s\n", downloader.GetAllowedSourcesFilePath())
	return nil
}

// addWorkflowSource adds a source entry
func addWorkflowSource(cmd *cobra.Command, args []string) error {
	entry := strings.TrimSpace(args[0])
	if entry == "" {
		return fmt.Errorf("source cannot be empty")
	}

	downloader, err := workflow.NewWorkflowDownloader()
	if err != nil {
		return fmt.Errorf("failed to initialize workflow downloader: %w", err)
	}

	created, err := downloader.AddAllowedSource(entry)
	if err != nil {
		return fmt.Errorf("failed to add source: %w", err)
	}
	if created {
		fmt.Printf("✅ Added source: %s\n", entry)
	} else {
		fmt.Printf("ℹ️  Source already exists: %s\n", entry)
	}
	return nil
}

// removeWorkflowSource removes a source entry
func removeWorkflowSource(cmd *cobra.Command, args []string) error {
	entry := strings.TrimSpace(args[0])
	if entry == "" {
		return fmt.Errorf("source cannot be empty")
	}

	downloader, err := workflow.NewWorkflowDownloader()
	if err != nil {
		return fmt.Errorf("failed to initialize workflow downloader: %w", err)
	}

	removed, err := downloader.RemoveAllowedSource(entry)
	if err != nil {
		return fmt.Errorf("failed to remove source: %w", err)
	}
	if removed {
		fmt.Printf("✅ Removed source: %s\n", entry)
	} else {
		fmt.Printf("ℹ️  Source not found: %s\n", entry)
	}
	return nil
}
