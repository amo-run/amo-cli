package filesystem

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"amo/pkg/env"
)

// FileInfo represents file information
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
	Mode    string `json:"mode"`
}

// FileSystem provides file system operations
type FileSystem struct {
	crossPlatform *env.CrossPlatformUtils
}

// NewFileSystem creates a new FileSystem instance
func NewFileSystem() *FileSystem {
	return &FileSystem{
		crossPlatform: env.NewCrossPlatformUtils(),
	}
}

// IsFile checks if the given path is a file
func (fs *FileSystem) IsFile(path string) bool {
	path = fs.crossPlatform.NormalizePath(path)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// IsDir checks if the given path is a directory
func (fs *FileSystem) IsDir(path string) bool {
	path = fs.crossPlatform.NormalizePath(path)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Exists checks if the given path exists
func (fs *FileSystem) Exists(path string) bool {
	path = fs.crossPlatform.NormalizePath(path)
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetFileInfo returns detailed information about a file or directory
func (fs *FileSystem) GetFileInfo(path string) (*FileInfo, error) {
	path = fs.crossPlatform.NormalizePath(path)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %w", path, err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	absPath = fs.crossPlatform.NormalizePath(absPath)

	return &FileInfo{
		Name:    info.Name(),
		Path:    absPath,
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().Format(time.RFC3339),
		Mode:    info.Mode().String(),
	}, nil
}

// List returns a list of files and directories in the given directory
func (fs *FileSystem) List(dirPath string) ([]FileInfo, error) {
	dirPath = fs.crossPlatform.NormalizePath(dirPath)
	if !fs.IsDir(dirPath) {
		return nil, fmt.Errorf("path is not a directory: %s", dirPath)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var files []FileInfo
	for _, entry := range entries {
		entryPath := fs.crossPlatform.JoinPath(dirPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't read
		}

		absPath, err := filepath.Abs(entryPath)
		if err != nil {
			absPath = entryPath
		}
		absPath = fs.crossPlatform.NormalizePath(absPath)

		files = append(files, FileInfo{
			Name:    info.Name(),
			Path:    absPath,
			Size:    info.Size(),
			IsDir:   info.IsDir(),
			ModTime: info.ModTime().Format(time.RFC3339),
			Mode:    info.Mode().String(),
		})
	}

	return files, nil
}

// MakeDir creates a directory and all necessary parent directories
func (fs *FileSystem) MakeDir(dirPath string) error {
	dirPath = fs.crossPlatform.NormalizePath(dirPath)
	err := fs.crossPlatform.CreateDirWithPermissions(dirPath)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}
	return nil
}

// Copy copies a file or directory from src to dst
func (fs *FileSystem) Copy(src, dst string) error {
	src = fs.crossPlatform.NormalizePath(src)
	dst = fs.crossPlatform.NormalizePath(dst)

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source path does not exist: %s", src)
	}

	if srcInfo.IsDir() {
		return fs.copyDir(src, dst)
	}
	return fs.copyFile(src, dst)
}

// copyFile copies a single file from src to dst
func (fs *FileSystem) copyFile(src, dst string) error {
	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)
	if err := fs.MakeDir(dstDir); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Copy file permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	err = os.Chmod(dst, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory from src to dst
func (fs *FileSystem) copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get source directory info: %w", err)
	}

	// Create destination directory with appropriate permissions
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := fs.crossPlatform.JoinPath(src, entry.Name())
		dstPath := fs.crossPlatform.JoinPath(dst, entry.Name())

		if entry.IsDir() {
			err = fs.copyDir(srcPath, dstPath)
		} else {
			err = fs.copyFile(srcPath, dstPath)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// Move moves a file or directory from src to dst
func (fs *FileSystem) Move(src, dst string) error {
	src = fs.crossPlatform.NormalizePath(src)
	dst = fs.crossPlatform.NormalizePath(dst)

	// Try to rename first (works if src and dst are on the same filesystem)
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// If rename fails, try copy and delete
	err = fs.Copy(src, dst)
	if err != nil {
		return fmt.Errorf("failed to copy during move operation: %w", err)
	}

	err = fs.Delete(src)
	if err != nil {
		// Try to clean up the copied destination
		fs.Delete(dst)
		return fmt.Errorf("failed to delete source during move operation: %w", err)
	}

	return nil
}

// Delete removes a file or directory
func (fs *FileSystem) Delete(path string) error {
	path = fs.crossPlatform.NormalizePath(path)
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", path, err)
	}
	return nil
}

// ReadFile reads the entire content of a file
func (fs *FileSystem) ReadFile(path string) (string, error) {
	path = fs.crossPlatform.NormalizePath(path)
	if !fs.IsFile(path) {
		return "", fmt.Errorf("path is not a file: %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return string(content), nil
}

// WriteFile writes content to a file
func (fs *FileSystem) WriteFile(path, content string) error {
	path = fs.crossPlatform.NormalizePath(path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := fs.MakeDir(dir); err != nil {
		return err
	}

	err := fs.crossPlatform.CreateFileWithPermissions(path, []byte(content), false)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}

// AppendFile appends content to a file
func (fs *FileSystem) AppendFile(path, content string) error {
	path = fs.crossPlatform.NormalizePath(path)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fs.crossPlatform.GetDefaultFilePermissions())
	if err != nil {
		return fmt.Errorf("failed to open file for append %s: %w", path, err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to append to file %s: %w", path, err)
	}

	return nil
}

// GetSize returns the size of a file or directory
func (fs *FileSystem) GetSize(path string) (int64, error) {
	path = fs.crossPlatform.NormalizePath(path)

	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get size for %s: %w", path, err)
	}

	if !info.IsDir() {
		return info.Size(), nil
	}

	// For directories, calculate total size recursively
	var totalSize int64
	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to calculate directory size for %s: %w", path, err)
	}

	return totalSize, nil
}

// Find searches for files and directories matching a pattern
func (fs *FileSystem) Find(rootPath, pattern string) ([]string, error) {
	rootPath = fs.crossPlatform.NormalizePath(rootPath)

	if !fs.Exists(rootPath) {
		return nil, fmt.Errorf("root path does not exist: %s", rootPath)
	}

	var matches []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the file name matches the pattern
		matched, err := filepath.Match(pattern, info.Name())
		if err != nil {
			return err
		}

		if matched {
			absPath, err := filepath.Abs(path)
			if err != nil {
				absPath = path
			}
			absPath = fs.crossPlatform.NormalizePath(absPath)
			matches = append(matches, absPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search in %s: %w", rootPath, err)
	}

	return matches, nil
}

// GetWorkingDir returns the current working directory
func (fs *FileSystem) GetWorkingDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return fs.crossPlatform.NormalizePath(cwd), nil
}

// ChangeDir changes the current working directory
func (fs *FileSystem) ChangeDir(path string) error {
	path = fs.crossPlatform.NormalizePath(path)
	if !fs.IsDir(path) {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	err := os.Chdir(path)
	if err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", path, err)
	}

	return nil
}

// GetAbsolutePath returns the absolute path of a given path
func (fs *FileSystem) GetAbsolutePath(path string) (string, error) {
	path = fs.crossPlatform.NormalizePath(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}
	return fs.crossPlatform.NormalizePath(absPath), nil
}

// GetRelativePath returns the relative path from base to target
func (fs *FileSystem) GetRelativePath(base, target string) (string, error) {
	base = fs.crossPlatform.NormalizePath(base)
	target = fs.crossPlatform.NormalizePath(target)

	relPath, err := filepath.Rel(base, target)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path from %s to %s: %w", base, target, err)
	}
	return fs.crossPlatform.NormalizePath(relPath), nil
}

// JoinPath joins path elements into a single path
func (fs *FileSystem) JoinPath(elements ...string) string {
	return fs.crossPlatform.JoinPath(elements...)
}

// SplitPath splits a path into directory and file components
func (fs *FileSystem) SplitPath(path string) (dir, file string) {
	path = fs.crossPlatform.NormalizePath(path)
	dir, file = filepath.Split(path)
	return fs.crossPlatform.NormalizePath(dir), file
}

// GetExtension returns the file extension
func (fs *FileSystem) GetExtension(path string) string {
	return filepath.Ext(path)
}

// GetFileName returns the file name of the path (without directory)
func (fs *FileSystem) GetFileName(path string) string {
	return filepath.Base(path)
}

// GetBaseName returns the base name of the path (without directory and extension)
func (fs *FileSystem) GetBaseName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext != "" {
		return strings.TrimSuffix(base, ext)
	}
	return base
}

// GetDirName returns the directory name of the path
func (fs *FileSystem) GetDirName(path string) string {
	return fs.crossPlatform.NormalizePath(filepath.Dir(path))
}

// IsValidPath checks if a path is valid for the current operating system
func (fs *FileSystem) IsValidPath(path string) bool {
	// Split path into components and check each one
	normalizedPath := fs.crossPlatform.NormalizePath(path)
	pathComponents := strings.Split(normalizedPath, fs.crossPlatform.GetPathSeparator())

	for _, component := range pathComponents {
		if component != "" && !fs.crossPlatform.IsValidFilename(component) {
			return false
		}
	}

	return true
}

// CreateExecutableFile creates an executable file with appropriate permissions and extension
func (fs *FileSystem) CreateExecutableFile(path string, content []byte) error {
	// Add executable extension if needed (e.g., .exe on Windows)
	path = fs.crossPlatform.AddExecutableExtensionIfNeeded(path)
	path = fs.crossPlatform.NormalizePath(path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := fs.MakeDir(dir); err != nil {
		return err
	}

	err := fs.crossPlatform.CreateFileWithPermissions(path, content, true)
	if err != nil {
		return fmt.Errorf("failed to create executable file %s: %w", path, err)
	}

	return nil
}

// GetFileMD5 calculates the MD5 hash of a file
func (fs *FileSystem) GetFileMD5(path string) (string, error) {
	path = fs.crossPlatform.NormalizePath(path)

	if !fs.IsFile(path) {
		return "", fmt.Errorf("path is not a file: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file for MD5 calculation: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate MD5 hash: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetTempFilePath returns a path for a temporary file with optional prefix
func (fs *FileSystem) GetTempFilePath(prefix string) (string, error) {
	// Get system temp directory
	tempDir := fs.crossPlatform.GetTempDir()

	// Create a unique filename
	if prefix == "" {
		prefix = "amo_tmp_"
	}

	// Generate a random string for uniqueness
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for temp file: %w", err)
	}

	randomStr := hex.EncodeToString(randomBytes)
	fileName := fmt.Sprintf("%s%s", prefix, randomStr)

	// Return the full path
	tempPath := filepath.Join(tempDir, fileName)
	return fs.crossPlatform.NormalizePath(tempPath), nil
}

// GenerateUniqueFilename generates a unique filename by adding a counter suffix if the file exists.
// If the original file does not exist, it returns the original path.
// If it exists, it adds "_1", "_2", etc. before the extension until finding an available name.
// maxAttempts limits the number of attempts to find a unique name (default: 1000).
func (fs *FileSystem) GenerateUniqueFilename(path string, maxAttempts int) (string, error) {
	path = fs.crossPlatform.NormalizePath(path)

	// If file doesn't exist, return the original path
	if !fs.Exists(path) {
		return path, nil
	}

	// Set default max attempts if not provided or invalid
	if maxAttempts <= 0 {
		maxAttempts = 1000
	}

	// Split the path into directory, base name, and extension
	dir := fs.GetDirName(path)
	baseName := fs.GetBaseName(path)
	ext := fs.GetExtension(path)

	// Try to find a unique filename by adding a counter suffix
	counter := 1
	var newPath string

	for counter <= maxAttempts {
		newFileName := fmt.Sprintf("%s_%d%s", baseName, counter, ext)
		newPath = fs.JoinPath(dir, newFileName)

		if !fs.Exists(newPath) {
			return newPath, nil
		}

		counter++
	}

	return "", fmt.Errorf("failed to generate unique filename after %d attempts", maxAttempts)
}
