package utils

import "os"

func GetSpireServerImage() string {
	spireServerImage := os.Getenv(SpireServerImageEnv)
	if spireServerImage == "" {
		return ""
	}
	return spireServerImage
}

func GetSpireAgentImage() string {
	spireAgentImage := os.Getenv(SpireAgentImageEnv)
	if spireAgentImage == "" {
		return ""
	}
	return spireAgentImage
}

func GetSpiffeCSIDriverImage() string {
	spiffeCSIDriverImage := os.Getenv(SpiffeCSIDriverImageEnv)
	if spiffeCSIDriverImage == "" {
		return ""
	}
	return spiffeCSIDriverImage
}

func GetSpireControllerManagerImage() string {
	spireControllerManagerImage := os.Getenv(SpireControllerManagerImageEnv)
	if spireControllerManagerImage == "" {
		return ""
	}
	return spireControllerManagerImage
}

func GetSpireOIDCDiscoveryProviderImage() string {
	spireOIDCDiscoveryProviderImage := os.Getenv(SpireOIDCDiscoveryProviderImageEnv)
	if spireOIDCDiscoveryProviderImage == "" {
		return ""
	}
	return spireOIDCDiscoveryProviderImage
}

func GetNodeDriverRegistrarImage() string {
	nodeDriverRegistrarImage := os.Getenv(NodeDriverRegistrarImageEnv)
	if nodeDriverRegistrarImage == "" {
		return ""
	}
	return nodeDriverRegistrarImage
}

func GetSpiffeCsiInitContainerImage() string {
	containerImage := os.Getenv(SpiffeCSIInitContainerImageEnv)
	if containerImage == "" {
		return "registry.access.redhat.com/ubi9:latest"
	}
	return containerImage
}
