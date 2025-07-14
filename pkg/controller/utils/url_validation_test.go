package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveTrailingSlash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no trailing slash",
			input:    "https://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "single trailing slash",
			input:    "https://example.com/path/",
			expected: "https://example.com/path",
		},
		{
			name:     "multiple trailing slashes",
			input:    "https://example.com/path///",
			expected: "https://example.com/path",
		},
		{
			name:     "many trailing slashes",
			input:    "https://example.com/path/////",
			expected: "https://example.com/path",
		},
		{
			name:     "root path with single slash - should preserve",
			input:    "/",
			expected: "/",
		},
		{
			name:     "root path with multiple slashes - should preserve one",
			input:    "///",
			expected: "/",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single character",
			input:    "a",
			expected: "a",
		},
		{
			name:     "single character with slash",
			input:    "a/",
			expected: "a",
		},
		{
			name:     "host only with single trailing slash",
			input:    "example.com/",
			expected: "example.com",
		},
		{
			name:     "host only with multiple trailing slashes",
			input:    "example.com///",
			expected: "example.com",
		},
		{
			name:     "URL with path and multiple trailing slashes",
			input:    "example.com/api/v1////",
			expected: "example.com/api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeTrailingSlash(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidJWTIssuerURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:  "valid HTTPS URL with lowercase domain",
			input: "https://example.com",
		},
		{
			name:  "valid HTTPS URL with uppercase domain",
			input: "https://EXAMPLE.COM",
		},
		{
			name:  "valid HTTPS URL with mixed case domain",
			input: "https://Example.COM",
		},
		{
			name:  "valid HTTPS URL with path",
			input: "https://EXAMPLE.com/Path/To/Resource",
		},
		{
			name:  "valid HTTP URL",
			input: "http://example.com",
		},
		{
			name:  "URL with trailing slash",
			input: "https://example.com/path/",
		},
		{
			name:  "URL with only root slash",
			input: "https://example.com/",
		},
		{
			name:  "URL with port",
			input: "https://example.com:8443/oidc",
		},
		{
			name:  "URL with subdomain",
			input: "https://oidc-discovery.EXAMPLE.org",
		},
		{
			name:        "empty URL",
			input:       "",
			expectError: true,
			errorMsg:    "cannot be empty",
		},
		{
			name:        "invalid URL format",
			input:       "not-a-url",
			expectError: true,
			errorMsg:    "scheme is required",
		},
		{
			name:        "missing scheme",
			input:       "example.com",
			expectError: true,
			errorMsg:    "scheme is required",
		},
		{
			name:        "unsupported scheme",
			input:       "ftp://example.com",
			expectError: true,
			errorMsg:    "scheme must be http or https",
		},
		{
			name:        "URL with fragment",
			input:       "https://example.com#fragment",
			expectError: true,
			errorMsg:    "fragments are not allowed",
		},
		{
			name:        "URL with query parameters",
			input:       "https://example.com?param=value",
			expectError: true,
			errorMsg:    "query parameters are not allowed",
		},
		{
			name:        "malformed URL",
			input:       "https://[invalid-host",
			expectError: true,
			errorMsg:    "invalid URL format",
		},
		{
			name:        "scheme only",
			input:       "https://",
			expectError: true,
			errorMsg:    "host is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsValidURL(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNormalizeJWTIssuerURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{
			name:     "HTTPS URL with uppercase domain - should normalize",
			input:    "https://EXAMPLE.COM",
			expected: "https://example.com",
		},
		{
			name:     "HTTPS URL with mixed case domain - should normalize",
			input:    "https://Example.COM",
			expected: "https://example.com",
		},
		{
			name:     "HTTPS URL with path - domain normalized, path preserved",
			input:    "https://EXAMPLE.com/Path/To/Resource",
			expected: "https://example.com/Path/To/Resource",
		},
		{
			name:     "URL with trailing slash - should remove",
			input:    "https://example.com/path/",
			expected: "https://example.com/path",
		},
		{
			name:     "URL with only root slash - should remove",
			input:    "https://example.com/",
			expected: "https://example.com",
		},
		{
			name:     "URL with multiple trailing slashes - should remove all",
			input:    "https://example.com/path///",
			expected: "https://example.com/path",
		},
		{
			name:     "URL with port - should preserve",
			input:    "https://example.com:8443/oidc",
			expected: "https://example.com:8443/oidc",
		},
		{
			name:        "invalid URL",
			input:       "not-a-url",
			expectError: true,
			errorMsg:    "scheme is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeURL(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestStripProtocolFromJWTIssuer(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
		errorMsg    string
	}{
		{
			name:     "HTTPS URL with mixed case domain",
			input:    "https://EXAMPLE.com/Path",
			expected: "example.com/Path",
		},
		{
			name:     "HTTP URL with path",
			input:    "http://example.com/oidc/discovery",
			expected: "example.com/oidc/discovery",
		},
		{
			name:     "URL with port",
			input:    "https://example.com:8443",
			expected: "example.com:8443",
		},
		{
			name:     "URL with trailing slash",
			input:    "https://example.com/path/",
			expected: "example.com/path",
		},
		{
			name:     "URL with multiple trailing slashes",
			input:    "https://example.com/path////",
			expected: "example.com/path",
		},
		{
			name:     "simple domain",
			input:    "https://example.com",
			expected: "example.com",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:        "invalid URL",
			input:       "not-a-url",
			expectError: true,
			errorMsg:    "invalid issuer URL",
		},
		{
			name:        "URL with query parameters",
			input:       "https://example.com?param=value",
			expectError: true,
			errorMsg:    "invalid issuer URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := StripProtocolFromJWTIssuer(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
