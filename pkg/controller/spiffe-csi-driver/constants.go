package spiffe_csi_driver

import (
	corev1 "k8s.io/api/core/v1"
)

// DaemonSet constants
const (
	// DaemonSet metadata
	SpiffeCSIDaemonSetName      = "spire-spiffe-csi-driver"
	SpiffeCSIServiceAccountName = "spire-spiffe-csi-driver"

	// Update strategy
	SpiffeCSIMaxUnavailable int32 = 1
)

// Init container constants
const (
	// Init container configuration
	SpiffeCSIInitContainerName = "set-context"

	// SELinux context command
	SpiffeCSICommandChcon   = "chcon"
	SpiffeCSIArgRecursive   = "-Rvt"
	SpiffeCSIArgSELinuxType = "container_file_t"
	SpiffeCSIArgTargetDir   = "spire-agent-socket/"

	// Termination message
	SpiffeCSITerminationMessagePath         = "/dev/termination-log"
	SpiffeCSITerminationMessageReadFileType = corev1.TerminationMessageReadFile
)

// Main container constants
const (
	// Container names
	SpiffeCSIContainerNameDriver    = "spiffe-csi-driver"
	SpiffeCSIContainerNameRegistrar = "node-driver-registrar"

	// SPIFFE CSI Driver arguments
	SpiffeCSIArgWorkloadAPISocketDir  = "-workload-api-socket-dir"
	SpiffeCSIWorkloadAPISocketDirPath = "/spire-agent-socket"
	SpiffeCSIArgPluginName            = "-plugin-name"
	SpiffeCSIDefaultPluginName        = "csi.spiffe.io"
	SpiffeCSIArgCSISocketPath         = "-csi-socket-path"
	SpiffeCSISocketPath               = "/spiffe-csi/csi.sock"

	// Node Driver Registrar arguments
	SpiffeCSIArgCSIAddress              = "-csi-address"
	SpiffeCSICSIAddressPath             = "/spiffe-csi/csi.sock"
	SpiffeCSIArgKubeletRegistrationPath = "-kubelet-registration-path"
	// SpiffeCSIKubeletRegistrationPath uses the plugin name (SpiffeCSIDefaultPluginName = "csi.spiffe.io")
	// This path must match the plugin registration directory on the host and is derived from the plugin name
	SpiffeCSIKubeletRegistrationPath       = "/var/lib/kubelet/plugins/" + SpiffeCSIDefaultPluginName + "/csi.sock"
	SpiffeCSIArgHealthPort                 = "-health-port"
	SpiffeCSIHealthPort                    = "9809"
	SpiffeCSIHealthPortInt           int32 = 9809
)

// Environment variables
const (
	SpiffeCSIEnvMyNodeName = "MY_NODE_NAME"
	SpiffeCSIEnvFieldPath  = "spec.nodeName"
)

// Volume names
const (
	SpiffeCSIVolumeNameAgentSocketDir            = "spire-agent-socket-dir"
	SpiffeCSIVolumeNameCSISocketDir              = "spiffe-csi-socket-dir"
	SpiffeCSIVolumeNameMountpoint                = "mountpoint-dir"
	SpiffeCSIVolumeNameKubeletPluginRegistration = "kubelet-plugin-registration-dir"
)

// Mount paths
const (
	SpiffeCSIMountPathAgentSocket               = "/spire-agent-socket"
	SpiffeCSIMountPathCSISocket                 = "/spiffe-csi"
	SpiffeCSIMountPathKubeletPods               = "/var/lib/kubelet/pods"
	SpiffeCSIMountPathKubeletPluginRegistration = "/registration"
)

// Host paths
const (
	SpiffeCSIHostPathAgentSockets = "/run/spire/agent-sockets"
	// SpiffeCSIHostPathCSIPlugin uses the plugin name (SpiffeCSIDefaultPluginName = "csi.spiffe.io")
	SpiffeCSIHostPathCSIPlugin             = "/var/lib/kubelet/plugins/" + SpiffeCSIDefaultPluginName
	SpiffeCSIHostPathKubeletPods           = "/var/lib/kubelet/pods"
	SpiffeCSIHostPathPluginsRegistry       = "/var/lib/kubelet/plugins_registry"
	SpiffeCSIHostPathTypeDirectoryOrCreate = corev1.HostPathDirectoryOrCreate
	SpiffeCSIHostPathTypeDirectory         = corev1.HostPathDirectory
)

// Probe configuration
const (
	SpiffeCSIProbePathHealthz = "/healthz"
	SpiffeCSIPortNameHealthz  = "healthz"

	// Registrar probe timing
	SpiffeCSIRegistrarLivenessInitialDelay int32 = 5
	SpiffeCSIRegistrarLivenessTimeout      int32 = 5
)

// Security configuration
const (
	// Capabilities to drop
	SpiffeCSICapabilityDropAll = "all"
)
