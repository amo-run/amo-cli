package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var DefaultAllowedDomains = []string{
	"github.com",
	"raw.githubusercontent.com",
	"gitlab.com",
	"bitbucket.org",
	"sourceforge.net",
	"toolchains.mirror.toulan.fun",
}

var AllowedDomains = append([]string(nil), DefaultAllowedDomains...)

const AllowedSourcesFileName = "allowed_workflow_hosts.txt"

func (wd *WorkflowDownloader) GetAllowedSourcesFilePath() string {
	return wd.env.GetCrossPlatformUtils().JoinPath(wd.env.GetUserConfigDir(), AllowedSourcesFileName)
}

func (wd *WorkflowDownloader) EnsureAllowedSourcesFile() error {
	filePath := wd.GetAllowedSourcesFilePath()

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		builder := &strings.Builder{}
		builder.WriteString("# Allowed workflow download sources - one domain or domain/path per line\n")
		builder.WriteString("# Matching rules:\n")
		builder.WriteString("# - 'github.com' allows github.com itself and any subdomain like api.github.com\n")
		builder.WriteString("# - 'raw.githubusercontent.com' allows itself and subdomains\n")
		builder.WriteString("# - 'github.com/owner' restricts to that owner only (and any subdomains)\n")
		builder.WriteString("# - 'api.github.com/v3' restricts to that path and below\n")
		builder.WriteString("# Lines starting with '#' are comments and ignored\n\n")

		for _, host := range DefaultAllowedDomains {
			builder.WriteString(host)
			builder.WriteString("\n")
		}

		return wd.env.GetCrossPlatformUtils().CreateFileWithPermissions(filePath, []byte(builder.String()), false)
	}

	return nil
}

func (wd *WorkflowDownloader) LoadAllowedSources() ([]string, error) {
	filePath := wd.GetAllowedSourcesFilePath()
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var entries []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		entries = append(entries, trimmed)
	}
	return entries, nil
}

func (wd *WorkflowDownloader) SaveAllowedSources(entries []string) error {
	filePath := wd.GetAllowedSourcesFilePath()
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	builder := &strings.Builder{}
	builder.WriteString("# Allowed workflow download sources - one domain or domain/path per line\n")
	builder.WriteString("# See comments above for matching rules.\n\n")

	seen := make(map[string]struct{})
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" || strings.HasPrefix(e, "#") {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		builder.WriteString(e)
		builder.WriteString("\n")
	}

	return wd.env.GetCrossPlatformUtils().CreateFileWithPermissions(filePath, []byte(builder.String()), false)
}

func (wd *WorkflowDownloader) ListAllowedSources() ([]string, error) {
	if err := wd.EnsureAllowedSourcesFile(); err != nil {
		return nil, err
	}
	entries, err := wd.LoadAllowedSources()
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	return entries, nil
}

func (wd *WorkflowDownloader) AddAllowedSource(entry string) (bool, error) {
	if err := wd.EnsureAllowedSourcesFile(); err != nil {
		return false, err
	}
	entry = strings.TrimSpace(strings.ToLower(entry))
	if entry == "" || strings.HasPrefix(entry, "#") {
		return false, fmt.Errorf("invalid source entry")
	}

	entries, err := wd.LoadAllowedSources()
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e), entry) {
			return false, nil
		}
	}
	entries = append(entries, entry)
	if err := wd.SaveAllowedSources(entries); err != nil {
		return false, err
	}
	return true, nil
}

func (wd *WorkflowDownloader) RemoveAllowedSource(entry string) (bool, error) {
	if err := wd.EnsureAllowedSourcesFile(); err != nil {
		return false, err
	}
	entry = strings.TrimSpace(strings.ToLower(entry))
	if entry == "" || strings.HasPrefix(entry, "#") {
		return false, fmt.Errorf("invalid source entry")
	}

	entries, err := wd.LoadAllowedSources()
	if err != nil {
		return false, err
	}
	var updated []string
	removed := false
	for _, e := range entries {
		normalized := strings.TrimSpace(strings.ToLower(e))
		if normalized == entry {
			removed = true
			continue
		}
		updated = append(updated, e)
	}
	if !removed {
		return false, nil
	}
	if err := wd.SaveAllowedSources(updated); err != nil {
		return false, err
	}
	return true, nil
}
