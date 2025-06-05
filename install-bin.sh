#!/bin/bash
# Version: 2025-05-30
# This script is used to build and install the binary to local bin directory for local testing and using the binary.
# Supports both Linux and macOS with proper version injection.

set -e

BIN_NAME="amo"
INSTALL_DIR="$HOME/go/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}📄 $BIN_NAME Installation${NC}"
echo "=================================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go is not installed. Please install Go first.${NC}"
    exit 1
fi

# Create install directory if it doesn't exist
mkdir -p "$INSTALL_DIR"

# Get version information
get_version_info() {
    # Try to get version from git tag
    VERSION=$(git describe --tags --exact-match 2>/dev/null || echo "")
    
    if [ -z "$VERSION" ]; then
        # If no exact tag, use branch and commit info
        BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
        COMMIT_SHORT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
        if [ "$BRANCH" = "main" ] || [ "$BRANCH" = "master" ]; then
            VERSION="v1.0.0-dev+${COMMIT_SHORT}"
        else
            VERSION="v1.0.0-${BRANCH}+${COMMIT_SHORT}"
        fi
    fi
    
    # Git commit (full hash)
    GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
    
    # Build time in ISO 8601 format
    BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
    
    # Build environment info
    BUILD_BY="${USER}@$(hostname)"
    
    echo -e "${BLUE}🔍 Version Information:${NC}"
    echo "  Version: $VERSION"
    echo "  Commit: ${GIT_COMMIT:0:8}"
    echo "  Build Time: $BUILD_TIME"
    echo ""
}

# Check if git is available and get version info
if command -v git &> /dev/null; then
    get_version_info
else
    echo -e "${YELLOW}⚠️  Git not found, using default version info${NC}"
    VERSION="v1.0.0-dev"
    GIT_COMMIT="unknown"
    BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
    BUILD_BY="${USER}@$(hostname)"
    echo ""
fi

# Build the binary with version injection
echo -e "${YELLOW}🔨 Building $BIN_NAME with version injection...${NC}"
if go build -v \
    -ldflags "-s -w \
        -X 'main.Version=${VERSION}' \
        -X 'main.GitCommit=${GIT_COMMIT}' \
        -X 'main.BuildTime=${BUILD_TIME}' \
        -X 'main.BuildBy=${BUILD_BY}'" \
    -o "$INSTALL_DIR/$BIN_NAME"; then
    echo -e "${GREEN}✅ Build successful!${NC}"
else
    echo -e "${RED}❌ Build failed!${NC}"
    exit 1
fi

# Make sure it's executable
chmod +x "$INSTALL_DIR/$BIN_NAME"

# Test the binary version
echo -e "${BLUE}🧪 Testing installed binary...${NC}"
if "$INSTALL_DIR/$BIN_NAME" --version > /dev/null 2>&1; then
    INSTALLED_VERSION=$("$INSTALL_DIR/$BIN_NAME" --version)
    echo -e "${GREEN}✅ Version check successful: ${INSTALLED_VERSION}${NC}"
else
    echo -e "${YELLOW}⚠️  Version check failed, but binary was installed${NC}"
fi

# Check if $HOME/go/bin is in PATH
if [[ ":$PATH:" != *":$HOME/go/bin:"* ]]; then
    echo -e "${YELLOW}⚠️  Warning: $HOME/go/bin is not in your PATH${NC}"
    echo "Add this to your shell profile (e.g., ~/.bashrc, ~/.zshrc):"
    echo "export PATH=\"\$HOME/go/bin:\$PATH\""
    echo ""
fi

echo -e "${GREEN}🎉 Installation completed!${NC}"
echo "Binary installed at: $INSTALL_DIR/$BIN_NAME"
echo ""
echo "Usage examples:"
echo "  $BIN_NAME --help                   # Show help"
echo "  $BIN_NAME --version                # Show version"
echo "  $BIN_NAME version                  # Show detailed version info"