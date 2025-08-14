package version

// These variables can be overridden at build time via -ldflags -X
// Defaults are sane fallbacks for local development.
var (
	// COMMIT and SHORTCOMMIT are injected from git via Makefile
	COMMIT      string
	SHORTCOMMIT string

	// Operator version (informational)
	OperatorVersion string = "0.2.0"

	// Per-component versions (used for app.kubernetes.io/version labels)
	SpiffeCsiVersion                  string = "0.2.7"
	SpireAgentVersion                 string = "1.12.4"
	SpireControllerManagerVersion     string = "0.6.2"
	SpireOIDCDiscoveryProviderVersion string = "1.12.4"
	SpireServerVersion                string = "1.12.4"
)
