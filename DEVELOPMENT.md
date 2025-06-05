# Development Guide

Development information for the Amo Workflow Engine project.

## 📚 Documentation Links

- **🏷️ [Version Management](VERSIONING.md)** - Build system, versioning, and release process
- **🚀 [Quick Start](QUICKSTART.md)** - Installation and basic usage  
- **📖 [User Guide](README.md)** - Complete usage documentation
- **💻 [Workflow Development](WORKFLOW-DEVELOPMENT.md)** - Creating workflows with IDE support

## 🏗️ Architecture Overview

The project follows clean architecture with modular design and clear separation of concerns.

### Project Structure

```
amo-cli/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command with subcommand structure
│   ├── run.go             # Workflow execution command (amo run)
│   ├── tool.go            # Tool management commands (Go-based)
│   ├── version.go         # Version display with build-time injection
│   └── workflow.go        # Workflow listing and download commands
├── pkg/
│   ├── cli/               # CLI parameter parsing
│   ├── env/               # Environment and cross-platform utilities
│   ├── filesystem/        # File system operations wrapper
│   ├── network/           # HTTP/Network operations
│   ├── tool/              # Tool management (Go implementation)
│   └── workflow/          # JavaScript workflow execution engine + downloader
├── assets/
│   ├── tools.json         # Tool configuration data
│   └── workflow/          # Embedded workflow script examples
├── main.go                # Entry point with version injection
├── assets.go              # Embedded asset management (limited scope)
├── build.sh               # Build script (see VERSIONING.md)
└── amo-workflow.d.ts      # TypeScript definitions for IDE support
```

## 🎯 Key Design Patterns

### 1. Command Structure (Cobra)

```go
type RootCmd struct {
    Use:   "amo",
    Short: "A CLI tool for managing tools and running JavaScript-based workflows",
    Subcommands: [
        NewRunCmd(),        // amo run <workflow>
        NewWorkflowCmd(),   // amo workflow list/get
        NewToolCmd(),       // amo tool list/install/permission/cache
        NewVersionCmd(),    // amo version
    ]
}
```

### 2. JavaScript Runtime Engine (Goja)

```go
type Engine struct {
    vm          *goja.Runtime
    vars        map[string]string
    context     context.Context
    filesystem  *filesystem.FileSystem
    assetReader AssetReader
    network     *network.NetworkClient
}

func (e *Engine) registerAPIs() {
    // File system operations with fs.xxx syntax
    e.registerFileSystemAPI()
    // Network operations (http.get, http.post, etc.)
    e.registerNetworkAPI()
    // Core functions (getVar, cliCommand, console)
    e.registerCoreAPI()
}
```

### 3. Tool Management (Go Implementation)

```go
type Manager struct {
    config      *ToolConfig
    environment *env.Environment
    pathCache   *ToolPathCache
}

func (m *Manager) CheckTool(toolName string) (*ToolStatus, error) {
    // Security check via environment.IsCommandAllowed()
    allowed, err := m.environment.IsCommandAllowed(tool.Check.Command)
    if err != nil || !allowed {
        return nil, fmt.Errorf("command not allowed")
    }
    
    // Use cached path if available, otherwise discover and cache
    command := m.findToolExecutable(tool)
    
    // Execute version check
    cmd := exec.CommandContext(ctx, command, args...)
    output, err := cmd.CombinedOutput()
    
    // Parse version using regex patterns and save path cache
    return &ToolStatus{
        Name:      tool.Name,
        Installed: true,
        Version:   extractedVersion,
    }, nil
}
```

**Tool Path Caching System:**

- **Cache File**: `~/.amo/tool_paths.json` stores discovered tool executable paths
- **Performance**: Avoids repeated PATH searches and file system checks
- **Reliability**: Validates cached paths and removes invalid entries automatically
- **User Control**: Users can manually edit cache file to specify custom tool locations
- **Management Commands**: `amo tool cache info` and `amo tool cache clear` for cache management

