package tool

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"amo/pkg/network"
)

// installViaHomebrew installs a tool using Homebrew
func (m *Manager) installViaHomebrew(packageName string) error {
	if _, err := exec.LookPath("brew"); err != nil {
		return fmt.Errorf("homebrew not found, install from: https://brew.sh/")
	}

	cmd := exec.Command("brew", "install", packageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("homebrew installation failed: %v", err)
	}

	return nil
}

// installViaPackageManager installs a tool using system package manager
func (m *Manager) installViaPackageManager(packages map[string]string) error {
	packageManagers := []string{"apt", "yum", "pacman"}

	for _, pm := range packageManagers {
		packageName, exists := packages[pm]
		if !exists {
			continue
		}

		if _, err := exec.LookPath(pm); err != nil {
			continue
		}

		var cmd *exec.Cmd
		switch pm {
		case "apt":
			cmd = exec.Command("sudo", "apt", "update")
			cmd.Run()
			cmd = exec.Command("sudo", "apt", "install", "-y", packageName)
		case "yum":
			cmd = exec.Command("sudo", "yum", "install", "-y", packageName)
		case "pacman":
			cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", packageName)
		default:
			continue
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s installation failed: %v", pm, err)
		}

		return nil
	}

	return fmt.Errorf("no suitable package manager found or allowed")
}

// installViaPip installs a tool using pip
func (m *Manager) installViaPip(packageName string) error {
	pipCommands := []string{"pip3", "pip"}

	for _, pip := range pipCommands {
		if _, err := exec.LookPath(pip); err != nil {
			continue
		}

		cmd := exec.Command(pip, "install", packageName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pip installation failed: %v", err)
		}

		return nil
	}

	return fmt.Errorf("pip not found")
}

// installViaGitHub installs a tool from GitHub releases with mirror fallback
func (m *Manager) installViaGitHub(toolName string, installInfo InstallInfo) error {
	fmt.Printf("📦 Installing %s from GitHub repository: %s\n", toolName, installInfo.Repo)

	installDir := m.getInstallDir()
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	if m.preferMirror {
		fmt.Printf("🔁 Prefer mirror is enabled, trying mirror first\n")
		if err := m.installFromMirror(toolName, installInfo, installDir); err != nil {
			fmt.Printf("⚠️  Mirror installation failed: %v\n", err)
			fmt.Printf("🔄 Falling back to GitHub\n")
			if err2 := m.installFromGitHub(toolName, installInfo, installDir); err2 != nil {
				fmt.Printf("❌ GitHub installation also failed: %v\n", err2)
				fmt.Printf("💡 Manual installation steps:\n")
				m.printManualInstallInstructions(toolName, installInfo)
				return fmt.Errorf("both mirror and GitHub installation failed: %w", err2)
			}
		}
		return nil
	}

	err := m.installFromGitHub(toolName, installInfo, installDir)
	if err != nil {
		fmt.Printf("⚠️  GitHub installation failed: %v\n", err)
		fmt.Printf("🔄 Trying mirror site: toolchains.mirror.toulan.fun\n")

		err = m.installFromMirror(toolName, installInfo, installDir)
		if err != nil {
			fmt.Printf("❌ Mirror installation also failed: %v\n", err)
			fmt.Printf("💡 Manual installation steps:\n")
			m.printManualInstallInstructions(toolName, installInfo)
			return fmt.Errorf("both GitHub and mirror installation failed: %w", err)
		}
	}

	return nil
}

