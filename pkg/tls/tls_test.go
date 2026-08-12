/*
Copyright 2026 Red Hat, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tls

import (
	"context"
	"crypto/tls"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	utiltls "github.com/openshift/controller-runtime-common/pkg/tls"
	libgocrypto "github.com/openshift/library-go/pkg/crypto"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

func applyTLSConfig(t *testing.T, tlsConfig func(*tls.Config)) *tls.Config {
	t.Helper()

	if tlsConfig == nil {
		t.Fatal("expected TLSConfig function to be set")
	}

	cfg := &tls.Config{}
	tlsConfig(cfg)
	return cfg
}

func TestGetOperatorAndOperandTLSConfig(t *testing.T) {
	oldProfile := configv1.TLSProfiles[configv1.TLSProfileOldType]
	modernProfile := configv1.TLSProfiles[configv1.TLSProfileModernType]
	setupLog := logr.Discard()

	tests := []struct {
		name               string
		adherence          configv1.TLSAdherencePolicy
		profileSpec        configv1.TLSProfileSpec
		wantNilOperator    bool
		wantNilOperand     bool
		wantMinTLSVersion  uint16
		wantCipherSuites   bool
		wantOperandMinTLS  configv1.TLSProtocolVersion
		wantOperandCiphers bool
	}{
		{
			name:            "NoOpinion skips operator and operand TLS profile",
			adherence:       configv1.TLSAdherencePolicyNoOpinion,
			profileSpec:     *oldProfile,
			wantNilOperator: true,
			wantNilOperand:  true,
		},
		{
			name:              "StrictAllComponents honors Old profile on operator only",
			adherence:         configv1.TLSAdherencePolicyStrictAllComponents,
			profileSpec:       *oldProfile,
			wantMinTLSVersion: libgocrypto.TLSVersionOrDie(string(oldProfile.MinTLSVersion)),
			wantCipherSuites:  true,
			wantNilOperand:    true,
		},
		{
			name:              "StrictAllComponents honors Modern profile without cipher suites",
			adherence:         configv1.TLSAdherencePolicyStrictAllComponents,
			profileSpec:       *modernProfile,
			wantMinTLSVersion: libgocrypto.TLSVersionOrDie(string(modernProfile.MinTLSVersion)),
			wantCipherSuites:  false,
			wantOperandMinTLS: modernProfile.MinTLSVersion,
		},
		{
			name:      "StrictAllComponents honors custom profile",
			adherence: configv1.TLSAdherencePolicyStrictAllComponents,
			profileSpec: configv1.TLSProfileSpec{
				Ciphers: []string{
					"ECDHE-RSA-AES128-GCM-SHA256",
					"ECDHE-RSA-AES256-GCM-SHA384",
				},
				MinTLSVersion: configv1.VersionTLS12,
			},
			wantMinTLSVersion:  tls.VersionTLS12,
			wantCipherSuites:   true,
			wantOperandMinTLS:  configv1.VersionTLS12,
			wantOperandCiphers: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operatorTLSConfig, operandTLSProfile := getOperatorAndOperandTLSConfig(tt.adherence, tt.profileSpec, setupLog)

			if tt.wantNilOperator {
				if operatorTLSConfig != nil {
					t.Fatal("expected nil operator TLS config")
				}
			} else {
				tlsCfg := applyTLSConfig(t, operatorTLSConfig)
				if tlsCfg.MinVersion != tt.wantMinTLSVersion {
					t.Fatalf("MinVersion = %d, want %d", tlsCfg.MinVersion, tt.wantMinTLSVersion)
				}
				if tt.wantCipherSuites {
					if len(tlsCfg.CipherSuites) == 0 {
						t.Fatal("expected cipher suites to be configured")
					}
				} else if len(tlsCfg.CipherSuites) != 0 {
					t.Fatalf("expected no cipher suites, got %v", tlsCfg.CipherSuites)
				}
			}

			if tt.wantNilOperand {
				if operandTLSProfile != nil {
					t.Fatalf("expected nil operand profile, got %#v", operandTLSProfile)
				}
				return
			}

			if operandTLSProfile == nil {
				t.Fatal("expected non-nil operand profile")
			}
			if operandTLSProfile.MinTLSVersion != tt.wantOperandMinTLS {
				t.Fatalf("MinTLSVersion = %q, want %q", operandTLSProfile.MinTLSVersion, tt.wantOperandMinTLS)
			}
			if tt.wantOperandCiphers && len(operandTLSProfile.CipherSuites) == 0 {
				t.Fatal("expected cipher suites to be populated")
			}
		})
	}
}

func TestGetOperandTLSConfig(t *testing.T) {
	setupLog := logr.Discard()
	oldProfile := configv1.TLSProfiles[configv1.TLSProfileOldType]

	if config := getOperandTLSConfig(*oldProfile, setupLog); config != nil {
		t.Fatalf("expected nil operand config for Old profile, got %#v", config)
	}

	customConfig := getOperandTLSConfig(configv1.TLSProfileSpec{
		Ciphers: []string{
			"ECDHE-RSA-AES128-GCM-SHA256",
			"ECDHE-RSA-AES256-GCM-SHA384",
		},
		MinTLSVersion: configv1.VersionTLS12,
	}, setupLog)
	if customConfig == nil {
		t.Fatal("expected non-nil operand config")
	}
	if customConfig.MinTLSVersion != configv1.VersionTLS12 {
		t.Fatalf("MinTLSVersion = %q, want %q", customConfig.MinTLSVersion, configv1.VersionTLS12)
	}
	if !reflect.DeepEqual(customConfig.CipherSuites, []string{
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	}) {
		t.Fatalf("CipherSuites = %q, want IANA cipher names", customConfig.CipherSuites)
	}
}

func TestDefaultClusterTLSProfileSpec(t *testing.T) {
	got, _ := utiltls.GetTLSProfileSpec(nil)
	want, err := utiltls.GetTLSProfileSpec(nil)
	if err != nil {
		t.Fatalf("GetTLSProfileSpec(nil) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaultClusterTLSProfileSpec() = %#v, want %#v", got, want)
	}
}

func TestFetchAPIServerTLSConfig_clientCreationError(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := configv1.Install(scheme); err != nil {
		t.Fatalf("failed to install configv1 scheme: %v", err)
	}

	_, err := FetchAPIServerTLSConfig(context.Background(), nil, scheme, logr.Discard())
	if err == nil {
		t.Fatal("expected error when rest config is nil, got nil")
	}
}

func TestFetchAPIServerTLSConfig_nonStrictProfileFetchSeedsDefaultBaseline(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := configv1.Install(scheme); err != nil {
		t.Fatalf("failed to install configv1 scheme: %v", err)
	}

	defaultProfile, _ := utiltls.GetTLSProfileSpec(nil)
	result, err := FetchAPIServerTLSConfig(context.Background(), &rest.Config{}, scheme, logr.Discard())
	if err != nil {
		t.Fatalf("FetchAPIServerTLSConfig() error = %v", err)
	}

	if result.TLSAdherencePolicy != configv1.TLSAdherencePolicyNoOpinion {
		t.Fatalf("TLSAdherencePolicy = %q, want %q", result.TLSAdherencePolicy, configv1.TLSAdherencePolicyNoOpinion)
	}
	if result.OperatorTLSConfig != nil {
		t.Fatal("expected nil operator TLS config under non-strict adherence")
	}
	if result.OperandTLSConfig != nil {
		t.Fatal("expected nil operand TLS config under non-strict adherence")
	}
	if !reflect.DeepEqual(result.TLSProfileSpec, defaultProfile) {
		t.Fatalf("TLSProfileSpec = %#v, want default Intermediate baseline %#v", result.TLSProfileSpec, defaultProfile)
	}
}

func TestGetTLSProfileSpecIntegration(t *testing.T) {
	defaultProfile := configv1.TLSProfiles[libgocrypto.DefaultTLSProfileType]

	profile, err := utiltls.GetTLSProfileSpec(nil)
	if err != nil {
		t.Fatalf("GetTLSProfileSpec(nil) error = %v", err)
	}
	if !reflect.DeepEqual(profile, *defaultProfile) {
		t.Fatalf("GetTLSProfileSpec(nil) = %#v, want default Intermediate profile", profile)
	}

	_, err = utiltls.GetTLSProfileSpec(&configv1.TLSSecurityProfile{Type: configv1.TLSProfileCustomType})
	if err != utiltls.ErrCustomProfileNil {
		t.Fatalf("GetTLSProfileSpec(invalid custom) error = %v, want %v", err, utiltls.ErrCustomProfileNil)
	}
}
