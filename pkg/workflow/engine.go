package workflow

import (
	"context"
	"fmt"
	"io/ioutil"
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

// Engine represents the workflow execution engine
type Engine struct {
	vm          *goja.Runtime
	vars        map[string]string
	context     context.Context
	filesystem  *filesystem.FileSystem
	assetReader AssetReader
	network     *network.NetworkClient
}

// NewEngine creates a new workflow engine
func NewEngine(ctx context.Context) *Engine {
	vm := goja.New()
	fs := filesystem.NewFileSystem()

	// Initialize network client
	networkClient, err := network.NewNetworkClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize network client: %v\n", err)
		networkClient = nil
	}

	engine := &Engine{
		vm:          vm,
		vars:        make(map[string]string),
		context:     ctx,
		filesystem:  fs,
		assetReader: nil,
		network:     networkClient,
	}

	// Register all APIs
	engine.registerAPIs()

	return engine
}

// SetAssetReader sets the asset reader for embedded resources
func (e *Engine) SetAssetReader(reader AssetReader) {
	e.assetReader = reader
}

// SetVars sets runtime variables
func (e *Engine) SetVars(vars map[string]string) {
	e.vars = vars
}

// RunWorkflow executes a workflow script
func (e *Engine) RunWorkflow(scriptPath string) error {
	script, err := e.loadScript(scriptPath)
	if err != nil {
		return err
	}

	return e.executeScript(script, scriptPath)
}

// loadScript loads script with enhanced priority:
// 1. Direct path
// 2. User's configured workflow directory
// 3. Default downloaded workflow directory
// 4. Embedded assets
func (e *Engine) loadScript(scriptPath string) (string, error) {
	// First priority: Try direct path (absolute or relative)
	if content, err := ioutil.ReadFile(scriptPath); err == nil {
		return string(content), nil
	}

	// If the path doesn't contain a separator, it might be just a filename
	// Try to find it in various locations
	if !strings.Contains(scriptPath, string(filepath.Separator)) && !strings.Contains(scriptPath, "/") {
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
	}

	return "", fmt.Errorf("script not found: %s", scriptPath)
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

	if content, err := ioutil.ReadFile(workflowPath); err == nil {
		return string(content), nil
	}

	return "", fmt.Errorf("script not found in configured workflow directory: %s", filename)
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

	if content, err := ioutil.ReadFile(userWorkflowPath); err == nil {
		return string(content), nil
	}

	return "", fmt.Errorf("user workflow not found: %s", filename)
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
	if err != nil {
		return fmt.Errorf("execution failed for %s: %w", scriptPath, err)
	}

	return nil
}

// registerAPIs registers all JavaScript APIs
func (e *Engine) registerAPIs() {
	// Register modular APIs
	e.registerCoreAPI()
	e.registerFileSystemAPI()
	e.registerNetworkAPI()
}
