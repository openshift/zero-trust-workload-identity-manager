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

package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"os"
	"path/filepath"

	routev1 "github.com/openshift/api/route/v1"
	operatorv1 "github.com/operator-framework/api/pkg/operators/v1"

	"k8s.io/klog/v2/textlogger"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	operatoropenshiftiov1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	spiffeCsiDriverController "github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/spiffe-csi-driver"
	spireAgentController "github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/spire-agent"
	spireOIDCDiscoveryProviderController "github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/spire-oidc-discovery-provider"
	spireServerController "github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/spire-server"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	ztwimController "github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/zero-trust-workload-identity-manager"

	securityv1 "github.com/openshift/api/security/v1"

	ctrlmgr "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

const (
	// metricsCertFileName is the certificate filename, which should be present
	// at the passed `metrics-cert-dir` path.
	metricsCertFileName = "tls.crt"

	// metricsKeyFileName is the private key filename, which should be present
	// at the passed `metrics-cert-dir` path.
	metricsKeyFileName = "tls.key"

	openshiftCACertificateFile = "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(operatoropenshiftiov1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		secureMetrics        bool
		enableHTTP2          bool
		logLevel             int
		metricsCerts         string
		metricsTLSOpts       []func(*tls.Config)
		webhookTLSOpts       []func(*tls.Config)
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8443", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP. Set to 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.IntVar(&logLevel, "v", 2, "operator log verbosity")
	flag.StringVar(&metricsCerts, "metrics-cert-dir", "",
		"Secret name containing the certificates for the metrics server which should be present in operator namespace. "+
			"If not provided self-signed certificates will be used")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logConfig := textlogger.NewConfig(textlogger.Verbosity(logLevel))
	ctrl.SetLogger(textlogger.NewLogger(logConfig))

	// Validate that OPERATOR_NAMESPACE is set
	operatorNamespace := utils.GetOperatorNamespace()
	if operatorNamespace == "" {
		setupLog.Error(nil, "failed to start the operator, operator namespace is empty")
		os.Exit(1)
	}
	setupLog.Info("Operator namespace configured", "namespace", operatorNamespace)

	if !enableHTTP2 {
		// if the enable-http2 flag is false (the default), http/2 should be disabled
		// due to its vulnerabilities.
		disableHTTP2 := func(c *tls.Config) {
			setupLog.Info("disabling http/2 for both metrics and webhook servers")
			c.NextProtos = []string{"http/1.1"}
		}
		metricsTLSOpts = append(metricsTLSOpts, disableHTTP2)
		webhookTLSOpts = append(webhookTLSOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress: metricsAddr,

		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		FilterProvider: filters.WithAuthenticationAndAuthorization,
	}

	if secureMetrics {
		setupLog.Info("setting up secure metrics server")
		metricsServerOptions.SecureServing = secureMetrics
		if metricsCerts != "" {
			if _, err := os.Stat(filepath.Join(metricsCerts, metricsCertFileName)); err != nil {
				setupLog.Error(err, "metrics certificate file not found at configured path")
				os.Exit(1)
			}
			if _, err := os.Stat(filepath.Join(metricsCerts, metricsKeyFileName)); err != nil {
				setupLog.Error(err, "metrics private key file not found at configured path")
				os.Exit(1)
			}
			setupLog.Info("using certificate key pair found in the configured dir for metrics server")
			metricsServerOptions.CertDir = metricsCerts
			metricsServerOptions.CertName = metricsCertFileName
			metricsServerOptions.KeyName = metricsKeyFileName
		}
		metricsTLSOpts = append(metricsTLSOpts, func(c *tls.Config) {
			certPool, err := x509.SystemCertPool()
			if err != nil {
				setupLog.Info("unable to load system certificate pool", "error", err)
				certPool = x509.NewCertPool()
			}
			openshiftCACert, err := os.ReadFile(openshiftCACertificateFile)
			if err != nil {
				setupLog.Error(err, "failed to read OpenShift CA certificate")
				os.Exit(1)
			}
			setupLog.Info("using openshift service CA for metrics client verification")
			certPool.AppendCertsFromPEM(openshiftCACert)
			c.ClientCAs = certPool
		})
		metricsServerOptions.TLSOpts = metricsTLSOpts
	}
	config := ctrl.GetConfigOrDie()

	// Increase QPS and Burst to allow more concurrent API calls
	config.QPS = 50    // Default is usually 5, increase as needed
	config.Burst = 100 // Default is usually 10, increase as needed

	// Add OpenShift SCC scheme
	if err := securityv1.AddToScheme(scheme); err != nil {
		exitOnError(err, "unable to add securityv1 scheme")
	}
	if err := ctrlmgr.AddToScheme(scheme); err != nil {
		exitOnError(err, "unable to add spiffev1alpha1 scheme")
	}

	if err := routev1.AddToScheme(scheme); err != nil {
		exitOnError(err, "unable to add routev1 scheme")
	}

	// Add OperatorCondition scheme for OLM integration
	if err := operatorv1.AddToScheme(scheme); err != nil {
		exitOnError(err, "unable to add operatorv1 scheme")
	}

	// Create unified cache builder to prevent race conditions between manager and reconciler caches
	cacheBuilder, err := customClient.NewCacheBuilder()
	exitOnError(err, "unable to create cache builder")

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "24a59323.operator.openshift.io",
		NewCache:               cacheBuilder,
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	exitOnError(err, "unable to start manager")

	ztwimControllerManager, err := ztwimController.New(mgr)
	exitOnError(err, "unable to set up ztwim controller manager")
	if err = ztwimControllerManager.SetupWithManager(mgr); err != nil {
		exitOnError(err, "unable to setup ztwim controller manager")
	}

	spireServerControllerManager, err := spireServerController.New(mgr)
	exitOnError(err, "unable to set up spire server controller manager")
	if err = spireServerControllerManager.SetupWithManager(mgr); err != nil {
		exitOnError(err, "unable to setup spire server controller manager")
	}

	spireAgentControllerManager, err := spireAgentController.New(mgr)
	if err != nil {
		exitOnError(err, "unable to set up spire agent controller manager")
	}
	if err = spireAgentControllerManager.SetupWithManager(mgr); err != nil {
		exitOnError(err, "unable to setup spire agent controller manager")
	}

	spiffeCsiDriverControllerManager, err := spiffeCsiDriverController.New(mgr)
	if err != nil {
		exitOnError(err, "unable to set up spiffe csi driver controller manager")
	}
	if err = spiffeCsiDriverControllerManager.SetupWithManager(mgr); err != nil {
		exitOnError(err, "unable to setup spiffe csi driver controller manager")
	}

	spireOIDCDiscoveryProviderControllerManager, err := spireOIDCDiscoveryProviderController.New(mgr)
	if err != nil {
		exitOnError(err, "unable to set up spire OIDC discovery provider controller manager")
	}
	if err = spireOIDCDiscoveryProviderControllerManager.SetupWithManager(mgr); err != nil {
		exitOnError(err, "unable to setup spire OIDC discovery provider controller manager")
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		exitOnError(err, "unable to set up health check")
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		exitOnError(err, "unable to set up ready check")
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	err = mgr.Start(ctrl.SetupSignalHandler())
	exitOnError(err, "problem running manager")
}

func exitOnError(err error, logMessage string) {
	if err != nil {
		setupLog.Error(err, logMessage)
		os.Exit(1)
	}
}
