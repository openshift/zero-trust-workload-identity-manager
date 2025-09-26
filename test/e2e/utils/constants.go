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

package utils

import "time"

const (
	OperatorNamespace      = "zero-trust-workload-identity-manager"
	OperatorDeploymentName = "zero-trust-workload-identity-manager-controller-manager"
	OperatorLabelSelector  = "name=zero-trust-workload-identity-manager"

	SpireServerStatefulSetName               = "spire-server"
	SpireServerPodLabel                      = "app.kubernetes.io/name=spire-server"
	SpireAgentDaemonSetName                  = "spire-agent"
	SpireAgentPodLabel                       = "app.kubernetes.io/name=spire-agent"
	SpiffeCSIDriverDaemonSetName             = "spire-spiffe-csi-driver"
	SpiffeCSIDriverPodLabel                  = "app.kubernetes.io/name=spiffe-csi-driver"
	SpireOIDCDiscoveryProviderDeploymentName = "spire-spiffe-oidc-discovery-provider"
	SpireOIDCDiscoveryProviderPodLabel       = "app.kubernetes.io/name=spiffe-oidc-discovery-provider"

	DefaultInterval = 10 * time.Second
	ShortInterval   = 5 * time.Second
	DefaultTimeout  = 5 * time.Minute
	ShortTimeout    = 2 * time.Minute
)
