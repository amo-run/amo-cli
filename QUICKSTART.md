# Quick Start Guide

Get started with Amo Workflow Engine in 3 minutes.

## 🚀 Quick Install

Download the binary from [Releases](../../releases) or build with `go build`.

```bash
# Build from source
git clone <repository-url>
cd amo-cli
go build -o amo .

# Or install to $HOME/go/bin
./install-bin.sh
```

## 📖 Basic Usage

### Simple Workflow Execution

```bash
# Run embedded workflow
amo run file-organizer.js

# Run with variables
amo run file-organizer.js --var source_dir=/path/to/messy --var target_dir=/path/to/organized

# Run external workflow file
amo run my-workflow.js --var input=/path/to/input --var output=/path/to/output

# List available workflows
amo workflow list

# Debug mode
amo run workflow.js --debug

# With timeout limit (in seconds)
amo run workflow.js --timeout 3600
```

### Workflow Examples

```bash
# File organization
amo run file-organizer.js \
  --var source_dir=/Downloads \
  --var target_dir=/Organized \
  --var dry_run=true

# File system API demonstration
amo run fs-api-demo.js --var cleanup=true

# Using convenience flags for common variables
amo run my-workflow.js --input /path/to/input --output /path/to/output

# Environment variables (accessible via getVar())
API_KEY=your_key amo run workflow.js --var debug=true
```

## ⚙️ Configuration

### Security Whitelist

On first run, configure allowed CLI commands:

```bash
# Edit allowed commands list
vim ~/.amo/allowed_cli.txt

# Or manage through CLI
amo tool permission list         # List allowed commands
amo tool permission add ffmpeg   # Add command to whitelist
amo tool permission remove echo  # Remove command from whitelist
```

Example allowed commands:
```
echo
ffmpeg
magick
pandoc
doc-to-text
surya_ocr
```

### Tool Management

Check and manage external tools:

```bash
# List all supported tools and their status
amo tool list

# Install tool automatically (no timeout restriction)
amo tool install doc-to-text

# View tool path cache
amo tool cache info

# Clear path cache (force re-detection)
amo tool cache clear

# Manage CLI command permissions
amo tool permission list

# PATH management (NEW!)
amo tool path info              # Show current PATH configuration
amo tool path setup             # Add tools directory to system PATH
```

### PATH Configuration (NEW!)

Amo automatically tries to add the tools directory (`~/.amo/tools`) to your system PATH when you install tools. This allows you to run installed tools directly from the command line without specifying the full path.

```bash
# Check if tools directory is in PATH
amo tool path info

# Manually setup PATH (if automatic setup failed)
amo tool path setup

# After PATH setup, you can run tools directly:
ffmpeg -version                 # Instead of ~/.amo/tools/ffmpeg -version
surya_ocr --help               # Instead of ~/.amo/tools/surya_ocr --help
```

**Platform-specific notes:**
- **macOS**: Automatically adds to `.zshrc` or `.bash_profile`
- **Linux**: Automatically adds to `.bashrc`, `.zshrc`, or `.profile`
- **Windows**: Provides manual instructions for Environment Variables

If automatic setup fails, the tool will provide detailed manual instructions for your platform.

### Workflow Management

```bash
# List all workflows (embedded + user)
amo workflow list

# Download workflow from remote source
amo workflow get https://github.com/user/repo/blob/main/workflow.js

# Download with custom filename
amo workflow get https://raw.githubusercontent.com/user/repo/main/workflow.js --filename my-custom-workflow.js
```

### Runtime Variables

```bash
# Pass variables to workflows
amo run workflow.js --var key1=value1 --var key2=value2

# Use shorthand for common variables
amo run workflow.js --input /path/to/input --output /path/to/output

# Environment variables (workflow can access via getVar())
KEY=value amo run workflow.js

# Show workflow help if supported
amo run workflow.js --workflow-help
```

### Configuration Settings

Amo allows you to customize various settings via a simple configuration system:

```bash
# List all current configuration settings
amo config ls

# Get a specific setting
amo config workflows  # Get custom workflows directory

# Change a configuration setting
amo config workflows ~/my-custom-workflows

# Reset to default value
amo config rm workflows
```

The configuration is stored in `~/.amo/config.yaml` and currently supports:
- **workflows**: Custom directory for workflow files (default: `~/.amo/workflows`)

## 🔧 Writing Workflows

### Basic Workflow Structure

