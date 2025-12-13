package cmd

import (
	"fmt"
	"strings"

	"amo/pkg/env"

	"github.com/spf13/cobra"
)

func runToolPermissionCommand(cmd *cobra.Command, args []string) error {
	fmt.Println("🔐 Workflow CLI Command Whitelist")
	fmt.Println("==================================")

	environment, err := env.NewEnvironment()
	if err != nil {
		return newInfraError(fmt.Errorf("failed to create environment: %w", err))
	}

	fmt.Printf("📁 Configuration file: %s\n", environment.GetAllowedCLIPath())
	fmt.Println()
	fmt.Println("📝 This file controls which CLI commands can be executed within JavaScript workflows.")
	fmt.Println("   It is a security whitelist to prevent unauthorized system access from workflow scripts.")
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: This is NOT for tool installation commands.")
	fmt.Println("   Only add commands that workflows need to execute directly.")
	fmt.Println()

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
		return newInfraError(fmt.Errorf("failed to create environment: %w", err))
	}

	commands, err := environment.LoadAllowedCLICommands()
	if err != nil {
		return newInfraError(fmt.Errorf("failed to load allowed commands: %w", err))
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
		return newInfraError(fmt.Errorf("failed to create environment: %w", err))
	}

	err = environment.AddAllowedCommand(command)
	if err != nil {
		if strings.Contains(err.Error(), "already in the whitelist") {
			fmt.Printf("ℹ️  Command '%s' is already in the whitelist\n", command)
			return nil
		}
		return newInfraError(fmt.Errorf("failed to add command: %w", err))
	}

	fmt.Printf("✅ Command '%s' added to whitelist\n", command)
	fmt.Println("💡 Workflows can now execute this command")

	return nil
}

func runToolPermissionRemoveCommand(cmd *cobra.Command, args []string) error {
	command := args[0]

	environment, err := env.NewEnvironment()
	if err != nil {
		return newInfraError(fmt.Errorf("failed to create environment: %w", err))
	}

	err = environment.RemoveAllowedCommand(command)
	if err != nil {
		if strings.Contains(err.Error(), "not in the whitelist") {
			fmt.Printf("ℹ️  Command '%s' is not in the whitelist\n", command)
			return nil
		}
		return newInfraError(fmt.Errorf("failed to remove command: %w", err))
	}

	fmt.Printf("✅ Command '%s' removed from whitelist\n", command)
	fmt.Println("⚠️  Workflows can no longer execute this command")

	return nil
}
