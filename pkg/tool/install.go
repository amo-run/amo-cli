package tool

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
