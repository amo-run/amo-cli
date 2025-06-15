package network

import (
	"testing"

	"amo/pkg/env"
)

func TestIsURLAllowed(t *testing.T) {
	// Create a minimal network client for testing
	environment, _ := env.NewEnvironment()
	client := &NetworkClient{
		client:         nil, // Not needed for this test
		environment:    environment,
		allowedSchemes: []string{"https", "http"},
		allowedHosts:   []string{"github.com", "api.example.com", "sourceforge.net"},
	}

	testCases := []struct {
		name     string
		url      string
		expected bool
	}{
		// Exact matches
		{"Exact match - github.com", "https://github.com", true},
		{"Exact match - api.example.com", "https://api.example.com", true},

		// Subdomain matches
		{"Subdomain match - api.github.com", "https://api.github.com", true},
		{"Subdomain match - v3.api.github.com", "https://v3.api.github.com", true},
		{"Subdomain match - docs.api.example.com", "https://docs.api.example.com", true},

		// Disallowed domains
		{"Disallowed domain - example.com", "https://example.com", false},
		{"Disallowed with similar prefix - mygithub.com", "https://mygithub.com", false},
		{"Disallowed with similar suffix - github.company.com", "https://github.company.com", false},

		// Scheme checks
		{"Disallowed scheme - ftp", "ftp://github.com", false},

		// Invalid URLs
		{"Invalid URL - no scheme", "github.com", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := client.isURLAllowed(tc.url)
			if result != tc.expected {
				t.Errorf("isURLAllowed(%q) = %v; expected %v", tc.url, result, tc.expected)
			}
		})
	}
}

func TestIsURLAllowedWithNoHosts(t *testing.T) {
	// Test special case: no hosts configured (should allow all)
	environment, _ := env.NewEnvironment()
	client := &NetworkClient{
		client:         nil, // Not needed for this test
		environment:    environment,
		allowedSchemes: []string{"https", "http"},
		allowedHosts:   []string{}, // Empty hosts list
	}

	// Should allow any domain when no hosts configured
	result := client.isURLAllowed("https://any-domain-should-work.com")
	if !result {
		t.Error("Expected URL to be allowed when no hosts are configured")
	}

	// Should still reject invalid schemes
	result = client.isURLAllowed("ftp://github.com")
	if result {
		t.Error("Expected FTP URL to be rejected even with no hosts configured")
	}
}

func TestExtractHeaders(t *testing.T) {
	// Create a minimal network client for testing
	environment, _ := env.NewEnvironment()
	client := &NetworkClient{
		environment: environment,
	}

	// Create test HTTP headers
	httpHeaders := make(map[string][]string)
	httpHeaders["Content-Type"] = []string{"application/json"}
	httpHeaders["X-Multiple"] = []string{"value1", "value2", "value3"}

	// Call the function
	result := client.extractHeaders(httpHeaders)

	// Verify results
	if result["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type to be 'application/json', got %q", result["Content-Type"])
	}

	// Should use only the first value for headers with multiple values
	if result["X-Multiple"] != "value1" {
		t.Errorf("Expected X-Multiple to be 'value1', got %q", result["X-Multiple"])
	}
}

func TestPathBasedWhitelist(t *testing.T) {
	nc := &NetworkClient{
		allowedSchemes: []string{"https", "http"},
		allowedHosts:   []string{"github.com/nodewee", "api.github.com/v3", "example.com"},
	}

	// Test cases for path-based whitelist
	testCases := []struct {
		url     string
		allowed bool
		desc    string
	}{
		// Path restrictions should work
		{"https://github.com/nodewee/repo", true, "URL with exact path match should be allowed"},
		{"https://github.com/nodewee/repo/subdir", true, "URL with path prefix match should be allowed"},
		{"https://github.com/other-user/repo", false, "URL with different path should be blocked"},
		{"https://github.com/nodeweexyz", false, "URL with similar but different path should be blocked"},

		// API version path restrictions
		{"https://api.github.com/v3/users", true, "API URL with path version match should be allowed"},
		{"https://api.github.com/v4/users", false, "API URL with different version should be blocked"},

		// Domain without path should still work for any path
		{"https://example.com", true, "Domain without path restriction should match any path"},
		{"https://example.com/any/path", true, "Domain without path restriction should match any path"},

		// Subdomains should work with path restrictions
		{"https://sub.github.com/nodewee/repo", true, "Subdomain with path match should be allowed"},
		{"https://sub.github.com/other-path", false, "Subdomain with non-matching path should be blocked"},
	}

	for _, tc := range testCases {
		allowed := nc.isURLAllowed(tc.url)
		if allowed != tc.allowed {
			t.Errorf("%s: URL %s should be %s, but got %v",
				tc.desc, tc.url,
				map[bool]string{true: "allowed", false: "blocked"}[tc.allowed],
				allowed)
		}
	}
}