### 4. Workflow Engine (`pkg/workflow/`)

**JavaScript execution with native API bindings:**

```go
func (e *Engine) registerFileSystemAPI() {
    e.vm.Set("fs", map[string]interface{}{
        // File/Directory checks
        "exists": e.exists,
        "isFile": e.isFile,
        "isDir":  e.isDir,
        
        // File operations
        "read":     e.readFile,
        "write":    e.writeFile,
        "copy":     e.copyFile,
        "move":     e.moveFile,
        "remove":   e.deleteFile,
        
        // Directory operations
        "readdir":  e.listDir,
        "mkdir":    e.makeDir,
        
        // Path operations
        "join":     e.joinPath,
        "split":    e.splitPath,
        "dirname":  e.getDirName,
        "basename": e.getBaseName,
        "ext":      e.getExtension,
        
        // Utilities
        "find":     e.findFiles,
        "size":     e.getFileSize,
        "cwd":      e.getWorkingDir,
    })
}

func (e *Engine) registerNetworkAPI() {
    e.vm.Set("http", map[string]interface{}{
        "get":          e.httpGet,
        "post":         e.httpPost,
        "getJSON":      e.httpGetJSON,
        "downloadFile": e.httpDownloadFile,
    })
}
```

### 5. Workflow Download Management

```go
type WorkflowDownloader struct {
    env    *env.Environment
    client *http.Client
}

// Supported domains for workflow downloads
var AllowedDomains = []string{
    "github.com",
    "raw.githubusercontent.com", 
    "gitlab.com",
    "bitbucket.org",
    "sourceforge.net",
}

func (wd *WorkflowDownloader) DownloadWorkflow(urlStr string, filename string) error {
    // Security validation of URL
    // Auto-conversion to raw URLs (GitHub, GitLab)
    // Validation of workflow format (must start with //!amo)
    // Save to ~/.amo/workflows/
}
```

### 6. Cross-Platform Environment (`pkg/env/`)

**Platform-aware utilities and security:**

```go
func (e *Environment) IsCommandAllowed(command string) (bool, error) {
    allowedCommands, err := e.LoadAllowedCLICommands()
    if err != nil {
        return false, err
    }
    
    for _, allowedCmd := range allowedCommands {
        if allowedCmd == command {
            return true, nil
        }
    }
    
    return false, nil
}

func (e *Environment) EnsureAllowedCLIFile() error {
    // Creates ~/.amo/allowed_cli.txt with default tool commands
    // Includes helpful comments and examples
}
```

### 7. Asset Management (`assets.go`)

**Limited scope - only for core functionality:**

```go
func (e *Engine) loadScript(scriptPath string) (string, error) {
    // Priority order:
    // 1. User workflows directory (~/.amo/workflows/)
    // 2. External file paths (filesystem)
    // 3. Embedded assets (workflow scripts only)
    
    // First priority: Check user workflows directory
    if userWorkflowPath, err := e.tryUserWorkflowPath(scriptPath); err == nil {
        return userWorkflowPath, nil
    }
    
    // Second priority: Try filesystem
    if content, err := ioutil.ReadFile(scriptPath); err == nil {
        return string(content), nil
    }
    
    // Third priority: Try embedded assets
    if e.assetReader != nil && e.assetReader.Exists(scriptPath) {
        return e.assetReader.ReadFileAsString(scriptPath)
    }
    
    return "", fmt.Errorf("script not found: %s", scriptPath)
}
```

## 🚀 Adding New Features

### Adding a New Tool Configuration

1. **Update `assets/tools.json`**:

```json
{
  "new_tool": {
    "name": "New Tool",
    "description": "Tool description",
    "category": "category",
    "website": "https://example.com",
    "check": {
      "command": "newtool",
      "args": ["--version"],
      "pattern": "newtool version ([^\\s]+)"
    },
    "install": {
      "darwin": {
        "method": "homebrew",
        "package": "newtool"
      },
      "linux": {
        "method": "package",
        "packages": {
          "apt": "newtool",
          "yum": "newtool"
        }
      }
    }
  }
}
```

