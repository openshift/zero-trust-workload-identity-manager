package static_resource_controller

import (
	"context"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

func (r *StaticResourceReconciler) CreateSpiffeCsiDriver(ctx context.Context) error {
	err := r.ctrlClient.Create(ctx, r.getSpiffeCsiObject())
	if err != nil && errors.IsAlreadyExists(err) {
		return nil
	}
	if err != nil {
		r.log.Error(err, "failed to create or apply spiffe csi driver resources")
		return err
	}
	return nil
}

func (r *StaticResourceReconciler) getSpiffeCsiObject() *storagev1.CSIDriver {
	csiDriver := utils.DecodeCsiDriverObjBytes(assets.MustAsset(utils.SpiffeCsiDriverAssetName))
	return csiDriver
}
