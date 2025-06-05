#!/bin/bash
# Build script for binary with version injection
# Supports both development and release builds

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

BINARY_NAME="amo"
OUTPUT_DIR="dist"

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
    
    echo "Version: $VERSION"
    echo "Commit: $GIT_COMMIT"
    echo "Build Time: $BUILD_TIME"
    echo "Built By: $BUILD_BY"
}

# Build function
build_binary() {
    local os=$1
    local arch=$2
    local output_name=$3
    
    echo -e "${BLUE}Building for $os/$arch...${NC}"
    
    # Set environment variables for cross-compilation
    export GOOS=$os
    export GOARCH=$arch
    export CGO_ENABLED=0
    
    # Build with version injection
    go build -v \
        -ldflags "-s -w \
            -X 'main.Version=${VERSION}' \
            -X 'main.GitCommit=${GIT_COMMIT}' \
            -X 'main.BuildTime=${BUILD_TIME}' \
            -X 'main.BuildBy=${BUILD_BY}'" \
        -o "${OUTPUT_DIR}/${output_name}" .
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ Successfully built: ${OUTPUT_DIR}/${output_name}${NC}"
        
        # Show file info
        if [ -f "${OUTPUT_DIR}/${output_name}" ]; then
            echo -e "${BLUE}   File size: $(ls -lh "${OUTPUT_DIR}/${output_name}" | awk '{print $5}')${NC}"
        fi
    else
        echo -e "${RED}❌ Build failed for $os/$arch${NC}"
        return 1
    fi
}

# Main build function
main() {
    echo -e "${BLUE}📦 Doc Text Extractor Build Script${NC}"
    echo "=================================="
    
    # Check if git is available
    if ! command -v git &> /dev/null; then
        echo -e "${YELLOW}⚠️  Git not found, using default version info${NC}"
        VERSION="v1.0.0-dev"
        GIT_COMMIT="unknown"
        BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
        BUILD_BY="${USER}@$(hostname)"
    else
        echo -e "${BLUE}🔍 Getting version information...${NC}"
        get_version_info
    fi
    
    echo ""
    
    # Create output directory
    mkdir -p "$OUTPUT_DIR"
    
    # Parse command line arguments
    case "${1:-local}" in
        "local")
            echo -e "${BLUE}🔨 Building for local platform...${NC}"
            build_binary "$(go env GOOS)" "$(go env GOARCH)" "$BINARY_NAME"
            ;;
        "all")
            echo -e "${BLUE}🔨 Building for all platforms...${NC}"
            echo ""
            
            # Linux builds
            build_binary "linux" "amd64" "${BINARY_NAME}-linux-amd64"
            build_binary "linux" "arm64" "${BINARY_NAME}-linux-arm64"
            
            # macOS builds
            build_binary "darwin" "amd64" "${BINARY_NAME}-darwin-amd64"
            build_binary "darwin" "arm64" "${BINARY_NAME}-darwin-arm64"
            
            # Windows builds
            build_binary "windows" "amd64" "${BINARY_NAME}-windows-amd64.exe"
            build_binary "windows" "arm64" "${BINARY_NAME}-windows-arm64.exe"
            ;;
        "release")
            echo -e "${BLUE}🚀 Building release version...${NC}"
            echo ""
            
            # Verify we're on a clean git state for release
            if command -v git &> /dev/null; then
                if [ -n "$(git status --porcelain)" ]; then
                    echo -e "${YELLOW}⚠️  Warning: Working directory is not clean${NC}"
                    echo -e "${YELLOW}   Consider committing changes before release build${NC}"
                    echo ""
                fi
            fi
            
            # Build all platforms for release
            build_binary "linux" "amd64" "${BINARY_NAME}-linux-amd64"
            build_binary "linux" "arm64" "${BINARY_NAME}-linux-arm64"
            build_binary "darwin" "amd64" "${BINARY_NAME}-darwin-amd64"
            build_binary "darwin" "arm64" "${BINARY_NAME}-darwin-arm64"
            build_binary "windows" "amd64" "${BINARY_NAME}-windows-amd64.exe"
            build_binary "windows" "arm64" "${BINARY_NAME}-windows-arm64.exe"
            
            # Generate checksums
            echo -e "${BLUE}📋 Generating checksums...${NC}"
            cd "$OUTPUT_DIR"
            for file in ${BINARY_NAME}-*; do
                if [ -f "$file" ]; then
                    sha256sum "$file" > "${file}.sha256"
                    echo "   ✅ ${file}.sha256"
                fi
            done
            cd ..
            ;;
        "dev")
            echo -e "${BLUE}🔧 Building development version...${NC}"
            build_binary "$(go env GOOS)" "$(go env GOARCH)" "${BINARY_NAME}-dev"
            ;;
        *)
            echo -e "${YELLOW}Usage: $0 [local|all|release|dev]${NC}"
            echo ""
            echo "Commands:"
            echo "  local   - Build for current platform (default)"
            echo "  all     - Build for all supported platforms"
            echo "  release - Build release version with checksums"
            echo "  dev     - Build development version"
            exit 1
            ;;
    esac
    
    echo ""
    echo -e "${GREEN}🎉 Build completed successfully!${NC}"
    echo -e "${BLUE}📁 Output directory: $OUTPUT_DIR${NC}"
    echo ""
    
    # Show built files
    if [ -d "$OUTPUT_DIR" ]; then
        echo "Built files:"
        ls -la "$OUTPUT_DIR"
    fi
}

# Run main function
main "$@" 