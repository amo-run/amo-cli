package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information variables - set by main.go
var (
	version   = "dev"
	gitCommit = "none"
	buildTime = "unknown"
	buildBy   = "unknown"
)

// SetVersionInfo sets the version information from main.go
func SetVersionInfo(v, commit, buildTimeParam, buildByParam string) {
	version = v
	gitCommit = commit
	buildTime = buildTimeParam
	buildBy = buildByParam
}

// GetVersionInfo returns the current version information
func GetVersionInfo() (string, string, string, string) {
	return version, gitCommit, buildTime, buildBy
}

// NewVersionCmd creates and returns the version command
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long: `Display version information for Amo Workflow Engine including:
- Application version
- Git commit hash
- Build time
- Build environment
- Go version and platform information`,
		Run: func(cmd *cobra.Command, args []string) {
			showVersionInfo()
		},
	}
}

// showVersionInfo displays comprehensive version information
func showVersionInfo() {
	fmt.Printf("🚀 Amo Workflow Engine\n")
	fmt.Printf("=======================\n\n")

	// Application information
	fmt.Printf("🔖 Version Information:\n")
	fmt.Printf("  Version:     %s\n", version)
	fmt.Printf("  Git Commit:  %s\n", gitCommit)
	fmt.Printf("  Build Time:  %s\n", buildTime)
	fmt.Printf("  Built By:    %s\n", buildBy)
	fmt.Printf("\n")

	// Runtime information
	fmt.Printf("⚙️ Runtime Information:\n")
	fmt.Printf("  Go Version:  %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Compiler:    %s\n", runtime.Compiler)
	fmt.Printf("\n")
}
