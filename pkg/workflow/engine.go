package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"amo/pkg/filesystem"
	"amo/pkg/network"

	"github.com/dop251/goja"
)

// AssetReader interface for reading assets
type AssetReader interface {
	ReadFileAsString(path string) (string, error)
	Exists(path string) bool
	GetWorkflowFileNames() ([]string, error)
}

type Engine struct {
	vm               *goja.Runtime
	vars             map[string]string
	context          context.Context
	filesystem       *filesystem.FileSystem
	assetReader      AssetReader
	network          *network.NetworkClient
	toolPathProvider ToolPathProvider
}

func NewEngine(ctx context.Context) *Engine {
	if ctx == nil {
		ctx = context.Background()
	}
	fs := filesystem.NewFileSystem()

	networkClient, err := network.NewNetworkClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize network client: %v\n", err)
		networkClient = nil
	}

	engine := &Engine{
		vm:          nil,
		vars:        make(map[string]string),
		context:     ctx,
		filesystem:  fs,
		assetReader: nil,
		network:     networkClient,
	}

	return engine
}

func (e *Engine) SetAssetReader(reader AssetReader) {
	e.assetReader = reader
}

func (e *Engine) SetVars(vars map[string]string) {
	e.vars = vars
}

func (e *Engine) RunWorkflow(scriptPath string) error {
	baseCtx := e.context
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(baseCtx)
	defer cancel()

	vm := goja.New()
	e.vm = vm
	e.registerAPIs()

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt(ctx.Err())
		case <-done:
		}
	}()

	script, err := e.loadScript(scriptPath)
	if err != nil {
		if e.shouldTryJsExtension(scriptPath, err) {
			altPath := scriptPath + ".js"
			script, err = e.loadScript(altPath)
			if err != nil {
				close(done)
				return err
			}
			scriptPath = altPath
		} else {
			close(done)
			return err
		}
	}

	err = e.executeScript(script, scriptPath)
	close(done)
	return err
}

func (e *Engine) loadScript(scriptPath string) (string, error) {
	// First priority: Try direct path (absolute or relative)
	if content, err := os.ReadFile(scriptPath); err == nil {
		return string(content), nil
	}

	// Check if this is a path with subdirectories
	hasSubdirectory := strings.Contains(scriptPath, string(filepath.Separator)) || strings.Contains(scriptPath, "/")

	// For simple filenames without separators, try all locations
	if !hasSubdirectory {
		// Second priority: Try user's configured workflow directory
		if configContent, err := e.tryConfiguredWorkflowPath(scriptPath); err == nil {
			return configContent, nil
		}

		// Third priority: Try default downloaded workflows directory
		if userWorkflowContent, err := e.tryUserWorkflowPath(scriptPath); err == nil {
			return userWorkflowContent, nil
		}

		// Fourth priority: Try embedded assets
		if e.assetReader != nil {
			normalizedPath := filepath.ToSlash(scriptPath)
			if e.shouldTryEmbeddedAsset(normalizedPath) && e.assetReader.Exists(scriptPath) {
				return e.assetReader.ReadFileAsString(scriptPath)
			}
		}
	} else {
		// For paths with subdirectories, check in workflow directories

		// Second priority: Try user's configured workflow directory with full subpath
		if configContent, err := e.tryConfiguredWorkflowSubpath(scriptPath); err == nil {
			return configContent, nil
		}

		// Third priority: Try default downloaded workflows directory with full subpath
		if userWorkflowContent, err := e.tryUserWorkflowSubpath(scriptPath); err == nil {
			return userWorkflowContent, nil
		}

		// Fourth priority: Try embedded assets with normalized path
		if e.assetReader != nil {
			normalizedPath := filepath.ToSlash(scriptPath)
			if e.assetReader.Exists(normalizedPath) {
				return e.assetReader.ReadFileAsString(normalizedPath)
			}
		}
	}

	return "", fmt.Errorf("script not found: %s", scriptPath)
}

func (e *Engine) isScriptNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "script not found:")
}

func (e *Engine) shouldTryJsExtension(scriptPath string, err error) bool {
	if !e.isScriptNotFoundError(err) {
		return false
	}

	if strings.Contains(scriptPath, "/") || strings.Contains(scriptPath, string(filepath.Separator)) {
		return false
	}

	if filepath.Ext(scriptPath) != "" {
		return false
	}

	return true
}

