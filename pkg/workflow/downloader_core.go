package workflow

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"amo/pkg/env"
	"amo/pkg/network"

	"github.com/spf13/viper"
)

type WorkflowDownloader struct {
	env    *env.Environment
	client *http.Client
}

func NewWorkflowDownloader() (*WorkflowDownloader, error) {
	environment, err := env.NewEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize environment: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &WorkflowDownloader{
		env:    environment,
		client: client,
	}, nil
}

func (wd *WorkflowDownloader) GetWorkflowsDir() string {
	return wd.env.GetCrossPlatformUtils().JoinPath(wd.env.GetUserConfigDir(), "workflows")
}

func (wd *WorkflowDownloader) EnsureWorkflowsDir() error {
	workflowsDir := wd.GetWorkflowsDir()
	return wd.env.GetCrossPlatformUtils().CreateDirWithPermissions(workflowsDir)
}

func (wd *WorkflowDownloader) GetConfiguredWorkflowsDir() string {
	configuredDir := os.Getenv("AMO_WORKFLOWS_DIR")
	if configuredDir != "" {
		return wd.env.GetCrossPlatformUtils().NormalizePath(configuredDir)
	}

	configDir := wd.env.GetUserConfigDir()
	configFile := filepath.Join(configDir, "config.yaml")

	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("yaml")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return ""
	}

	if err := v.ReadInConfig(); err != nil {
		return ""
	}

	configuredDir = v.GetString("workflows")
	if configuredDir != "" {
		return wd.env.GetCrossPlatformUtils().NormalizePath(configuredDir)
	}

	return ""
}

func (wd *WorkflowDownloader) DownloadWorkflow(urlStr string, filename string) error {
	if err := wd.IsValidURL(urlStr); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	rawURL, err := wd.ConvertToRawURL(urlStr)
	if err != nil {
		return fmt.Errorf("failed to convert URL: %w", err)
	}

	if filename == "" {
		filename, err = wd.ExtractFilename(rawURL)
		if err != nil {
			return fmt.Errorf("failed to extract filename: %w", err)
		}
	} else {
		filename = wd.sanitizeFilename(filename)
		if !strings.HasSuffix(strings.ToLower(filename), ".js") {
			filename += ".js"
		}
	}

	workflowsDir := wd.GetWorkflowsDir()

	if err := wd.EnsureWorkflowsDir(); err != nil {
		return fmt.Errorf("failed to create workflows directory: %w", err)
	}

	tempName := wd.buildTempName(filename, rawURL) + ".download"
	tempPath := wd.env.GetCrossPlatformUtils().JoinPath(workflowsDir, tempName)

	if err := wd.downloadToFileWithResume(rawURL, tempPath); err != nil {
		fmt.Printf("⚠️  Original URL failed: %v\n", err)

		parsedURL, parseErr := url.Parse(rawURL)
		if parseErr == nil && wd.isGitHubURL(parsedURL) {
			fmt.Printf("🔄 Trying mirror site: toolchains.mirror.toulan.fun\n")
			mirrorURL, mirrorErr := wd.convertToMirrorURL(rawURL)
			if mirrorErr == nil {
				if err2 := wd.downloadToFileWithResume(mirrorURL, tempPath); err2 != nil {
					return fmt.Errorf("both original and mirror download failed: original=%v, mirror=%v", err, err2)
				}
				fmt.Printf("✅ Successfully downloaded from mirror site\n")
			} else {
				return fmt.Errorf("original download failed and mirror URL conversion failed: original=%v, mirror=%v", err, mirrorErr)
			}
		} else {
			return fmt.Errorf("download failed: %w", err)
		}
	}

	fileBytes, readErr := os.ReadFile(tempPath)
	if readErr != nil {
		return fmt.Errorf("failed to read downloaded file: %w", readErr)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(fileBytes)), "//!amo") {
		_ = os.Remove(tempPath)
		return fmt.Errorf("downloaded file is not a valid amo workflow (must start with //!amo)")
	}

	workflowPath := wd.env.GetCrossPlatformUtils().JoinPath(workflowsDir, filename)
	if err := os.Rename(tempPath, workflowPath); err != nil {
		if copyErr := os.WriteFile(workflowPath, fileBytes, 0644); copyErr != nil {
			return fmt.Errorf("failed to save workflow file: %w", copyErr)
		}
		_ = os.Remove(tempPath)
	}

	return nil
}

