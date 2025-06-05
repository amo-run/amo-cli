package main

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"amo/pkg/env"
)

// EmbeddedAssets contains all embedded asset files
//
//go:embed assets
var EmbeddedAssets embed.FS

// AssetManager provides access to embedded assets
type AssetManager struct {
	fs            embed.FS
	crossPlatform *env.CrossPlatformUtils
}

// NewAssetManager creates a new asset manager
func NewAssetManager() *AssetManager {
	return &AssetManager{
		fs:            EmbeddedAssets,
		crossPlatform: env.NewCrossPlatformUtils(),
	}
}

// ReadFile reads a file from the embedded assets
func (am *AssetManager) ReadFile(path string) ([]byte, error) {
	// Normalize path to use forward slashes and remove leading slash
	path = strings.TrimPrefix(filepath.ToSlash(path), "/")

	// If path doesn't start with assets/, try different combinations
	if !strings.HasPrefix(path, "assets/") {
		// Try workflow files
		if strings.HasSuffix(path, ".js") {
			// Try workflow directory
			if data, err := am.fs.ReadFile("assets/workflow/" + path); err == nil {
				return data, nil
			}
		}

		// Try tools.json directly
		if path == "tools.json" {
			if data, err := am.fs.ReadFile("assets/tools.json"); err == nil {
				return data, nil
			}
		}

		// Default to assets prefix
		path = "assets/" + path
	}

	return am.fs.ReadFile(path)
}

// ReadFileAsString reads a file from the embedded assets as string
func (am *AssetManager) ReadFileAsString(path string) (string, error) {
	data, err := am.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Exists checks if a file exists in the embedded assets
func (am *AssetManager) Exists(path string) bool {
	// Normalize path to use forward slashes and remove leading slash
	path = strings.TrimPrefix(filepath.ToSlash(path), "/")

	// If path doesn't start with assets/, try different combinations
	if !strings.HasPrefix(path, "assets/") {
		// Try workflow files
		if strings.HasSuffix(path, ".js") {
			// Try workflow directory
			if _, err := am.fs.ReadFile("assets/workflow/" + path); err == nil {
				return true
			}
		}

		// Try tools.json directly
		if path == "tools.json" {
			if _, err := am.fs.ReadFile("assets/tools.json"); err == nil {
				return true
			}
		}

		// Then try with assets/ prefix
		path = "assets/" + path
	}

	_, err := am.fs.ReadFile(path)
	return err == nil
}

// ListFiles lists all files in a directory within the embedded assets
func (am *AssetManager) ListFiles(dir string) ([]string, error) {
	// Normalize directory path for embedded assets (always use forward slashes)
	dir = strings.TrimPrefix(filepath.ToSlash(dir), "/")
	if !strings.HasPrefix(dir, "assets/") {
		dir = "assets/" + dir
	}

	var files []string
	err := fs.WalkDir(am.fs, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			// Remove the assets/ prefix for cleaner paths
			cleanPath := strings.TrimPrefix(path, "assets/")
			files = append(files, cleanPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files in directory %s: %w", dir, err)
	}

	return files, nil
}

// GetWorkflowFileNames returns a list of available workflow file names
func (am *AssetManager) GetWorkflowFileNames() ([]string, error) {
	var allFiles []string

	// List from workflow directory
	workflowFiles, err := am.ListFiles("workflow")
	if err == nil {
		allFiles = append(allFiles, workflowFiles...)
	}

	// Note: No longer checking workflows directory since tool management moved to Go

	var names []string
	for _, file := range allFiles {
		// Extract just the filename from the full path
		name := filepath.Base(file)
		if strings.HasSuffix(name, ".js") {
			names = append(names, name)
		}
	}

	return names, nil
}

// WriteToFile writes an embedded asset to a physical file
func (am *AssetManager) WriteToFile(assetPath, outputPath string) error {
	data, err := am.ReadFile(assetPath)
	if err != nil {
		return fmt.Errorf("failed to read embedded asset %s: %w", assetPath, err)
	}

	// Normalize output path for the current platform
	outputPath = am.crossPlatform.NormalizePath(outputPath)

	// Create directory if it doesn't exist with appropriate permissions
	dir := filepath.Dir(outputPath)
	if err := am.crossPlatform.CreateDirWithPermissions(dir); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file with appropriate permissions
	err = am.crossPlatform.CreateFileWithPermissions(outputPath, data, false)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", outputPath, err)
	}

	return nil
}

// WriteExecutableToFile writes an embedded asset as an executable file
func (am *AssetManager) WriteExecutableToFile(assetPath, outputPath string) error {
	data, err := am.ReadFile(assetPath)
	if err != nil {
		return fmt.Errorf("failed to read embedded asset %s: %w", assetPath, err)
	}

	// Add executable extension if needed and normalize path
	outputPath = am.crossPlatform.AddExecutableExtensionIfNeeded(outputPath)
	outputPath = am.crossPlatform.NormalizePath(outputPath)

	// Create directory if it doesn't exist with appropriate permissions
	dir := filepath.Dir(outputPath)
	if err := am.crossPlatform.CreateDirWithPermissions(dir); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write executable file with appropriate permissions
	err = am.crossPlatform.CreateFileWithPermissions(outputPath, data, true)
	if err != nil {
		return fmt.Errorf("failed to write executable file %s: %w", outputPath, err)
	}

	return nil
}

// GetAssetPath normalizes and validates an asset path
func (am *AssetManager) GetAssetPath(path string) (string, error) {
	// Normalize input path
	inputPath := am.crossPlatform.NormalizePath(path)

	// Convert to embedded asset path format (forward slashes)
	assetPath := filepath.ToSlash(inputPath)

	// Validate the path
	if !am.IsValidAssetPath(assetPath) {
		return "", fmt.Errorf("invalid asset path: %s", path)
	}

	return assetPath, nil
}

// IsValidAssetPath checks if an asset path is valid
func (am *AssetManager) IsValidAssetPath(path string) bool {
	// Check for dangerous path components
	if strings.Contains(path, "..") {
		return false
	}

	// Check for absolute paths (embedded assets should be relative)
	if strings.HasPrefix(path, "/") && len(path) > 1 {
		return false
	}

	return true
}

// GetAssetManager returns a global asset manager instance
func GetAssetManager() *AssetManager {
	return NewAssetManager()
}
