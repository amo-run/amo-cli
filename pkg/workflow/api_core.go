package workflow

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"amo/pkg/env"
)

// Core API functions (getVar, cliCommand, console)

func (e *Engine) getVar(key string) string {
	return e.vars[key]
}

func (e *Engine) getRegion() string {
	environment, err := env.NewEnvironment()
	if err != nil {
		return "global"
	}
	return environment.DetectRegion()
}

func (e *Engine) getOS() string {
	environment, err := env.NewEnvironment()
	if err != nil {
		return "unknown"
	}
	return environment.GetOperatingSystem()
}

func (e *Engine) consoleLog(args ...interface{}) {
	fmt.Println(args...)
}

func (e *Engine) consoleError(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
}

func (e *Engine) consoleWarn(args ...interface{}) {
	fmt.Fprint(os.Stderr, "WARNING: ")
	fmt.Fprintln(os.Stderr, args...)
}

func (e *Engine) cliCommand(name string, args []string, opts map[string]interface{}) map[string]interface{} {
	// Security check - CLI command whitelist
	environment, err := env.NewEnvironment()
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to initialize environment for security check: %v", err),
		}
	}

	// Extract base command name from path for whitelist validation
	baseName := filepath.Base(name)
	allowed, err := environment.IsCommandAllowed(baseName)
	if err != nil || !allowed {
		return map[string]interface{}{
			"error": fmt.Sprintf("command '%s' (base: '%s') is not in the allowed CLI commands list", name, baseName),
		}
	}

	// Parse options
	timeout := 3600 // default timeout in seconds
	var workingDir string
	var envVars []string
	interactive := false
	var stdin string

	if opts != nil {
		if t, ok := opts["timeout"].(int); ok {
			timeout = t
		}
		if t, ok := opts["timeout"].(float64); ok {
			timeout = int(t)
		}
		if wd, ok := opts["cwd"].(string); ok {
			workingDir = wd
		}
		if env, ok := opts["env"].(map[string]interface{}); ok {
			for k, v := range env {
				if vStr, ok := v.(string); ok {
					envVars = append(envVars, fmt.Sprintf("%s=%s", k, vStr))
				}
			}
		}
		if inter, ok := opts["interactive"].(bool); ok {
			interactive = inter
		}
		if s, ok := opts["stdin"].(string); ok {
			stdin = s
		}
	}

	// Get the actual command path - try direct execution first, then tool cache
	commandPath := e.resolveCommandPath(name)

	// Create command with independent timeout context
	// Note: Use context.Background() to ensure cliCommand timeout is independent
	// of the workflow-level timeout, allowing individual commands to have their own timeout limits
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, commandPath, args...)

	// Set working directory if specified
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Set environment variables
	if len(envVars) > 0 {
		cmd.Env = append(os.Environ(), envVars...)
	}

	// Handle stdin if provided and not in interactive mode
	if stdin != "" && !interactive {
		cmd.Stdin = strings.NewReader(stdin)
	}

	// Handle interactive mode
	if interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			return map[string]interface{}{
				"error": err.Error(),
			}
		}

		return map[string]interface{}{
			"stdout": "",
			"stderr": "",
		}
	}

	// Execute command and capture output separately for stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := map[string]interface{}{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			result["error"] = fmt.Sprintf("command timed out after %d seconds", timeout)
		} else {
			result["error"] = err.Error()
		}
	}

	return result
}

// resolveCommandPath attempts to resolve command path using the following priority:
// 1. Try direct execution (exec.LookPath)
// 2. Try tool path cache lookup if direct execution fails
// 3. Return original command name if all fail
func (e *Engine) resolveCommandPath(commandName string) string {
	// First try direct execution using system PATH
	if path, err := exec.LookPath(commandName); err == nil {
		return path
	}

	// If direct execution fails, try tool path cache
	if cachedPath := e.getToolPathFromCache(commandName); cachedPath != "" {
		// Validate cached path still exists
		if _, err := os.Stat(cachedPath); err == nil {
			return cachedPath
		}
	}

	// Return original command name as fallback
	return commandName
}

// ToolPathProvider interface for getting cached tool paths without importing tool package
type ToolPathProvider interface {
	GetCachedToolPath(commandName string) (string, bool)
}

// SetToolPathProvider sets the tool path provider for the engine
func (e *Engine) SetToolPathProvider(provider ToolPathProvider) {
	e.toolPathProvider = provider
}

// getToolPathFromCache attempts to get tool path from the tool cache
func (e *Engine) getToolPathFromCache(commandName string) string {
	if e.toolPathProvider != nil {
		if cachedPath, exists := e.toolPathProvider.GetCachedToolPath(commandName); exists {
			return cachedPath
		}
	}
	return ""
}

// checkFileOperationSecurity validates file paths for security
func (e *Engine) checkFileOperationSecurity(path string) error {
	// Clean the path
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal not allowed: %s", path)
	}

	// Additional security checks can be added here
	// For example, restricting access to certain directories

	return nil
}

// getCurrentUser returns the current username
func getCurrentUser() string {
	if currentUser, err := user.Current(); err == nil {
		return currentUser.Username
	}
	return "unknown"
}

func (e *Engine) registerCoreAPI() {
	e.vm.Set("getVar", e.getVar)
	e.vm.Set("getRegion", e.getRegion)
	e.vm.Set("getOS", e.getOS)
	e.vm.Set("cliCommand", e.cliCommand)

	e.vm.Set("console", map[string]interface{}{
		"log":   e.consoleLog,
		"error": e.consoleError,
		"warn":  e.consoleWarn,
	})
}