// installFromGitHub attempts to install from GitHub releases
func (m *Manager) installFromGitHub(toolName string, installInfo InstallInfo, installDir string) error {
	release, err := m.getLatestGitHubRelease(installInfo.Repo)
	if err != nil {
		return fmt.Errorf("failed to get GitHub release info: %w", err)
	}

	assetName := m.expandPattern(installInfo.Pattern, release.TagName)
	asset := m.findMatchingAsset(release.Assets, assetName)
	if asset == nil {
		fmt.Printf("⚠️  Available GitHub assets:\n")
		for _, a := range release.Assets {
			fmt.Printf("   - %s\n", a.Name)
		}
		return fmt.Errorf("no matching asset found for pattern: %s", assetName)
	}

	fmt.Printf("📥 Downloading from GitHub: %s (version %s)\n", asset.Name, release.TagName)

	tempFile, err := m.downloadFile(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("GitHub download failed: %w", err)
	}
	defer os.Remove(tempFile)

	targetName := installInfo.Target
	if targetName == "" {
		targetName = toolName
		if runtime.GOOS == "windows" {
			targetName += ".exe"
		}
	}

	targetPath := filepath.Join(installDir, targetName)

	if err := m.installDownloadedFile(tempFile, targetPath, asset.Name); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	m.setCachedToolPath(toolName, targetPath)
	if err := m.savePathCache(); err != nil {
		fmt.Printf("⚠️  Warning: Failed to save path cache: %v\n", err)
	}

	fmt.Printf("✅ %s installed successfully from GitHub to: %s\n", toolName, targetPath)
	return nil
}

// installFromMirror attempts to install from mirror site
func (m *Manager) installFromMirror(toolName string, installInfo InstallInfo, installDir string) error {
	possibleVersions := []string{"latest"}

	if release, err := m.getLatestGitHubRelease(installInfo.Repo); err == nil && release != nil {
		tag := strings.TrimSpace(release.TagName)
		if tag != "" && tag != "latest" {
			possibleVersions = append(possibleVersions, tag)
			noV := strings.TrimPrefix(tag, "v")
			if noV != tag {
				possibleVersions = append(possibleVersions, noV)
			}
		}
	} else {
		fmt.Printf("ℹ️  Unable to query GitHub for latest tag; trying 'latest' on mirror only\n")
	}

	var tempFile string
	var finalAssetName string
	var downloadErr error

	for _, version := range possibleVersions {
		testAssetName := m.expandPattern(installInfo.Pattern, version)
		mirrorURL := fmt.Sprintf("https://toolchains.mirror.toulan.fun/%s/%s/%s",
			installInfo.Repo, version, testAssetName)

		fmt.Printf("📥 Trying mirror download: %s\n", mirrorURL)

		tempFile, downloadErr = m.downloadFile(mirrorURL)
		if downloadErr == nil {
			finalAssetName = testAssetName
			break
		}

		fmt.Printf("⚠️  Mirror URL failed: %v\n", downloadErr)
	}

	if downloadErr != nil {
		return fmt.Errorf("mirror download failed for all versions: %w", downloadErr)
	}

	defer os.Remove(tempFile)

	targetName := installInfo.Target
	if targetName == "" {
		targetName = toolName
		if runtime.GOOS == "windows" {
			targetName += ".exe"
		}
	}

	targetPath := filepath.Join(installDir, targetName)

	if err := m.installDownloadedFile(tempFile, targetPath, finalAssetName); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	m.setCachedToolPath(toolName, targetPath)
	if err := m.savePathCache(); err != nil {
		fmt.Printf("⚠️  Warning: Failed to save path cache: %v\n", err)
	}

	fmt.Printf("✅ %s installed successfully from mirror to: %s\n", toolName, targetPath)
	return nil
}

