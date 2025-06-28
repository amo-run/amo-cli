#!/bin/bash
set -e

# Amo CLI Installation Script
# Supports: Linux (amd64, arm64, armv7), macOS (amd64, arm64)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="amo-run/amo-cli"
BINARY_NAME="amo"
INSTALL_DIR="/usr/local/bin"
BASE_URL="https://cli.release.amo.run"

# Helper functions
log_info() {
    printf "${BLUE}[INFO]${NC} %s\n" "$1"
}

log_success() {
    printf "${GREEN}[SUCCESS]${NC} %s\n" "$1"
}

log_warning() {
    printf "${YELLOW}[WARNING]${NC} %s\n" "$1"
}

log_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1"
}

# Detect platform and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case "$os" in
        linux*)
            PLATFORM="linux"
            ;;
        darwin*)
            PLATFORM="darwin"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    case "$arch" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        armv7l)
            if [ "$PLATFORM" = "linux" ]; then
                ARCH="armv7"
            else
                log_error "ARMv7 is only supported on Linux"
                exit 1
            fi
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    if [ "$PLATFORM" = "darwin" ] && [ "$ARCH" = "armv7" ]; then
        log_error "ARMv7 is not supported on macOS"
        exit 1
    fi
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Download file with progress
download_file() {
    local url="$1"
    local output="$2"
    
    if command_exists curl; then
        curl -fsSL --progress-bar "$url" -o "$output"
    elif command_exists wget; then
        wget -q --show-progress "$url" -O "$output"
    else
        log_error "Neither curl nor wget is available. Please install one of them."
        exit 1
    fi
}

# Get SHA256 hash of a file (cross-platform)
get_sha256() {
    local file="$1"
    
    if command_exists sha256sum; then
        sha256sum "$file" | cut -d' ' -f1
    elif command_exists shasum; then
        shasum -a 256 "$file" | cut -d' ' -f1
    elif command_exists openssl; then
        openssl dgst -sha256 "$file" | cut -d' ' -f2
    else
        return 1
    fi
}

# Verify checksum if available
verify_checksum() {
    local file="$1"
    local checksum_url="${BASE_URL}/${BINARY_FILE}.sha256"
    
    # Check if any SHA256 tool is available
    if command_exists sha256sum || command_exists shasum || command_exists openssl; then
        log_info "Verifying checksum..."
        local temp_checksum=$(mktemp)
        
        if download_file "$checksum_url" "$temp_checksum" 2>/dev/null; then
            local expected_checksum=$(cat "$temp_checksum" | cut -d' ' -f1)
            local actual_checksum=$(get_sha256 "$file")
            
            if [ $? -eq 0 ] && [ "$expected_checksum" = "$actual_checksum" ]; then
                log_success "Checksum verification passed ✓"
            else
                log_warning "Checksum verification failed, but continuing installation"
                log_warning "Expected: $expected_checksum"
                log_warning "Actual:   $actual_checksum"
            fi
            rm -f "$temp_checksum"
        else
            log_warning "Could not download checksum file, skipping verification"
        fi
    else
        log_warning "No SHA256 tool available (sha256sum/shasum/openssl), skipping checksum verification"
        log_info "To enable checksum verification, install one of: sha256sum, shasum, or openssl"
    fi
}

# Check if user has sudo privileges
check_sudo() {
    if [ "$EUID" -ne 0 ] && [ ! -w "$INSTALL_DIR" ]; then
        if command_exists sudo; then
            SUDO="sudo"
            log_info "Installation requires sudo privileges"
        else
            log_error "No write permission to $INSTALL_DIR and sudo is not available"
            log_info "Try running with sudo or install to a different directory"
            exit 1
        fi
    fi
}

# Create install directory if it doesn't exist
create_install_dir() {
    if [ ! -d "$INSTALL_DIR" ]; then
        log_info "Creating install directory: $INSTALL_DIR"
        $SUDO mkdir -p "$INSTALL_DIR"
    fi
}

# Install binary
install_binary() {
    local temp_file="$1"
    local install_path="${INSTALL_DIR}/${BINARY_NAME}"
    
    log_info "Installing $BINARY_NAME to $install_path"
    $SUDO cp "$temp_file" "$install_path"
    $SUDO chmod 755 "$install_path"
    
    # Verify installation
    if [ -x "$install_path" ]; then
        log_success "Installation completed successfully!"
        log_info "Run '$BINARY_NAME --help' to get started"
        
        # Try to show version if possible
        if command_exists "$BINARY_NAME"; then
            log_info "Installed version: $($BINARY_NAME --version 2>/dev/null || printf 'Unknown')"
        else
            log_warning "Binary installed but not in PATH. You may need to restart your shell or add $INSTALL_DIR to your PATH"
        fi
    else
        log_error "Installation failed: binary not executable"
        exit 1
    fi
}

# Main installation function
main() {
    log_info "Amo CLI Installation Script"
    log_info "=========================="
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --install-dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            --help|-h)
                printf "Usage: %s [OPTIONS]\n" "$0"
                printf "\n"
                printf "Options:\n"
                printf "  --install-dir DIR    Install directory (default: /usr/local/bin)\n"
                printf "  --help, -h          Show this help message\n"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                printf "Use --help for usage information\n"
                exit 1
                ;;
        esac
    done
    
    # Detect platform and architecture
    detect_platform
    log_info "Detected platform: $PLATFORM"
    log_info "Detected architecture: $ARCH"
    
    # Set binary filename
    BINARY_FILE="${BINARY_NAME}_${PLATFORM}_${ARCH}"
    if [ "$ARCH" = "armv7" ]; then
        BINARY_FILE="${BINARY_NAME}_${PLATFORM}_armv7"
    fi
    
    DOWNLOAD_URL="${BASE_URL}/${BINARY_FILE}"
    log_info "Download URL: $DOWNLOAD_URL"
    
    # Check prerequisites
    check_sudo
    create_install_dir
    
    # Download binary
    log_info "Downloading $BINARY_NAME..."
    TEMP_FILE=$(mktemp)
    trap "rm -f $TEMP_FILE" EXIT
    
    download_file "$DOWNLOAD_URL" "$TEMP_FILE"
    
    # Make sure the downloaded file is not empty
    if [ ! -s "$TEMP_FILE" ]; then
        log_error "Downloaded file is empty. Please check if the release exists."
        exit 1
    fi
    
    # Verify checksum
    verify_checksum "$TEMP_FILE"
    
    # Make binary executable
    chmod +x "$TEMP_FILE"
    
    # Install binary
    install_binary "$TEMP_FILE"
    
    log_success "Installation completed! 🎉"
    printf "\n"
    log_info "Quick start:"
    printf "  %s --help         # Show help\n" "$BINARY_NAME"
    printf "  %s workflow list  # List available workflows\n" "$BINARY_NAME"
    printf "  %s tool list      # List available tools\n" "$BINARY_NAME"
}

# Run main function
main "$@" 