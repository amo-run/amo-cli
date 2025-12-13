package env

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func (e *Environment) EnsureToolsDirInPath(toolsDir string) error {
	if e.isToolsDirInPath(toolsDir) {
		return nil
	}

	if err := e.addToolsDirToPath(toolsDir); err != nil {
		e.printManualPathInstructions(toolsDir, err)
		return nil
	}

	label := "system PATH"
	if runtime.GOOS == "windows" {
		label = "user PATH"
	}
	fmt.Printf("✅ Successfully added %s to %s\n", toolsDir, label)
	if runtime.GOOS == "windows" {
		fmt.Println("💡 Please restart your terminal (or sign out/in) to apply changes")
	} else {
		fmt.Println("💡 Please restart your terminal or run 'source ~/.zshrc' (or appropriate shell config) to apply changes")
	}

	return nil
}

func (e *Environment) isToolsDirInPath(toolsDir string) bool {
	pathEnv := e.crossPlatform.GetEnvironmentVariable("PATH")
	pathSeparator := e.crossPlatform.GetPathListSeparator()

	paths := strings.Split(pathEnv, pathSeparator)
	absToolsDir, err := filepath.Abs(toolsDir)
	if err != nil {
		return false
	}

	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err == nil && absPath == absToolsDir {
			return true
		}
	}

	return false
}