// installViaDownload installs a tool via direct download
func (m *Manager) installViaDownload(toolName string, installInfo InstallInfo) error {
	if strings.TrimSpace(installInfo.URL) == "" {
		return fmt.Errorf("no download URL specified. Provide --url to specify the installer or binary source")
	}
	fmt.Printf("📦 Installing %s via download from: %s\n", toolName, installInfo.URL)

	installDir := m.getInstallDir()
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	tempFile, err := m.downloadFile(installInfo.URL)
	if err != nil {
		fmt.Printf("❌ Download failed: %v\n", err)
		fmt.Printf("💡 Please download manually from: %s\n", installInfo.URL)
		fmt.Printf("   Install to: %s\n", installDir)
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tempFile)

	targetName := installInfo.Target
	if targetName == "" {
		targetName = toolName
		if runtime.GOOS == "windows" {
			targetName += ".exe"
		}
	}

	targetPath := filepath.Join(installDir, targetName)
	filename := filepath.Base(installInfo.URL)

	if err := m.installDownloadedFile(tempFile, targetPath, filename); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	m.setCachedToolPath(toolName, targetPath)
	if err := m.savePathCache(); err != nil {
		fmt.Printf("⚠️  Warning: Failed to save path cache: %v\n", err)
	}

	fmt.Printf("✅ %s installed successfully to: %s\n", toolName, targetPath)
	return nil
}

// installViaInstaller handles installer downloads
func (m *Manager) installViaInstaller(installInfo InstallInfo) error {
	if strings.TrimSpace(installInfo.URL) == "" {
		return fmt.Errorf("no installer URL specified. Provide --url to open a specific installer page")
	}
	fmt.Printf("📦 Opening installer download page: %s\n", installInfo.URL)
	fmt.Printf("💡 Please download and run the installer manually\n")
	fmt.Printf("   After installation, the tool should be available in your PATH\n")

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", installInfo.URL)
	case "linux":
		cmd = exec.Command("xdg-open", installInfo.URL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", installInfo.URL)
	default:
		fmt.Printf("   URL: %s\n", installInfo.URL)
		return nil
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("   Failed to open browser, please visit: %s\n", installInfo.URL)
	}

	return fmt.Errorf("manual installation required")
}

// installViaWorkflow runs a workflow to install the tool
func (m *Manager) installViaWorkflow(toolName string, installInfo InstallInfo) error {
	workflowName := installInfo.Workflow
	if workflowName == "" {
		return fmt.Errorf("no workflow specified for tool: %s", toolName)
	}

	workflowEngine, err := m.getWorkflowEngine()
	if err != nil {
		return fmt.Errorf("failed to get workflow engine: %w", err)
	}

	downloadURL := installInfo.URL
	if m.isChinaRegion() && installInfo.MirrorURL != "" {
		downloadURL = installInfo.MirrorURL
		fmt.Printf("Using mirror URL for China region: %s\n", downloadURL)
	}

	params := map[string]interface{}{
		"toolName":      toolName,
		"installDir":    m.getInstallDir(),
		"targetFile":    installInfo.Target,
		"portableUrl":   installInfo.PortableURL,
		"mirrorUrl":     installInfo.MirrorURL,
		"url":           downloadURL,
		"pattern":       installInfo.Pattern,
		"isChinaRegion": m.isChinaRegion(),
	}

	fmt.Printf("🔄 Running installation workflow: %s\n", workflowName)
	result, err := workflowEngine.RunWorkflow(workflowName, params)
	if err != nil {
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		if errorMsg, ok := result["error"].(string); ok {
			return fmt.Errorf("workflow installation failed: %s", errorMsg)
		}
		return fmt.Errorf("workflow installation failed: unknown error")
	}

	fmt.Printf("✅ Workflow completed successfully\n")
	return nil
}

// getWorkflowEngine gets or creates the workflow engine instance
func (m *Manager) getWorkflowEngine() (WorkflowEngine, error) {
	if m.workflowEngine != nil {
		return m.workflowEngine, nil
	}

	return nil, fmt.Errorf("workflow engine not initialized")
}