// tryConfiguredWorkflowPath attempts to load script from the user's configured workflow directory
func (e *Engine) tryConfiguredWorkflowPath(filename string) (string, error) {
	// Import the config package dynamically to avoid circular import
	configManager, err := createConfigManager()
	if err != nil {
		return "", err
	}

	// Get configured workflows directory
	workflowsDir := configManager.GetWorkflowsDir()
	if workflowsDir == "" {
		return "", fmt.Errorf("no configured workflow directory")
	}

	workflowPath := filepath.Join(workflowsDir, filename)

	if content, err := os.ReadFile(workflowPath); err == nil {
		return string(content), nil
	}

	return "", fmt.Errorf("script not found in configured workflow directory: %s", filename)
}

// tryConfiguredWorkflowSubpath attempts to load script from subdirectories in the user's configured workflow directory
func (e *Engine) tryConfiguredWorkflowSubpath(relPath string) (string, error) {
	// Import the config package dynamically to avoid circular import
	configManager, err := createConfigManager()
	if err != nil {
		return "", err
	}

	// Get configured workflows directory
	workflowsDir := configManager.GetWorkflowsDir()
	if workflowsDir == "" {
		return "", fmt.Errorf("no configured workflow directory")
	}

	// Normalize the path to use OS-specific separators
	normalizedPath := filepath.FromSlash(relPath)
	workflowPath := filepath.Join(workflowsDir, normalizedPath)

	if content, err := os.ReadFile(workflowPath); err == nil {
		return string(content), nil
	}

	return "", fmt.Errorf("script not found in configured workflow directory: %s", relPath)
}

// workflowDirProvider is a helper struct that provides workflow directories
// while avoiding circular imports with the config package
type workflowDirProvider struct {
	downloader *WorkflowDownloader
}

// GetWorkflowsDir returns the configured workflow directory or falls back to default
func (wp *workflowDirProvider) GetWorkflowsDir() string {
	// First get the configured directory
	configuredDir := wp.downloader.GetConfiguredWorkflowsDir()
	if configuredDir != "" {
		return configuredDir
	}

	// Fall back to default directory if no custom directory is configured
	return wp.downloader.GetWorkflowsDir()
}

// createConfigManager creates a config manager instance without direct import
// This avoids circular imports between workflow and config packages
func createConfigManager() (interface{ GetWorkflowsDir() string }, error) {
	// Since we can't directly import config package due to circular references,
	// we'll create a stub that directly reads from the config file
	downloader, err := NewWorkflowDownloader()
	if err != nil {
		return nil, err
	}

	return &workflowDirProvider{downloader: downloader}, nil
}

// tryUserWorkflowPath attempts to load script from user downloads workflows directory
func (e *Engine) tryUserWorkflowPath(filename string) (string, error) {
	downloader, err := NewWorkflowDownloader()
	if err != nil {
		return "", err
	}

	userWorkflowPath := downloader.env.GetCrossPlatformUtils().JoinPath(downloader.GetWorkflowsDir(), filename)

	if content, err := os.ReadFile(userWorkflowPath); err == nil {
		return string(content), nil
	}

	return "", fmt.Errorf("user workflow not found: %s", filename)
}

// tryUserWorkflowSubpath attempts to load script from subdirectories in the user downloads workflows directory
func (e *Engine) tryUserWorkflowSubpath(relPath string) (string, error) {
	downloader, err := NewWorkflowDownloader()
	if err != nil {
		return "", err
	}

	// Normalize the path to use OS-specific separators
	normalizedPath := filepath.FromSlash(relPath)
	userWorkflowPath := downloader.env.GetCrossPlatformUtils().JoinPath(downloader.GetWorkflowsDir(), normalizedPath)

	if content, err := os.ReadFile(userWorkflowPath); err == nil {
		return string(content), nil
	}

	return "", fmt.Errorf("user workflow not found: %s", relPath)
}

// shouldTryEmbeddedAsset determines if we should try loading from embedded assets
func (e *Engine) shouldTryEmbeddedAsset(path string) bool {
	return !strings.Contains(path, "/") ||
		strings.HasPrefix(path, "workflow/") ||
		strings.HasPrefix(path, "tools/")
}

// executeScript executes a workflow script
func (e *Engine) executeScript(script, scriptPath string) error {
	if !strings.HasPrefix(strings.TrimSpace(script), "//!amo") {
		return fmt.Errorf("invalid amo workflow: %s (must start with //!amo)", scriptPath)
	}

	_, err := e.vm.RunString(script)
	if err == nil {
		return nil
	}

	if exception, ok := err.(*goja.Exception); ok {
		return fmt.Errorf("execution failed for %s: %s", scriptPath, exception.String())
	}

	return fmt.Errorf("execution failed for %s: %w", scriptPath, err)
}

// registerAPIs registers all JavaScript APIs
func (e *Engine) registerAPIs() {
	// Register modular APIs
	e.registerCoreAPI()
	e.registerFileSystemAPI()
	e.registerNetworkAPI()
	e.registerEncodingAPI()
	e.registerClipboardAPI()
}
