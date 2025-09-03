package utils

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CreateOnlyAnnotation     = "ztwim.openshift.io/create-only"
	CreateOnlyModeStatusType = "CreateOnlyMode"
	CreateOnlyModeEnabled    = "CreateOnlyModeEnabled"
	CreateOnlyModeDisabled   = "CreateOnlyModeDisabled"
)

func IsCreateOnlyAnnotationEnabled(obj client.Object) bool {
	if obj == nil {
		return false
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	return annotations[CreateOnlyAnnotation] == "true"
}

func IsInCreateOnlyMode(obj client.Object, createOnlyFlag *bool) bool {
	currentCreateOnly := IsCreateOnlyAnnotationEnabled(obj)
	if currentCreateOnly {
		*createOnlyFlag = true
		return true
	}
	if *createOnlyFlag {
		return true
	}
	return false
}
