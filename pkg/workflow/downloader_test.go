package workflow

import (
	"testing"
)

func TestIsValidURL(t *testing.T) {
	// Save original AllowedDomains and restore at end
	originalAllowed := AllowedDomains
	defer func() {
		AllowedDomains = originalAllowed
	}()

	// Set test allowed domains
	AllowedDomains = []string{"github.com", "api.example.com"}

	// Create a downloader for testing
	downloader, err := NewWorkflowDownloader()
	if err != nil {
		t.Fatalf("Failed to create workflow downloader: %v", err)
	}

	testCases := []struct {
		name        string
		url         string
		expectError bool
	}{
		// Allowed domains
		{"GitHub main domain", "https://github.com/user/repo/blob/main/file.js", false},
		{"GitHub subdomain", "https://api.github.com/repos/user/repo", false},
		{"GitHub nested subdomain", "https://docs.api.github.com/v3/guides", false},
		{"Example API domain", "https://api.example.com/v1/data", false},
		{"Example API subdomain", "https://v2.api.example.com/data", false},

		// Disallowed domains
		{"Generic example.com", "https://example.com/file.js", true},
		{"Similar but not subdomain", "https://mygithub.com/file.js", true},
		{"Similar suffix", "https://github.company.com/file.js", true},

		// Invalid schemes
		{"FTP URL", "ftp://github.com/file.js", true},
		{"File URL", "file:///home/user/file.js", true},

		// Invalid formats
		{"Invalid format", "not-a-url", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := downloader.IsValidURL(tc.url)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for URL %q, but got none", tc.url)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error for URL %q, but got: %v", tc.url, err)
			}
		})
	}
}

func TestConvertToRawURL(t *testing.T) {
	downloader, err := NewWorkflowDownloader()
	if err != nil {
		t.Fatalf("Failed to create workflow downloader: %v", err)
	}

	testCases := []struct {
		name           string
		inputURL       string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "GitHub blob URL",
			inputURL:       "https://github.com/user/repo/blob/main/file.js",
			expectedOutput: "https://raw.githubusercontent.com/user/repo/main/file.js",
			expectError:    false,
		},
		{
			name:           "GitHub already raw URL",
			inputURL:       "https://raw.githubusercontent.com/user/repo/main/file.js",
			expectedOutput: "https://raw.githubusercontent.com/user/repo/main/file.js",
			expectError:    false,
		},
		{
			name:           "GitLab blob URL",
			inputURL:       "https://gitlab.com/user/repo/-/blob/main/file.js",
			expectedOutput: "https://gitlab.com/user/repo/-/raw/main/file.js",
			expectError:    false,
		},
		{
			name:           "Non-convertible URL (still valid)",
			inputURL:       "https://example.com/file.js",
			expectedOutput: "https://example.com/file.js", // Should return as-is
			expectError:    false,
		},
		{
			name:           "Invalid URL format",
			inputURL:       "://invalid-url", // Truly invalid URL format
			expectedOutput: "",
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := downloader.ConvertToRawURL(tc.inputURL)

			if tc.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}
			if !tc.expectError && output != tc.expectedOutput {
				t.Errorf("Expected %q, but got %q", tc.expectedOutput, output)
			}
		})
	}
}

func TestPathBasedDomainValidation(t *testing.T) {
	// Save original allowed domains and restore after test
	originalDomains := AllowedDomains
	defer func() {
		AllowedDomains = originalDomains
	}()

	// Set test domains with path restrictions
	AllowedDomains = []string{
		"github.com/nodewee",
		"gitlab.com/special-org",
		"raw.githubusercontent.com",
	}

	wd, err := NewWorkflowDownloader()
	if err != nil {
		t.Fatalf("Failed to create workflow downloader: %v", err)
	}

	testCases := []struct {
		url       string
		shouldErr bool
		desc      string
	}{
		// Path restrictions should work
		{"https://github.com/nodewee/repo/workflow.js", false, "URL with path match should be allowed"},
		{"https://github.com/nodewee/another-repo/file.js", false, "Another URL with path match should be allowed"},
		{"https://github.com/other-user/repo/file.js", true, "URL with different path should be blocked"},

		// Domain without path should still work for any path
		{"https://raw.githubusercontent.com/any/path/file.js", false, "Domain without path restriction should match any path"},

		// Organization path restrictions
		{"https://gitlab.com/special-org/repo/file.js", false, "Organization-scoped URL should be allowed"},
		{"https://gitlab.com/different-org/file.js", true, "Different organization URL should be blocked"},

		// Subdomains with paths
		{"https://subdomain.github.com/nodewee/repo/file.js", false, "Subdomain with matching path should be allowed"},
		{"https://subdomain.github.com/other-path/file.js", true, "Subdomain with non-matching path should be blocked"},
	}

	for _, tc := range testCases {
		err := wd.IsValidURL(tc.url)
		if tc.shouldErr && err == nil {
			t.Errorf("%s: Expected URL %s to be blocked, but it was allowed", tc.desc, tc.url)
		} else if !tc.shouldErr && err != nil {
			t.Errorf("%s: Expected URL %s to be allowed, but got error: %v", tc.desc, tc.url, err)
		}
	}
}