func (wd *WorkflowDownloader) downloadFromURL(urlStr string) ([]byte, error) {
	resp, err := wd.client.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	contentLength := resp.ContentLength
	var downloaded int64
	buffer := make([]byte, 32*1024)
	startTime := time.Now()
	lastReport := startTime

	var out []byte
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			out = append(out, buffer[:n]...)
			downloaded += int64(n)

			now := time.Now()
			if now.Sub(lastReport) >= 200*time.Millisecond || (contentLength > 0 && downloaded == contentLength) {
				elapsed := now.Sub(startTime)
				if elapsed <= 0 {
					elapsed = time.Millisecond
				}
				speed := float64(downloaded) / elapsed.Seconds()
				if contentLength > 0 {
					percentage := int(float64(downloaded) / float64(contentLength) * 100)
					fmt.Printf("\r⬇️  Fetching script... %3d%% (%s/%s) - %s",
						percentage,
						formatBytes(downloaded),
						formatBytes(contentLength),
						formatBytes(int64(speed))+"/s",
					)
				} else {
					fmt.Printf("\r⬇️  Fetching script... %s - %s",
						formatBytes(downloaded),
						formatBytes(int64(speed))+"/s",
					)
				}
				lastReport = now
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("failed to read response body: %w", readErr)
		}
	}

	if downloaded > 0 {
		fmt.Println()
	}

	return out, nil
}

func (wd *WorkflowDownloader) downloadToFileWithResume(urlStr, outputPath string) error {
	nc, err := network.NewNetworkClient()
	if err != nil {
		return fmt.Errorf("failed to init network client: %w", err)
	}

	var lastPercent = -1
	resp := nc.DownloadFileResume(urlStr, outputPath, func(p network.DownloadProgress) {
		if p.Total > 0 {
			if p.Percentage != lastPercent {
				fmt.Printf("\r⬇️  Fetching script... %3d%% (%s/%s) - %s",
					p.Percentage,
					formatBytes(p.Downloaded),
					formatBytes(p.Total),
					p.Speed,
				)
				lastPercent = p.Percentage
			}
		} else {
			fmt.Printf("\r⬇️  Fetching script... %s - %s",
				formatBytes(p.Downloaded),
				p.Speed,
			)
		}
	})
	if resp.Error != "" {
		fmt.Println()
		return fmt.Errorf("%s", resp.Error)
	}
	fmt.Println()
	return nil
}

func (wd *WorkflowDownloader) buildTempName(filename, urlStr string) string {
	h := sha1.Sum([]byte(urlStr))
	short := fmt.Sprintf("%x", h)[:10]
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	if name == "" {
		name = "workflow"
	}
	return name + "-" + short
}

func (wd *WorkflowDownloader) ListUserWorkflows() ([]string, error) {
	workflowMap := make(map[string]bool)
	var err1, err2 error

	defaultWorkflowsDir := wd.GetWorkflowsDir()
	if _, statErr := os.Stat(defaultWorkflowsDir); !os.IsNotExist(statErr) {
		entries, err := os.ReadDir(defaultWorkflowsDir)
		if err != nil {
			err1 = fmt.Errorf("failed to read default workflows directory: %w", err)
		} else {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".js") {
					workflowMap[entry.Name()] = true
				}
			}
		}
	}

	configuredDir := wd.GetConfiguredWorkflowsDir()
	if configuredDir != "" && configuredDir != defaultWorkflowsDir {
		if _, statErr := os.Stat(configuredDir); !os.IsNotExist(statErr) {
			entries, err := os.ReadDir(configuredDir)
			if err != nil {
				err2 = fmt.Errorf("failed to read configured workflows directory: %w", err)
			} else {
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".js") {
						workflowMap[entry.Name()] = true
					}
				}
			}
		}
	}

	if err1 != nil && err2 != nil {
		return nil, fmt.Errorf("failed to list workflows: %v; %v", err1, err2)
	} else if err1 != nil {
		return nil, err1
	} else if err2 != nil {
		return nil, err2
	}

	workflows := make([]string, 0, len(workflowMap))
	for workflow := range workflowMap {
		workflows = append(workflows, workflow)
	}

	sort.Strings(workflows)

	return workflows, nil
}

