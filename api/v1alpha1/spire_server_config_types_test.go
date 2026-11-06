package v1alpha1

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// Exercise the generated admission rules, so tests also detect stale CRD output.
// These are schema/CEL unit tests; API-server admission is a separate integration check.
func upstreamSchema(t *testing.T) apiextensionsv1.JSONSchemaProps {
	t.Helper()
	data, err := os.ReadFile("../../config/crd/bases/operator.openshift.io_spireservers.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatal(err)
	}
	return crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["upstreamAuthority"]
}

func evaluateRules(t *testing.T, schema apiextensionsv1.JSONSchemaProps, value any) bool {
	t.Helper()
	env, err := cel.NewEnv(cel.Variable("self", cel.DynType))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range schema.XValidations {
		ast, issues := env.Compile(rule.Rule)
		if issues.Err() != nil {
			t.Fatal(issues.Err())
		}
		program, err := env.Program(ast)
		if err != nil {
			t.Fatal(err)
		}
		result, _, err := program.Eval(map[string]any{"self": value})
		if err != nil {
			t.Fatal(err)
		}
		if result.Value() != true {
			return false
		}
	}
	return true
}

func TestUpstreamAuthorityUnion(t *testing.T) {
	schema := upstreamSchema(t)
	if len(schema.XValidations) == 0 {
		t.Fatal("missing authority union CEL validation")
	}
	for mask := 0; mask < 8; mask++ {
		value := map[string]any{}
		for i, name := range []string{"certManager", "vault", "spire"} {
			if mask&(1<<i) != 0 {
				value[name] = map[string]any{}
			}
		}
		if got, want := evaluateRules(t, schema, value), len(value) == 1; got != want {
			t.Errorf("authority set %v: valid=%v, want %v", value, got, want)
		}
	}
}

func TestSpireAuthoritySchema(t *testing.T) {
	schema := upstreamSchema(t).Properties["spire"]
	for _, field := range []string{"serverAddress", "trustBundleSecret"} {
		if !slices.Contains(schema.Required, field) {
			t.Errorf("%s must be required", field)
		}
	}
	for _, tc := range []struct {
		field string
		value string
		valid bool
	}{
		{"serverAddress", "upstream.example.org", true},
		{"serverAddress", "localhost", true},
		{"serverAddress", strings.Repeat("a", 63) + ".example", true},
		{"serverAddress", "", false},
		{"serverAddress", "https://example.org", false},
		{"serverAddress", "example.org:443", false},
		{"serverAddress", "example.org/path", false},
		{"serverAddress", "-example.org", false},
		{"serverAddress", "example..org", false},
		{"serverAddress", "UPSTREAM.example.org", false},
		{"serverAddress", strings.Repeat("a", 64) + ".example", false},
		{"serverAddress", strings.Repeat("a.", 127) + "a", false},
		{"trustBundleSecret", "upstream-bundle.v1", true},
		{"trustBundleSecret", "", false},
		{"trustBundleSecret", "Bad_Secret", false},
		{"trustBundleSecret", "bundle/secret", false},
		{"trustBundleSecret", strings.Repeat("a", 254), false},
	} {
		t.Run(tc.field+"/"+tc.value, func(t *testing.T) {
			field := schema.Properties[tc.field]
			if field.MinLength == nil || field.MaxLength == nil || field.Pattern == "" {
				t.Fatal("missing generated string constraints")
			}
			valid := int64(len(tc.value)) >= *field.MinLength && int64(len(tc.value)) <= *field.MaxLength &&
				regexp.MustCompile(field.Pattern).MatchString(tc.value) && evaluateRules(t, field, tc.value)
			if valid != tc.valid {
				t.Errorf("valid=%v, want %v", valid, tc.valid)
			}
		})
	}
	port := schema.Properties["serverPort"]
	if slices.Contains(schema.Required, "serverPort") || port.Default == nil || string(port.Default.Raw) != "443" {
		t.Fatal("optional serverPort must default to 443")
	}
	if port.Minimum == nil || port.Maximum == nil {
		t.Fatal("missing port bounds")
	}
	for _, value := range []int{0, 1, 443, 65535, 65536} {
		valid := float64(value) >= *port.Minimum && float64(value) <= *port.Maximum
		if valid != (value >= 1 && value <= 65535) {
			t.Errorf("unexpected validation for port %d", value)
		}
	}
}

func TestSpireAuthorityDeepCopy(t *testing.T) {
	original := &SpireServer{Spec: SpireServerSpec{UpstreamAuthority: &UpstreamAuthorityConfig{
		Spire: &UpstreamAuthoritySpire{ServerAddress: "upstream.example.org", TrustBundleSecret: "bundle"},
	}}}
	copy := original.DeepCopy()
	copy.Spec.UpstreamAuthority.Spire.ServerAddress = "changed.example.org"
	if original.Spec.UpstreamAuthority.Spire.ServerAddress != "upstream.example.org" {
		t.Fatal("deep copy aliases the original authority")
	}
}

func TestGeneratedSpireServerCELSyntax(t *testing.T) {
	data, err := os.ReadFile("../../config/crd/bases/operator.openshift.io_spireservers.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatal(err)
	}
	env, err := cel.NewEnv(cel.Variable("self", cel.DynType), cel.Variable("oldSelf", cel.DynType))
	if err != nil {
		t.Fatal(err)
	}
	var check func(apiextensionsv1.JSONSchemaProps)
	check = func(schema apiextensionsv1.JSONSchemaProps) {
		for _, rule := range schema.XValidations {
			if _, issues := env.Compile(rule.Rule); issues.Err() != nil {
				t.Errorf("invalid generated CEL %q: %v", rule.Rule, issues.Err())
			}
		}
		for _, property := range schema.Properties {
			check(property)
		}
		if schema.Items != nil && schema.Items.Schema != nil {
			check(*schema.Items.Schema)
		}
	}
	for _, version := range crd.Spec.Versions {
		check(*version.Schema.OpenAPIV3Schema)
	}
}
