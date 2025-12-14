package workflow

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

func (wd *WorkflowDownloader) IsValidURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTP and HTTPS URLs are allowed")
	}

	hostname := strings.ToLower(parsedURL.Hostname())

	overridden := false
	if len(AllowedDomains) != len(DefaultAllowedDomains) {
		overridden = true
	} else {
		for i := range AllowedDomains {
			if !strings.EqualFold(strings.TrimSpace(AllowedDomains[i]), strings.TrimSpace(DefaultAllowedDomains[i])) {
				overridden = true
				break
			}
		}
	}

	var allowedEntries []string
	if overridden {
		allowedEntries = append([]string(nil), AllowedDomains...)
	} else {
		if entries, loadErr := wd.LoadAllowedSources(); loadErr == nil {
			allowedEntries = entries
		} else {
			allowedEntries = append([]string(nil), AllowedDomains...)
		}
	}

	urlPath := parsedURL.Path

	for _, allowedEntry := range allowedEntries {
		hostPart := allowedEntry
		pathPart := ""

		if strings.Contains(allowedEntry, "/") {
			parts := strings.SplitN(allowedEntry, "/", 2)
			hostPart = parts[0]
			pathPart = "/" + parts[1]
		}

		hostnameMatches := false
		if hostname == hostPart {
			hostnameMatches = true
		} else if strings.HasSuffix(hostname, "."+hostPart) {
			hostnameMatches = true
		}

		if hostnameMatches {
			if pathPart == "" {
				return nil
			} else {
				if strings.HasPrefix(urlPath, pathPart) &&
					(len(urlPath) == len(pathPart) || urlPath[len(pathPart)] == '/' || pathPart[len(pathPart)-1] == '/') {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("URL with domain %s and path %s is not in the allowed list", hostname, urlPath)
}

func (wd *WorkflowDownloader) ConvertToRawURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	hostname := strings.ToLower(parsedURL.Hostname())

	if hostname == "github.com" {
		path := parsedURL.Path
		if strings.Contains(path, "/blob/") {
			path = strings.Replace(path, "/blob/", "/", 1)
			return fmt.Sprintf("https://raw.githubusercontent.com%s", path), nil
		}
		return urlStr, nil
	}

	if hostname == "gitlab.com" || strings.HasSuffix(hostname, ".gitlab.com") {
		path := parsedURL.Path
		if strings.Contains(path, "/-/blob/") {
			path = strings.Replace(path, "/-/blob/", "/-/raw/", 1)
			return fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, path), nil
		}
		return urlStr, nil
	}

	return urlStr, nil
}

func (wd *WorkflowDownloader) ExtractFilename(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	filename := filepath.Base(parsedURL.Path)
	if filename == "." || filename == "/" {
		return "", fmt.Errorf("could not extract filename from URL")
	}

	if !strings.HasSuffix(strings.ToLower(filename), ".js") {
		filename += ".js"
	}

	filename = wd.sanitizeFilename(filename)

	return filename, nil
}

func (wd *WorkflowDownloader) sanitizeFilename(filename string) string {
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	filename = reg.ReplaceAllString(filename, "_")

	filename = strings.Trim(filename, " .")

	if filename == "" {
		filename = "workflow.js"
	}

	return filename
}

func (wd *WorkflowDownloader) isGitHubURL(parsedURL *url.URL) bool {
	hostname := strings.ToLower(parsedURL.Hostname())
	return hostname == "github.com" || hostname == "raw.githubusercontent.com" || strings.HasSuffix(hostname, ".github.com")
}

func (wd *WorkflowDownloader) convertToMirrorURL(githubURL string) (string, error) {
	parsedURL, err := url.Parse(githubURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	path := parsedURL.Path

	if hostname == "raw.githubusercontent.com" {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 4 {
			owner := parts[0]
			repo := parts[1]
			filename := parts[len(parts)-1]

			mirrorURL := fmt.Sprintf("https://toolchains.mirror.toulan.fun/%s/%s/latest/%s",
				owner, repo, filename)
			return mirrorURL, nil
		}
	}

	if hostname == "github.com" && strings.Contains(path, "/blob/") {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 5 {
			owner := parts[0]
			repo := parts[1]
			filename := parts[len(parts)-1]

			mirrorURL := fmt.Sprintf("https://toolchains.mirror.toulan.fun/%s/%s/latest/%s",
				owner, repo, filename)
			return mirrorURL, nil
		}
	}

	return "", fmt.Errorf("unsupported GitHub URL format: %s", githubURL)
}