2. **Tool is automatically available** - no code changes needed.

### Adding a New JavaScript API

1. **Implement the Go function** in `pkg/workflow/api_*.go`:

```go
func (e *Engine) newAPIFunction(param string) map[string]interface{} {
    result, err := e.filesystem.SomeOperation(param)
    return e.createResult(err == nil, result, err)
}
```

2. **Register in the appropriate API file** (e.g., `api_filesystem.go`):

```go
func (e *Engine) registerFileSystemAPI() {
    e.vm.Set("fs", map[string]interface{}{
        // ... existing functions
        "newFunction": e.newAPIFunction,
    })
}
```

3. **Add TypeScript definitions** to `amo-workflow.d.ts`:

```typescript
declare const fs: {
  // ... existing functions
  newFunction(param: string): Amo.Result;
};
```

### Adding a New Embedded Workflow

1. **Create workflow file** in `assets/workflow/`:

```javascript
//!amo

function main() {
    console.log("🎯 New Workflow");
    
    var input = getVar("input") || "";
    if (!input) {
        console.error("❌ Error: input is required");
        return false;
    }
    
    // Workflow implementation using available APIs:
    // - fs.* for file operations
    // - http.* for network operations  
    // - cliCommand() for system commands
    // - console.* for output
    
    return true;
}

main();
```

2. **Test the workflow**:

```bash
amo run new-workflow.js --var input=/path/to/input
```

### Adding a New CLI Command

1. **Create command file** in `cmd/`:

```go
func NewMyCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "my-command",
        Short: "Description of my command",
        RunE:  runMyCommand,
    }
}

func runMyCommand(cmd *cobra.Command, args []string) error {
    // Command implementation
    return nil
}
```

2. **Register in root command** (`cmd/root.go`):

```go
func NewRootCmd() *cobra.Command {
    rootCmd := &cobra.Command{...}
    
    rootCmd.AddCommand(NewMyCmd())
    
    return rootCmd
}
```

## 🧪 Testing Strategy

### Key Test Areas

1. **Command Structure**: Test all CLI commands and subcommands
2. **Tool Management**: Test Go-based tool status checking and installation
3. **JavaScript Engine Integration**: Test API bindings and security
4. **Workflow Downloads**: Test remote workflow downloading and validation
5. **Cross-platform Compatibility**: Test file operations across platforms
6. **Security**: Test CLI command whitelist and permissions

### Example Test Structure

```go
func TestToolManager_CheckTool(t *testing.T) {
    manager, err := tool.NewManager()
    require.NoError(t, err)
    
    // Load test configuration
    config := []byte(`{"tools": {"test": {...}}}`)
    err = manager.LoadConfig(config)
    require.NoError(t, err)
    
    status, err := manager.CheckTool("test")
    assert.NoError(t, err)
    assert.NotNil(t, status)
}

func TestWorkflowEngine_FileSystemAPI(t *testing.T) {
    engine := workflow.NewEngine(context.Background())
    
    // Test fs.exists API
    result := engine.CallJavaScript("fs.exists('/tmp')")
    assert.True(t, result.(bool))
}
```

## 🔍 Code Quality Standards

### Project-Specific Guidelines

1. **Security First**: 
   - Workflows cannot access embedded assets directly
   - Tool management uses native Go for security
   - CLI commands must be whitelisted
   - Network operations limited to allowed domains
2. **Clear Separation**: Tools management in Go, workflow execution in JS
3. **Cross-platform**: Use `env.CrossPlatformUtils` for all path operations
4. **Asset Isolation**: Embedded assets only accessible from Go code
5. **Consistent APIs**: All JavaScript APIs return structured objects with success/error

### Critical Security Architecture

