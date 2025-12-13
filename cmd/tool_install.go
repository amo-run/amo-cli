package cmd

import (
	"fmt"
	"strings"

	"amo/pkg/tool"

	"github.com/spf13/cobra"
)

func createToolManager() (*tool.Manager, error) {
	manager, err := tool.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create tool manager: %w", err)
	}

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

	fmt.Printf("📊 Configuration: %s\n", manager.GetConfigVersion())
	fmt.Println()
	fmt.Println("⏳ Checking tools (results will appear as they are processed)...")
	fmt.Println()

	installedCount := 0
	totalTools := 0

	err = manager.CheckToolsWithCallback(func(t tool.ToolStatus) {
		if t.Installed {
			installedCount++
		}
		totalTools++

		status := tool.FormatToolStatus(t)
		fmt.Println(status)

		if showDetails && t.Error != "" {
			fmt.Printf("   🔍 Details: %s\n", t.Error)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to check tools: %w", err)
	}

	fmt.Println()
	fmt.Printf("📊 Summary: %d/%d tools installed\n", installedCount, totalTools)

	if installedCount < totalTools {
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

	manager.SetPreferMirror(preferMirror)

	if toolName == "all" {
		if sourceURL != "" {
			return fmt.Errorf("--url cannot be used with 'all'. Provide a specific tool name.")
		}
		return runToolInstallAllCommand(manager)
	}

	return runToolInstallSingleCommand(manager, toolName)
}

func runToolInstallSingleCommand(manager *tool.Manager, toolName string) error {
	fmt.Printf("📦 Installing %s\n", toolName)
	fmt.Println(strings.Repeat("=", 20+len(toolName)))

	status, err := manager.CheckTool(toolName)
	if err != nil {
		return fmt.Errorf("failed to check tool status: %w", err)
	}

	if status.Installed && !forceReinstall {
		fmt.Printf("✅ %s is already installed (%s)\n", status.Name, status.Version)
		fmt.Println("💡 Use --force flag to reinstall")
		return nil
	}

	if sourceURL != "" {
		err = manager.InstallToolWithOptions(toolName, forceReinstall, &tool.InstallOptions{URL: sourceURL})
	} else {
		err = manager.InstallTool(toolName, forceReinstall)
	}
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

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

	var successfulInstalls []string
	var skippedInstalls []string
	var failedInstalls []string

	for i, toolName := range toolNames {
		fmt.Printf("📦 [%d/%d] Installing %s...\n", i+1, len(toolNames), toolName)

		status, err := manager.CheckTool(toolName)
		if err != nil {
			fmt.Printf("❌ Failed to check %s status: %v\n", toolName, err)
			failedInstalls = append(failedInstalls, toolName)
			fmt.Println()
			continue
		}

		if status.Installed && !forceReinstall {
			fmt.Printf("✅ %s is already installed (%s) - skipping\n", status.Name, status.Version)
			skippedInstalls = append(skippedInstalls, toolName)
			fmt.Println()
			continue
		}

		err = manager.InstallTool(toolName, forceReinstall)
		if err != nil {
			fmt.Printf("❌ Installation of %s failed: %v\n", toolName, err)
			failedInstalls = append(failedInstalls, toolName)
			fmt.Println()
			continue
		}

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
