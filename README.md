# Amo

A CLI tool for managing tools and running JavaScript-based workflows with native system integration.

## ✨ Features

- **JavaScript Workflows**: Execute workflows written in JavaScript with full system access
- **Native API Integration**: File system, command execution, network operations, and cross-platform utilities
- **Embedded Workflows**: Pre-built workflows for common tasks (file organization, API demos, etc.)
- **Workflow Management**: Download and manage workflows from remote sources (GitHub, GitLab, etc.)
- **Cross-Platform**: Native support for Windows, macOS, and Linux with path normalization
- **Tool Management**: Integrated tool detection, installation, and path caching system

## 📚 Documentation

- **🚀 [Quick Start](QUICKSTART.md)** - Installation and basic usage
- **🔧 [Development](DEVELOPMENT.md)** - Architecture and development guide
- **🏷️ [Versioning](VERSIONING.md)** - Version management and build system
- **💻 [Workflow Development](WORKFLOW-DEVELOPMENT.md)** - Creating workflows with IDE autocompletion

## 🛠️ Installation

### Quick Install

```bash
# Download pre-built binary from releases
# Or build from source:

git clone <repository-url>
cd amo-cli
go build -o amo .

# Install to $HOME/go/bin
./install-bin.sh
```

## 🔧 Basic Usage

```bash
# List available workflows (embedded + user)
amo workflow list

# Run embedded workflow
amo run file-organizer.js --var source_dir=/Downloads --var target_dir=/Organized

# Run external workflow file
amo run my-workflow.js --var input=/path/to/input --var output=/path/to/output

# Debug mode
amo run workflow.js --debug

# With timeout limit (in seconds)
amo run workflow.js --timeout 3600

# Tool management
amo tool list                    # List all supported tools
amo tool install pandoc         # Install tool automatically (no timeout)
amo tool cache info             # View tool path cache info

# Version info
amo --version    # Quick version
amo version      # Detailed build info

# Configuration management
amo config ls                    # List all configuration settings
amo config workflows /path/to/dir # Set custom workflows directory
amo config rm workflows          # Reset to default value
```

## ⚙️ Configuration

### Security Whitelist

Amo uses a whitelist to ensure workflows can only run approved commands.

```bash
# Manage allowed commands through CLI
amo tool permission list              # List allowed commands  
amo tool permission add ffmpeg        # Add command to whitelist
amo tool permission remove ffmpeg       # Remove command from whitelist

# Or edit directly with any text editor, e.g.
vim ~/.amo/allowed_cli.txt
```

### Tool Path Cache

Amo automatically caches discovered tool paths for better performance.

```bash
# View cache information
amo tool cache info

# Clear cache to force re-detection
amo tool cache clear

# Cache file location: ~/.amo/tool_paths.json
```

### Workflow Management

```bash
# List all workflows (embedded + user downloaded)
amo workflow list

# Download workflow from remote source
amo workflow get https://github.com/user/repo/blob/main/workflow.js

# Download with custom filename
amo workflow get https://raw.githubusercontent.com/user/repo/main/workflow.js --filename my-workflow.js

# Supported domains: GitHub, GitLab, Bitbucket, SourceForge
```

### Runtime Variables

```bash
# Pass variables to workflows
amo run workflow.js --var key1=value1 --var key2=value2

# Common variable shortcuts
amo run workflow.js --input /path/to/input --output /path/to/output

# Environment variables
VARIABLE=value amo run workflow.js

# Show workflow help (if supported)
amo run workflow.js --workflow-help
```

### Configuration Settings

Amo stores user configuration in `~/.amo/config.yaml` which can be managed through the CLI.

```bash
# List all configuration settings
amo config ls

# Get a specific configuration value
amo config workflows

# Set a configuration value
amo config workflows ~/custom/workflows/dir

# Remove a configuration value (restore default)
amo config rm workflows

# Currently supported configuration keys:
# - workflows: Directory path for custom workflows
```

## 📁 Embedded Workflows

### File Organization

```bash
# Organize files by extension with many options
amo run file-organizer.js \
  --var source_dir=/Downloads \
  --var target_dir=/Organized \
  --var dry_run=true \
  --var copy=true \
  --var include_hidden=true
```

### System Utilities

```bash
# File system API demonstration with fs.xxx syntax
amo run fs-api-demo.js --var cleanup=true
```

## 🔧 Writing Workflows

### Basic Structure