```go
// SECURITY: Tool management isolated from workflow execution
func (cmd *ToolCommand) Execute() {
    // Direct Go implementation - no JavaScript involved
    manager, err := tool.NewManager()
    manager.LoadConfig(embeddedAssets.ReadFile("tools.json"))
    manager.CheckTool(toolName)
}

// SECURITY: Workflows cannot access assets directly
func (e *Engine) registerAPIs() {
    e.registerFileSystemAPI()   // File operations only
    e.registerNetworkAPI()      // Network operations only
    e.registerCoreAPI()         // Core functions only
    // NO registerAssetAPI() call
}

// SECURITY: CLI command whitelist validation
func (e *Engine) cliCommand(name string, args []string, opts map[string]interface{}) {
    environment, err := env.NewEnvironment()
    allowed, err := environment.IsCommandAllowed(name)
    if err != nil || !allowed {
        return map[string]interface{}{
            "error": fmt.Sprintf("command '%s' is not in the allowed CLI commands list", name),
        }
    }
    // ... execute command
}
```

## 🔧 Development Environment

### Initial Setup

```bash
# Clone and setup
git clone <repository-url>
cd amo-cli
go mod download

# Build and test
go build -o amo .
go test ./...
```

### Development Workflow

For build commands and version management, see **[VERSIONING.md](VERSIONING.md)**:

```bash
# Quick development build (see VERSIONING.md for details)
./build.sh local

# Install for local testing  
./install-bin.sh
```

### Testing Different Components

```bash
# Test tool management
amo tool list                    # List all tools
amo tool install surya_ocr      # Install tool (no timeout restriction)
amo tool cache info             # View tool path cache

# Test workflow execution
amo run file-organizer.js --var source_dir=/path/to/source --debug

# Test workflow downloads
amo workflow get https://github.com/user/repo/blob/main/workflow.js

# Test permission management  
amo tool permission list
amo tool permission add ffmpeg
```

## 📋 Project-Specific Conventions

### Tool Configuration Format

- **Centralized**: All tools in `assets/tools.json`
- **Structured**: Clear separation of check/install methods
- **Platform-aware**: Different installation methods per OS
- **Secure**: Tool commands validated against whitelist

### JavaScript API Design

- **No Direct Asset Access**: Workflows cannot read embedded files
- **Consistent Returns**: All APIs return structured objects with success/error
- **Path Handling**: All path operations use cross-platform utilities
- **Security**: CLI commands require explicit whitelist approval
- **Aliases**: Multiple function names for convenience (e.g., `fs.read` and `fs.readFile`)

### Command Structure

- **Subcommands**: Clear separation of functionality (run, tool, workflow, version)
- **Consistent Flags**: Use `--var` for workflow variables, `--input`/`--output` shortcuts
- **Help Integration**: All commands have comprehensive help text
- **Error Handling**: Consistent error messaging and exit codes

### Security Model

- **Asset Isolation**: Only Go code can access embedded assets
- **Tool Security**: Native Go implementation for tool management
- **CLI Whitelist**: Commands must be explicitly allowed
- **Network Security**: Controlled domain access for downloads
- **Clear Boundaries**: Workflows for automation, Go for system management

## 📚 Key Interfaces and Dependencies

### Core Interfaces

- `cmd.NewRootCmd()`: Main CLI command structure
- `tool.Manager`: Native Go tool management
- `workflow.Engine`: JavaScript execution engine
- `workflow.WorkflowDownloader`: Remote workflow management
- `filesystem.FileSystem`: Cross-platform file operations wrapper
- `env.Environment`: Platform-aware environment and security utilities

### External Dependencies

- **Cobra**: CLI framework for command structure
- **Goja**: JavaScript runtime for Go
- **Embed**: Go 1.16+ embed for asset management (Go access only)

### Platform Considerations

The project uses comprehensive cross-platform support:

- **Windows**: Executable extensions, path separators, environment variables
- **macOS**: Application bundles, case-sensitive paths, special binary paths
- **Linux**: Package paths, permissions

All cross-platform logic is centralized in `pkg/env/crossplatform.go`. 