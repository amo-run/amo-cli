package cmd

import (
	"fmt"

	"amo/pkg/workflow"

	"github.com/spf13/cobra"
)

type ExitCodeError interface {
	error
	ExitCode() int
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *exitError) ExitCode() int {
	if e == nil {
		return 1
	}
	return e.code
}

const (
	ExitCodeInfraError   = 1
	ExitCodeRuntimeError = 2
	ExitCodeUserError    = 3
)

func newInfraError(err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: ExitCodeInfraError, err: err}
}

func newRuntimeError(err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: ExitCodeRuntimeError, err: err}
}

func newUserError(message string, args ...interface{}) error {
	return &exitError{
		code: ExitCodeUserError,
		err:  fmt.Errorf(message, args...),
	}
}

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
