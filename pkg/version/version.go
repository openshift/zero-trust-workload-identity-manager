package version

// These variables can be overridden at build time via -ldflags -X
// Defaults are sane fallbacks for local development.
var (
	// COMMIT and SHORTCOMMIT are injected from git via Makefile
	COMMIT      string
	SHORTCOMMIT string

	// Operator version (informational)
	OperatorVersion string = "1.1.3"

	// Per-component versions (used for app.kubernetes.io/version labels)
	SpiffeCsiVersion                  string = "0.2.13"
	SpireAgentVersion                 string = "1.14.9"
	SpireControllerManagerVersion     string = "0.6.6"
	SpireOIDCDiscoveryProviderVersion string = "1.14.9"
	SpireServerVersion                string = "1.14.9"
)
