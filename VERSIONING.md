# Version Management Guide

Semantic versioning system, build management and release process for the Amo Workflow Engine.

## 📚 Related Documentation

- **🔧 [Development Guide](DEVELOPMENT.md)** - Architecture, code patterns, and development workflow
- **🚀 [Quick Start](QUICKSTART.md)** - Installation and basic usage
- **📖 [User Guide](README.md)** - Complete usage documentation

## 🏷️ Semantic Versioning

This project follows [Semantic Versioning](https://semver.org/) with `MAJOR.MINOR.PATCH` format:

- **MAJOR**: Breaking changes
- **MINOR**: New features, backward compatible  
- **PATCH**: Bug fixes, backward compatible

### Pre-release Examples
- `v1.0.0-alpha.1` - Alpha version
- `v1.0.0-beta.2` - Beta version  
- `v1.0.0-rc.3` - Release candidate

## 📊 Version Information Display

### Viewing Version Details

```bash
# Quick version
amo --version
# Output: amo v1.0.0

# Detailed information
amo version
# Output:
# 🚀 Amo Workflow Engine
# 🔖 Version Information:
#   Version:     v1.0.0
#   Git Commit:  a1b2c3d4...
#   Build Time:  2024-01-15T10:30:45Z
#   Built By:    user@hostname
# ⚙️ Runtime Information:
#   Go Version:  go1.21.0
#   OS/Arch:     darwin/arm64
```

### Version Sources

1. **Git Tags**: Official releases (`v1.0.0`)
2. **Development**: Branch + commit (`v1.0.0-dev+a1b2c3d`)
3. **Build Info**: Timestamp and builder environment

### Development Version Naming

| Scenario | Version Format | Example |
|----------|----------------|---------|
| **Main branch** | `v1.0.0-dev+{commit}` | `v1.0.0-dev+a1b2c3d` |
| **Feature branch** | `v1.0.0-{branch}+{commit}` | `v1.0.0-feature-api+a1b2c3d` |
| **Tagged release** | `v{major}.{minor}.{patch}` | `v1.0.0` |

## 🔨 Build System

### Using Build Script

```bash
# Local development build
./build.sh local

# Development build with dev suffix  
./build.sh dev

# Multi-platform release build
./build.sh release
```

### Manual Build with Version Injection

```bash
# Get version info
VERSION=$(git describe --tags --exact-match 2>/dev/null || echo "v1.0.0-dev+$(git rev-parse --short HEAD)")
GIT_COMMIT=$(git rev-parse HEAD)
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
BUILD_BY="${USER}@$(hostname)"

# Build with injected version
go build -ldflags "-s -w \
    -X 'main.Version=${VERSION}' \
    -X 'main.GitCommit=${GIT_COMMIT}' \
    -X 'main.BuildTime=${BUILD_TIME}' \
    -X 'main.BuildBy=${BUILD_BY}'" \
    -o amo .
```

### Quick Local Install

```bash
# Install to $HOME/go/bin with version injection
./install-bin.sh
```

### Build System Features

- **Cross-platform builds**: Linux, macOS, Windows (x64, ARM64)
- **Version injection**: Git-based version info embedded at build time
- **Automatic checksums**: SHA256 for all release binaries
- **Development builds**: Include branch and commit suffix for traceability

## 🚀 Release Process

### 1. Create Release Tag

```bash
# Create and push tag
git tag v1.1.0
git push origin v1.1.0

# Or annotated tag with message
git tag -a v1.1.0 -m "Release v1.1.0: New workflow features"
git push origin v1.1.0
```

### 2. Automated Release (GitHub Actions)

Tag push automatically triggers:
1. Multi-platform binary builds (Linux, macOS, Windows x64/ARM64)
2. Version injection with build-time info
3. SHA256 checksum generation
4. GitHub release creation with artifacts

### 3. Manual Release Build

```bash
# Build all platforms with checksums
./build.sh release

# Output in dist/ directory
ls -la dist/
# amo-linux-amd64        amo-linux-amd64.sha256
# amo-darwin-arm64       amo-darwin-arm64.sha256
# amo-windows-amd64.exe  amo-windows-amd64.exe.sha256
```

## 🤖 CI/CD Integration

### GitHub Actions Workflow

The workflow (`.github/workflows/release.yml`) provides:

- **Automatic version detection** from Git tags and commits
- **Multi-platform builds** with consistent version injection
- **Build artifact management** with checksums
- **Release automation** for tagged versions

### Triggering Releases

```bash
# Push tag to trigger automated release
git tag v1.1.0
git push origin v1.1.0

# GitHub Actions automatically:
# 1. Detects tag and extracts version
# 2. Builds binaries for all platforms  
# 3. Generates checksums and creates release
# 4. Uploads all artifacts with release notes
```

### Build Artifact Organization

Release creates organized artifacts:
```
amo-linux-amd64.tar.gz
amo-linux-amd64.sha256
amo-darwin-arm64.tar.gz  
amo-darwin-arm64.sha256
amo-windows-amd64.zip
amo-windows-amd64.sha256
```

## ✅ Quick Reference

| Task | Command |
|------|---------|
| **Show version** | `amo --version` |
| **Detailed version** | `amo version` |
| **Local build** | `./build.sh local` |
| **Development build** | `./build.sh dev` |
| **Release build** | `./build.sh release` |
| **Install locally** | `./install-bin.sh` |
| **Create release** | `git tag v1.x.x && git push origin v1.x.x` |
| **List versions** | `git tag -l` |

## 📚 Git Version Management

### Useful Commands

```bash
# Version comparison
git log --oneline v1.0.0..v1.1.0
git diff --name-only v1.0.0 v1.1.0

# Latest tag
git describe --tags --abbrev=0

# Tag management
git tag -l                    # List all tags
git tag -d v1.0.0            # Delete local tag  
git push origin :refs/tags/v1.0.0  # Delete remote tag
```

### Release Checklist

- [ ] All tests pass (`go test ./...`)
- [ ] Documentation updated
- [ ] Version tag created (`git tag v1.x.x`)
- [ ] Tag pushed to trigger release (`git push origin v1.x.x`)
- [ ] GitHub Actions builds complete successfully
- [ ] Release artifacts verified (checksums match)

This versioning system ensures consistent, traceable, and automated version management throughout the development and release lifecycle. 