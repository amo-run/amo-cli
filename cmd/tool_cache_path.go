package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"amo/pkg/env"

	"github.com/spf13/cobra"
)

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

	cachedPaths := manager.GetCachedToolPaths()
	if len(cachedPaths) > 0 {
		fmt.Println("🗺️  Tool Command → Path Mappings:")
		fmt.Println("----------------------------------")

		var commands []string
		for command := range cachedPaths {
			commands = append(commands, command)
		}

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

func runToolPathInfoCommand(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 PATH Configuration Information")
	fmt.Println("=================================")

	manager, err := createToolManager()
	if err != nil {
		return fmt.Errorf("failed to create tool manager: %w", err)
	}

	toolsDir := manager.GetInstallDir()
	fmt.Printf("Tools directory: %s\n", toolsDir)

	if _, err := os.Stat(toolsDir); os.IsNotExist(err) {
		fmt.Println("Status: Tools directory does not exist yet")
		fmt.Println("💡 Install some tools first using 'amo tool install <tool-name>'")
		return nil
	}

	envObj, err := env.NewEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	pathEnv := envObj.GetCrossPlatformUtils().GetEnvironmentVariable("PATH")
	pathSeparator := envObj.GetCrossPlatformUtils().GetPathListSeparator()

	paths := strings.Split(pathEnv, pathSeparator)
	absToolsDir, _ := filepath.Abs(toolsDir)

	inPath := false
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err == nil && absPath == absToolsDir {
			inPath = true
			break
		}
	}

	if inPath {
		fmt.Println("Status: ✅ Tools directory is in PATH")
	} else {
		fmt.Println("Status: ❌ Tools directory is NOT in PATH")
		fmt.Println("💡 Run 'amo tool path setup' to add it to PATH")
	}

	fmt.Println("")
	fmt.Println("Installed tools in directory:")

	files, err := ioutil.ReadDir(toolsDir)
	if err != nil {
		fmt.Printf("⚠️  Cannot read tools directory: %v\n", err)
		return nil
	}

	executableCount := 0
	for _, file := range files {
		if !file.IsDir() {
			icon := "📄"
			isExecutable := false
			if runtime.GOOS == "windows" {
				if strings.HasSuffix(strings.ToLower(file.Name()), ".exe") {
					isExecutable = true
				}
			} else if (file.Mode().Perm() & 0111) != 0 {
				isExecutable = true
			}
			if isExecutable {
				icon = "🔧"
				executableCount++
			}
			fmt.Printf("  %s %s (%s)\n", icon, file.Name(), formatFileSize(file.Size()))
		}
	}

	if executableCount == 0 {
		fmt.Println("  (No executable tools found)")
	} else {
		fmt.Printf("\nFound %d executable tool(s)\n", executableCount)
	}

	return nil
}

func runToolPathSetupCommand(cmd *cobra.Command, args []string) error {
	fmt.Println("🔧 Setting up tools directory in PATH")
	fmt.Println("=====================================")

	manager, err := createToolManager()
	if err != nil {
		return fmt.Errorf("failed to create tool manager: %w", err)
	}

	toolsDir := manager.GetInstallDir()
	fmt.Printf("Tools directory: %s\n", toolsDir)

	if _, err := os.Stat(toolsDir); os.IsNotExist(err) {
		fmt.Println("⚠️  Tools directory does not exist yet")
		fmt.Println("💡 Install some tools first using 'amo tool install <tool-name>'")
		return nil
	}

	if err := manager.EnsureToolsInPath(); err != nil {
		return fmt.Errorf("failed to setup PATH: %w", err)
	}

	envObj, err := env.NewEnvironment()
	if err == nil {
		pathEnv := envObj.GetCrossPlatformUtils().GetEnvironmentVariable("PATH")
		pathSeparator := envObj.GetCrossPlatformUtils().GetPathListSeparator()
		absToolsDir, _ := filepath.Abs(toolsDir)
		inPath := false
		for _, p := range strings.Split(pathEnv, pathSeparator) {
			if absP, err := filepath.Abs(p); err == nil && absP == absToolsDir {
				inPath = true
				break
			}
		}
		if !inPath {
			fmt.Println()
			fmt.Println("ℹ️  Tools directory may not be visible in the current terminal session.")
			if runtime.GOOS == "windows" {
				fmt.Println("💡 On Windows, PATH changes apply to new Command Prompt/PowerShell windows.")
				fmt.Println("   Please close and reopen your terminal.")
				fmt.Println()
				fmt.Println("Manual Setup Instructions:")
				fmt.Println("===========================")
				fmt.Println("1. Open Settings → System → About → Advanced system settings")
				fmt.Println("2. Click 'Environment Variables...'")
				fmt.Println("3. Under 'User variables', select 'Path' → 'Edit...'")
				fmt.Printf("4. Click 'New' and add: %s\n", toolsDir)
				fmt.Println("5. Click 'OK' to save, then restart your terminal")
				fmt.Println()
				fmt.Println("Alternatively (PowerShell):")
				fmt.Printf("   $env:PATH += ';%s'\n", toolsDir)
				fmt.Println("   [Environment]::SetEnvironmentVariable('PATH', $env:PATH, 'User')")
				fmt.Println()
				fmt.Println("Fallback: use the full path to run a tool, e.g.")
				fmt.Printf("   \"%s\\<tool>.exe\" --help\n", toolsDir)
			} else {
				fmt.Println("💡 On Unix-like systems, run 'source ~/.bashrc' or 'source ~/.zshrc' or reopen the terminal.")
			}
		}
	}

	return nil
}

func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}