// isChinaRegion detects if the user is in China region
func (m *Manager) isChinaRegion() bool {
	if lang := os.Getenv("LANG"); lang != "" {
		if strings.Contains(lang, "zh_CN") || strings.Contains(lang, "zh-CN") {
			return true
		}
	}

	if lc := os.Getenv("LC_ALL"); lc != "" {
		if strings.Contains(lc, "zh_CN") || strings.Contains(lc, "zh-CN") {
			return true
		}
	}

	if tz := os.Getenv("TZ"); tz != "" {
		if strings.Contains(tz, "Shanghai") || strings.Contains(tz, "Beijing") || strings.Contains(tz, "Chongqing") {
			return true
		}
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-Command", "(Get-WinSystemLocale).Name")
		if output, err := cmd.Output(); err == nil {
			locale := strings.TrimSpace(string(output))
			if strings.HasPrefix(locale, "zh-CN") {
				return true
			}
		}
	}

	if os.Getenv("CHINA_MIRROR") == "true" || os.Getenv("AMO_USE_CHINA_MIRROR") == "true" {
		return true
	}

	return false
}

// getLatestGitHubRelease gets the latest release from a GitHub repository
func (m *Manager) getLatestGitHubRelease(repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

// expandPattern expands placeholders in asset filename patterns
func (m *Manager) expandPattern(pattern, version string) string {
	result := pattern

	result = strings.ReplaceAll(result, "{version}", strings.TrimPrefix(version, "v"))

	arch := runtime.GOARCH
	if arch == "amd64" {
		if strings.Contains(pattern, "x86_64") {
			arch = "x86_64"
		}
	}
	result = strings.ReplaceAll(result, "{arch}", arch)

	return result
}

// findMatchingAsset finds an asset that matches the given name pattern
func (m *Manager) findMatchingAsset(assets []GitHubReleaseAsset, pattern string) *GitHubReleaseAsset {
	for _, asset := range assets {
		if asset.Name == pattern {
			return &asset
		}
	}

	pattern = strings.ToLower(pattern)
	for _, asset := range assets {
		if strings.ToLower(asset.Name) == pattern {
			return &asset
		}
	}

	for _, asset := range assets {
		if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(pattern)) {
			return &asset
		}
	}

	return nil
}

// downloadFile downloads a file from the given URL and returns the temporary file path
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

// formatBytes formats bytes into human readable string
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

// sanitizeFilename removes most problematic characters for building a temp filename
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

// installDownloadedFile installs a downloaded file to the target path
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

// extractAndInstallZip extracts a zip file and installs the binary
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

// printManualInstallInstructions prints manual installation instructions
func (m *Manager) printManualInstallInstructions(toolName string, installInfo InstallInfo) {
	installDir := m.getInstallDir()

	fmt.Printf("   1. Visit: https://github.com/%s/releases\n", installInfo.Repo)
	fmt.Printf("   2. Download the appropriate binary for your system:\n")

	switch runtime.GOOS {
	case "windows":
		fmt.Printf("      - Look for files containing 'windows' and 'amd64'\n")
		fmt.Printf("      - Example: %s\n", strings.ReplaceAll(installInfo.Pattern, "{arch}", "amd64"))
	case "darwin":
		fmt.Printf("      - Look for files containing 'darwin' and your architecture\n")
		if runtime.GOARCH == "arm64" {
			fmt.Printf("      - For Apple Silicon: %s\n", strings.ReplaceAll(installInfo.Pattern, "{arch}", "arm64"))
		} else {
			fmt.Printf("      - For Intel Mac: %s\n", strings.ReplaceAll(installInfo.Pattern, "{arch}", "amd64"))
		}
	case "linux":
		fmt.Printf("      - Look for files containing 'linux' and your architecture\n")
		fmt.Printf("      - Example: %s\n", strings.ReplaceAll(installInfo.Pattern, "{arch}", runtime.GOARCH))
	}

	fmt.Printf("   3. Create directory: %s\n", installDir)
	fmt.Printf("   4. Copy the downloaded binary to: %s\n", filepath.Join(installDir, toolName))
	if runtime.GOOS != "windows" {
		fmt.Printf("   5. Make it executable: chmod +x %s\n", filepath.Join(installDir, toolName))
	}
	fmt.Printf("   6. Add to PATH or run: amo tool cache clear (to re-detect)\n")
}

