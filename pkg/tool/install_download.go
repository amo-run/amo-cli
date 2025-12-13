package tool

import (
	"archive/zip"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"amo/pkg/network"
)

func (m *Manager) downloadFile(url string) (string, error) {
	tempDir := m.environment.GetCrossPlatformUtils().GetTempDir()
	base := filepath.Base(url)
	if base == "." || base == "/" || base == "" {
		base = "download.bin"
	}
	h := sha1.Sum([]byte(url))
	short := fmt.Sprintf("%x", h)[:10]
	safeBase := sanitizeFilename(base)
	tempPath := filepath.Join(tempDir, "amo-"+safeBase+"-"+short)

	nc, err := network.NewNetworkClient()
	if err != nil {
		return "", fmt.Errorf("failed to init network client: %w", err)
	}

	var lastPercent = -1
	resp := nc.DownloadFileResume(url, tempPath, func(p network.DownloadProgress) {
		var totalStr string
		if p.Total > 0 {
			totalStr = "/" + formatBytes(p.Total)
		}
		if p.Total > 0 {
			if p.Percentage != lastPercent {
				fmt.Printf("\r⬇️  Downloading... %3d%% (%s%s) - %s", p.Percentage, formatBytes(p.Downloaded), totalStr, p.Speed)
				lastPercent = p.Percentage
			}
		} else {
			fmt.Printf("\r⬇️  Downloading... %s%s - %s", formatBytes(p.Downloaded), totalStr, p.Speed)
		}
	})
	if resp.Error != "" {
		fmt.Println()
		return "", fmt.Errorf("%s", resp.Error)
	}
	fmt.Println()
	return tempPath, nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	name = replacer.Replace(name)
	if len(name) > 128 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		if len(base) > 120-len(ext) {
			base = base[:120-len(ext)]
		}
		name = base + ext
	}
	if strings.TrimSpace(name) == "" {
		return "download.bin"
	}
	return name
}

func (m *Manager) installDownloadedFile(sourcePath, targetPath, originalName string) error {
	if strings.HasSuffix(strings.ToLower(originalName), ".zip") {
		return m.extractAndInstallZip(sourcePath, targetPath, originalName)
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(targetPath, 0755); err != nil {
			return fmt.Errorf("failed to make file executable: %w", err)
		}
	}

	return nil
}

func (m *Manager) extractAndInstallZip(zipPath, targetPath, originalName string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	var executableFile *zip.File
	targetBaseName := strings.TrimSuffix(filepath.Base(targetPath), filepath.Ext(filepath.Base(targetPath)))

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		fileName := filepath.Base(file.Name)
		fileNameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		if strings.EqualFold(fileNameWithoutExt, targetBaseName) ||
			(runtime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(fileName), ".exe")) ||
			(runtime.GOOS != "windows" && (file.FileInfo().Mode()&0111) != 0) {
			executableFile = file
			break
		}
	}

	if executableFile == nil {
		return fmt.Errorf("no executable file found in zip archive")
	}

	srcFile, err := executableFile.Open()
	if err != nil {
		return fmt.Errorf("failed to open file from zip: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(targetPath, 0755); err != nil {
			return fmt.Errorf("failed to make file executable: %w", err)
		}
	}

	return nil
}
