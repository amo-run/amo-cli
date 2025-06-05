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

// loadScript loads script with priority: user workflows dir > filesystem > embedded assets
func (e *Engine) loadScript(scriptPath string) (string, error) {
	// First priority: Check user workflows directory if scriptPath is just a filename
	if !strings.Contains(scriptPath, string(filepath.Separator)) && !strings.Contains(scriptPath, "/") {
		userWorkflowPath, err := e.tryUserWorkflowPath(scriptPath)
		if err == nil {
			return userWorkflowPath, nil
		}
	}

	// Second priority: Try absolute/relative filesystem path
	if content, err := ioutil.ReadFile(scriptPath); err == nil {
		return string(content), nil
	}

	// Third priority: Try embedded assets
	if e.assetReader != nil {
		normalizedPath := filepath.ToSlash(scriptPath)
		if e.shouldTryEmbeddedAsset(normalizedPath) && e.assetReader.Exists(scriptPath) {
			return e.assetReader.ReadFileAsString(scriptPath)
		}
	}

	return "", fmt.Errorf("script not found: %s", scriptPath)
}

// tryUserWorkflowPath attempts to load script from user workflows directory
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
	// Core functions
	e.vm.Set("cliCommand", e.cliCommand)
	e.vm.Set("getVar", e.getVar)

	// Console
	e.vm.Set("console", map[string]interface{}{
		"log":   e.consoleLog,
		"error": e.consoleError,
		"warn":  e.consoleWarn,
	})

	// Register modular APIs
	e.registerFileSystemAPI()
	e.registerNetworkAPI()
}