func (e *Environment) addToolsDirToPath(toolsDir string) error {
	switch runtime.GOOS {
	case "darwin", "linux":
		return e.addToUnixPath(toolsDir)
	case "windows":
		return e.addToWindowsPath(toolsDir)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (e *Environment) addToUnixPath(toolsDir string) error {
	homeDir, err := e.crossPlatform.GetHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	shell := e.getCurrentShell()
	configFile := e.getShellConfigFile(shell, homeDir)

	if configFile == "" {
		return fmt.Errorf("could not determine shell configuration file")
	}

	exportLine := fmt.Sprintf("export PATH=\"$PATH:%s\"", toolsDir)
	comment := "# Added by amo-cli for tool management"

	if e.isPathInConfigFile(configFile, toolsDir) {
		return nil
	}

	return e.appendToConfigFile(configFile, comment, exportLine)
}

func (e *Environment) addToWindowsPath(toolsDir string) error {
	psPath, psErr := exec.LookPath("powershell")
	if psErr == nil {
		escaped := strings.ReplaceAll(toolsDir, "'", "''")
		script := "$tools='" + escaped + "';" +
			"$current=[Environment]::GetEnvironmentVariable('PATH','User');" +
			"if([string]::IsNullOrEmpty($current)){ $current='' }" +
			";$parts=@(); if($current -ne ''){ $parts = $current.Split(';') | Where-Object { $_ -ne '' } }" +
			";if($parts -contains $tools){ exit 0 }" +
			";$sep = ($current -ne '' -and -not $current.TrimEnd().EndsWith(';')) ? ';' : '';" +
			"$new = $current + $sep + $tools;" +
			"[Environment]::SetEnvironmentVariable('PATH',$new,'User')"
		cmd := exec.Command(psPath, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	cmd := exec.Command("cmd", "/c", "setx", "PATH", fmt.Sprintf("\"%%PATH%%;%s\"", toolsDir))
	if err := cmd.Run(); err == nil {
		return nil
	}

	return fmt.Errorf("failed to modify user PATH automatically on Windows")
}

func (e *Environment) getCurrentShell() string {
	shell := e.crossPlatform.GetEnvironmentVariable("SHELL")
	if shell == "" {
		if runtime.GOOS == "darwin" {
			return "zsh"
		}
		return "bash"
	}

	return filepath.Base(shell)
}

func (e *Environment) getShellConfigFile(shell, homeDir string) string {
	switch shell {
	case "zsh":
		zshrc := filepath.Join(homeDir, ".zshrc")
		if _, err := os.Stat(zshrc); err == nil {
			return zshrc
		}
		return filepath.Join(homeDir, ".zprofile")
	case "bash":
		bashrc := filepath.Join(homeDir, ".bashrc")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc
		}
		bashProfile := filepath.Join(homeDir, ".bash_profile")
		if _, err := os.Stat(bashProfile); err == nil {
			return bashProfile
		}
		return filepath.Join(homeDir, ".profile")
	case "fish":
		configDir := filepath.Join(homeDir, ".config", "fish")
		return filepath.Join(configDir, "config.fish")
	default:
		return filepath.Join(homeDir, ".profile")
	}
}

func (e *Environment) isPathInConfigFile(configFile, toolsDir string) bool {
	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		return false
	}

	contentStr := string(content)
	return strings.Contains(contentStr, toolsDir) &&
		(strings.Contains(contentStr, "export PATH") || strings.Contains(contentStr, "PATH="))
}

func (e *Environment) appendToConfigFile(configFile, comment, exportLine string) error {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := e.crossPlatform.CreateFileWithPermissions(configFile, []byte(""), false); err != nil {
			return fmt.Errorf("failed to create config file %s: %w", configFile, err)
		}
	}

	file, err := os.OpenFile(configFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open config file %s: %w", configFile, err)
	}
	defer file.Close()

	content, err := ioutil.ReadFile(configFile)
	if err == nil && len(content) > 0 && content[len(content)-1] != '\n' {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	lines := []string{
		"",
		comment,
		exportLine,
		"",
	}

	for _, line := range lines {
		if _, err := file.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("failed to write to config file: %w", err)
		}
	}

	return nil
}

func (e *Environment) printManualPathInstructions(toolsDir string, err error) {
	fmt.Printf("⚠️  Could not automatically add tools directory to PATH: %v\n", err)
	fmt.Println("")
	fmt.Println("📋 Manual Setup Instructions:")
	fmt.Println("=============================")

	switch runtime.GOOS {
	case "darwin":
		e.printMacOSInstructions(toolsDir)
	case "linux":
		e.printLinuxInstructions(toolsDir)
	case "windows":
		e.printWindowsInstructions(toolsDir)
	default:
		e.printGenericUnixInstructions(toolsDir)
	}
}

func (e *Environment) printMacOSInstructions(toolsDir string) {
	shell := e.getCurrentShell()

	fmt.Println("For macOS:")
	fmt.Printf("1. Open Terminal and edit your shell configuration file:\n")

	switch shell {
	case "zsh":
		fmt.Printf("   nano ~/.zshrc\n")
	case "bash":
		fmt.Printf("   nano ~/.bash_profile\n")
	default:
		fmt.Printf("   nano ~/.zshrc    # for zsh (default on macOS)\n")
		fmt.Printf("   nano ~/.bash_profile    # for bash\n")
	}

	fmt.Printf("\n2. Add this line at the end of the file:\n")
	fmt.Printf("   export PATH=\"$PATH:%s\"\n", toolsDir)
	fmt.Printf("\n3. Save the file (Ctrl+X, then Y, then Enter in nano)\n")
	fmt.Printf("\n4. Reload your shell configuration:\n")

	switch shell {
	case "zsh":
		fmt.Printf("   source ~/.zshrc\n")
	case "bash":
		fmt.Printf("   source ~/.bash_profile\n")
	default:
		fmt.Printf("   source ~/.zshrc    # for zsh\n")
		fmt.Printf("   source ~/.bash_profile    # for bash\n")
	}

	fmt.Printf("\n5. Verify the setup:\n")
	fmt.Printf("   echo $PATH | grep %s\n", toolsDir)
}

func (e *Environment) printLinuxInstructions(toolsDir string) {
	fmt.Println("For Linux:")
	fmt.Printf("1. Edit your shell configuration file:\n")
	fmt.Printf("   nano ~/.bashrc    # for bash\n")
	fmt.Printf("   nano ~/.zshrc     # for zsh\n")
	fmt.Printf("   nano ~/.profile   # for other shells\n")
	fmt.Printf("\n2. Add this line at the end of the file:\n")
	fmt.Printf("   export PATH=\"$PATH:%s\"\n", toolsDir)
	fmt.Printf("\n3. Save and reload:\n")
	fmt.Printf("   source ~/.bashrc    # or appropriate config file\n")
	fmt.Printf("\n4. Verify:\n")
	fmt.Printf("   echo $PATH | grep %s\n", toolsDir)
}

func (e *Environment) printWindowsInstructions(toolsDir string) {
	fmt.Println("For Windows:")
	fmt.Printf("1. Open Settings → System → About → Advanced system settings\n")
	fmt.Printf("2. Click 'Environment Variables...'\n")
	fmt.Printf("3. In 'User variables', select 'Path' and click 'Edit...'\n")
	fmt.Printf("4. Click 'New' and add: %s\n", toolsDir)
	fmt.Printf("5. Click 'OK' to save all dialogs\n")
	fmt.Printf("6. Restart your command prompt/PowerShell\n")
	fmt.Printf("\nAlternatively, using PowerShell (as Administrator):\n")
	fmt.Printf("   $env:PATH += \";%s\"\n", toolsDir)
	fmt.Printf("   [Environment]::SetEnvironmentVariable(\"PATH\", $env:PATH, \"User\")\n")
	fmt.Printf("\nAs a last resort, you can run tools via their full path, e.g.:\n")
	fmt.Printf("   \"%s\\<tool>.exe\" --help\n", toolsDir)
}

func (e *Environment) printGenericUnixInstructions(toolsDir string) {
	fmt.Printf("Add this line to your shell configuration file (~/.bashrc, ~/.zshrc, etc.):\n")
	fmt.Printf("   export PATH=\"$PATH:%s\"\n", toolsDir)
	fmt.Printf("\nThen reload your shell configuration:\n")
	fmt.Printf("   source ~/.bashrc    # or your shell's config file\n")
}

