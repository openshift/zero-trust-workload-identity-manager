package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// IsValidURL validates URL format.
func IsValidURL(issuerURL string) error {
	if issuerURL == "" {
		return fmt.Errorf("issuer URL cannot be empty")
	}

	u, err := url.Parse(issuerURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	return validateURLComponents(u)
}

// NormalizeURL normalizes JWT issuer URL for consistent comparison
func NormalizeURL(issuerURL string) (string, error) {
	if err := IsValidURL(issuerURL); err != nil {
		return "", err
	}

	u, err := url.Parse(issuerURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}
	return buildNormalizedURL(u), nil
}

// StripProtocolFromJWTIssuer removes protocol from validated JWT issuer URL
func StripProtocolFromJWTIssuer(issuerURL string) (string, error) {
	if issuerURL == "" {
		return "", nil
	}

	if err := IsValidURL(issuerURL); err != nil {
		return "", fmt.Errorf("invalid issuer URL: %w", err)
	}

	normalizedURL, err := NormalizeURL(issuerURL)
	if err != nil {
		return "", fmt.Errorf("invalid issuer URL: %w", err)
	}
	u, err := url.Parse(normalizedURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	return stripProtocol(u), nil
}

// validateURLComponents checks individual URL components
func validateURLComponents(u *url.URL) error {
	if u.Scheme == "" {
		return fmt.Errorf("scheme is required")
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return fmt.Errorf("scheme must be http or https, got: %s", u.Scheme)
	}

	if u.Host == "" {
		return fmt.Errorf("host is required")
	}

	if u.Fragment != "" {
		return fmt.Errorf("fragments are not allowed")
	}

	if u.RawQuery != "" {
		return fmt.Errorf("query parameters are not allowed")
	}

	return nil
}

// buildNormalizedURL creates a normalized URL string
func buildNormalizedURL(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Host)
	path := u.Path

	normalized := fmt.Sprintf("%s://%s%s", scheme, host, path)
	return removeTrailingSlash(normalized)
}

// stripProtocol removes scheme from URL, keeping host and path
func stripProtocol(u *url.URL) string {
	result := u.Host + u.Path
	return removeTrailingSlash(result)
}

// removeTrailingSlash removes trailing slash unless it's the root
func removeTrailingSlash(s string) string {
	if len(s) <= 1 {
		return s
	}
	trimmed := strings.TrimRight(s, "/")
	if trimmed == "" {
		return "/"
	}

	return trimmed
}
