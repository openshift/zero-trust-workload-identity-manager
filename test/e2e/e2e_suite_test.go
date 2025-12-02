/*
Copyright 2025.

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

package e2e

import (
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"
	operatorv1 "github.com/operator-framework/api/pkg/operators/v1"

	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cfg          *rest.Config
	k8sClient    client.Client
	clientset    kubernetes.Interface
	apiextClient apiextclient.Interface
	configClient configv1.ConfigV1Interface
)

var _ = BeforeSuite(func() {
	var err error

	// Get Kubernetes configuration
	cfg, err = utils.GetKubeConfig()
	Expect(err).NotTo(HaveOccurred(), "failed to get Kubernetes config")

	// Create runtime scheme with necessary types
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorv1.AddToScheme(scheme))

	// Create controller-runtime client
	k8sClient, err = client.New(cfg, client.Options{
		Scheme: scheme,
	})
	Expect(err).NotTo(HaveOccurred(), "failed to create controller-runtime client")

	// Create Kubernetes clientset
	clientset, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred(), "failed to create Kubernetes clientset")

	// Create Kubernetes API extensions clientset
	apiextClient, err = apiextclient.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred(), "failed to create apiextensions clientset")

	// Create OpenShift config client
	configClient, err = configv1.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred(), "failed to create OpenShift config client")
})

// TestE2E runs the e2e test suite
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	// Configure Ginkgo for e2e tests
	suiteConfig, reporterConfig := GinkgoConfiguration()

	// Suite-level configuration
	suiteConfig.Timeout = 20 * time.Minute // Global timeout per individual test
	suiteConfig.FailFast = false           // Continue after first failure to see all issues
	suiteConfig.FlakeAttempts = 0          // Retry on flaky tests (helpful when deflaking tests)
	suiteConfig.MustPassRepeatedly = 1     // Must pass repeatedly times (helpful when deflaking tests)

	// Reporter configuration
	reporterConfig.Verbose = true                                               // Show verbose outputs
	reporterConfig.ShowNodeEvents = true                                        // Show node events
	reporterConfig.FullTrace = true                                             // Show full stack traces
	reporterConfig.SilenceSkips = true                                          // Silence skipped tests
	reporterConfig.NoColor = true                                               // No color to avoid rendering issues in CI
	reporterConfig.JUnitReport = filepath.Join(utils.GetTestDir(), "junit.xml") // Write JUnit report to test directory

	RunSpecs(t, "Zero Trust Workload Identity Manager E2E test suite", suiteConfig, reporterConfig)
}