```javascript
//!amo

function main() {
    console.log("🎯 My Workflow");
    
    // Get runtime variables
    var input = getVar("input") || "";
    if (!input) {
        console.error("❌ Error: input is required");
        return false;
    }
    
    // File operations using new fs API
    if (!fs.exists(input)) {
        console.error("❌ Input path does not exist:", input);
        return false;
    }
    
    // Process files
    var files = fs.readdir(input);
    if (!files.success) {
        console.error("❌ Failed to read directory:", files.error);
        return false;
    }
    
    console.log("📁 Found", files.files.length, "files");
    
    return true;
}

main();
```

### API Reference

```javascript
// File System Operations (new fs.xxx syntax)
fs.exists(path)           // Check if file/directory exists
fs.isFile(path)          // Check if path is a file
fs.isDir(path)           // Check if path is a directory
fs.read(path)            // Read file content
fs.write(path, content)  // Write file content
fs.copy(src, dst)        // Copy file/directory
fs.move(src, dst)        // Move file/directory
fs.readdir(path)         // List directory contents
fs.mkdir(path)           // Create directory
fs.remove(path)          // Delete file/directory

// Path Operations
fs.join([...paths])      // Join path components
fs.dirname(path)         // Get directory name
fs.basename(path)        // Get base name without extension
fs.ext(path)             // Get file extension
fs.split(path)           // Split path into {dir, file}
fs.absolute(path)        // Get absolute path
fs.relative(base, target) // Get relative path

// Network Operations
http.get(url, headers)                    // HTTP GET request
http.post(url, body, headers)             // HTTP POST request
http.getJSON(url, headers)                // GET with JSON parsing
http.downloadFile(url, path, options)     // Download file with progress

// System Commands (whitelisted only)
cliCommand("command", ["arg1", "arg2"], {
    timeout: 3600,         // seconds (default: no timeout in workflows)
    cwd: "/path/to/dir",  // working directory
    env: {"VAR": "value"} // environment variables
});

// Runtime Variables
getVar("variable_name")  // Get runtime variable

// Console Output
console.log("message")
console.error("error")
console.warn("warning")
```

### Advanced Features

```javascript
//!amo

function main() {
    // Network operations
    var response = http.get("https://api.example.com/data");
    if (response.status_code === 200) {
        console.log("API response:", response.body);
    }
    
    // JSON responses
    var jsonData = http.getJSON("https://api.example.com/json");
    if (jsonData.data) {
        console.log("Parsed JSON:", jsonData.data);
    }
    
    // File downloads with progress
    var download = http.downloadFile(
        "https://example.com/file.zip",
        "./downloads/file.zip", 
        { show_progress: true }
    );
    
    // File system operations with error handling
    var files = fs.find("./", "*.txt");
    if (files.success) {
        files.files.forEach(function(filePath) {
            console.log("Found text file:", fs.basename(filePath));
        });
    }
    
    return true;
}

main();
```

## 💡 Key Concepts

### Security Model

Amo implements a comprehensive security model:

- **CLI Commands**: Only explicitly allowed commands can be executed
- **Path Validation**: All file operations are validated for security
- **Timeout Protection**: Commands have configurable timeouts
- **Network Security**: Controlled domain access for downloads
- **Configuration**: Security settings stored in `~/.amo/allowed_cli.txt`

### Workflow Loading Priority

Workflow loading follows this priority:

1. **User workflows**: `~/.amo/workflows/` (highest priority)
2. **External file paths**: Full/relative file system paths  
3. **Embedded workflows**: Pre-built workflows included with the binary
4. **Error**: If no source is available

### Cross-Platform Support

Amo handles platform differences automatically:

- **Path separators**: Automatic normalization (`/` vs `\`)
- **Executable extensions**: Automatic `.exe` handling on Windows
- **File permissions**: Platform-appropriate permission handling
- **Environment variables**: Case-insensitive on Windows
- **Tool paths**: Automatic tool discovery with caching

## 🚨 Common Issues

**"Command not in whitelist"**: Add the command using `amo tool permission add <command>`

**Workflow not found**: Use `amo workflow list` to see available workflows, or provide full path to external files

**Permission errors**: Ensure amo binary has execute permissions (`chmod +x amo`)

**JavaScript errors**: Use `--debug` flag for detailed execution information

**Path issues**: Use absolute paths when possible, or the `fs.join()` function for cross-platform compatibility

**Download failed**: Ensure the URL is from an allowed domain (GitHub, GitLab, Bitbucket, SourceForge)

**Tool not found**: Use `amo tool list` to check tool status and `amo tool install <tool>` to install

## 📄 License

[MIT License](LICENSE)