```javascript
//!amo

function main() {
    console.log("🎯 My Workflow");
    
    // Get runtime variables
    var input = getVar("input") || "";
    var output = getVar("output") || "";
    
    // Validate parameters
    if (!input) {
        console.error("❌ Error: input is required");
        return false;
    }
    
    // Check if input exists
    if (!fs.exists(input)) {
        console.error("❌ Error: Input path does not exist:", input);
        return false;
    }
    
    // Process files
    var files = fs.readdir(input);
    if (!files.success) {
        console.error("❌ Failed to read directory:", files.error);
        return false;
    }
    
    console.log("📁 Found", files.files.length, "files");
    
    // Use CLI commands (must be whitelisted)
    var result = cliCommand("echo", ["Processing complete"], { timeout: 10 });
    if (result.error) {
        console.error("❌ Command failed:", result.error);
        return false;
    }
    
    console.log("✅ Success!");
    return true;
}

main();
```

### File System Operations

```javascript
//!amo

function main() {
    // File operations using new fs API
    var content = fs.read("/path/to/file.txt");
    if (content.success) {
        console.log("File content:", content.content);
    }
    
    // Write file
    var writeResult = fs.write("/path/to/output.txt", "Hello World");
    if (!writeResult.success) {
        console.error("Write failed:", writeResult.error);
    }
    
    // Path operations
    var joined = fs.join(["/path", "to", "file.txt"]);
    var extension = fs.ext("/path/to/file.txt");
    var basename = fs.basename("/path/to/file.txt");
    
    console.log("Joined path:", joined);
    console.log("Extension:", extension);
    console.log("Base name:", basename);
    
    // Directory operations
    var dirContent = fs.readdir("./");
    if (dirContent.success) {
        dirContent.files.forEach(function(file) {
            var icon = file.is_dir ? "📁" : "📄";
            console.log(icon + " " + file.name + " (" + file.size + " bytes)");
        });
    }
    
    return true;
}

main();
```

### Network Operations

```javascript
//!amo

function main() {
    // HTTP GET request
    var response = http.get("https://api.example.com/data");
    if (response.status_code === 200) {
        console.log("Response:", response.body);
    } else {
        console.error("Request failed:", response.error || response.status_code);
    }
    
    // JSON response
    var jsonResponse = http.getJSON("https://api.example.com/json");
    if (jsonResponse.data) {
        console.log("JSON data:", jsonResponse.data);
    }
    
    // File download with progress
    var downloadResult = http.downloadFile(
        "https://example.com/file.zip",
        "./download.zip",
        { show_progress: true }
    );
    
    if (downloadResult.status_code === 200) {
        console.log("Download completed successfully");
    }
    
    return true;
}

main();
```

## 💡 Key Features

### Embedded Workflows

Pre-built workflows included:
- **file-organizer.js**: Organize files by extension with many options
- **fs-api-demo.js**: File system API demonstration

### Workflow Loading Priority

Amo loads workflows in this order:
1. **User workflows**: `~/.amo/workflows/` (highest priority)
2. **External file paths**: Full/relative file system paths
3. **Embedded workflows**: Built-in workflows

### Cross-Platform Support

Amo works consistently across:
- **Windows**: Handles path separators and executable extensions
- **macOS**: Native path handling and permissions
- **Linux**: Full POSIX support

### Security Features

- **CLI Whitelist**: Only explicitly allowed commands can be executed
- **Path Validation**: All file operations are validated
- **Timeout Protection**: Commands have configurable timeouts
- **Network Security**: Controlled domain access for downloads

## 🚨 Troubleshooting

**Command not allowed**: Add the command using `amo tool permission add <command>`

**Permission errors**: Ensure the amo binary has execute permissions

**Workflow not found**: Use `amo workflow list` to see available workflows

**JavaScript errors**: Use `--debug` flag to see detailed execution information

**File operations fail**: Check file permissions and paths (use absolute paths when in doubt)

**Download failed**: Ensure the URL is from an allowed domain (GitHub, GitLab, etc.)

## 📚 Next Steps

- **Development**: [DEVELOPMENT.md](DEVELOPMENT.md) for architecture and development info
- **Versioning**: [VERSIONING.md](VERSIONING.md) for build and release details
- **Workflow Development**: [WORKFLOW-DEVELOPMENT.md](WORKFLOW-DEVELOPMENT.md) for creating workflows with IDE support
- **User Guide**: [README.md](README.md) for complete documentation 