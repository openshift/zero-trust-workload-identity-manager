/*
Copyright 2026.

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

import (
	"context"

	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const testdataMySQLStack = "testdata/datastore-mysql/mysql.yaml"

// InstallMySQLDatastore deploys in-cluster MySQL for e2e.
func InstallMySQLDatastore(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, namespace string) {
	applyDatastoreTestdata(ctx, k8sClient, testdataMySQLStack, namespace)
	WaitForDeploymentAvailable(ctx, clientset, MySQLServiceName, namespace, DatastoreSetupTimeout)
}

// PatchSpireServerMySQLDatastore switches SpireServer to external MySQL.
func PatchSpireServerMySQLDatastore(ctx context.Context, k8sClient client.Client, namespace string) {
	conn := mysqlConnectionString(namespace)
	server := &operatorv1alpha1.SpireServer{}
	server.Name = OperandCRName
	err := UpdateCRWithRetry(ctx, k8sClient, server, func() {
		server.Spec.Datastore.DatabaseType = "mysql"
		server.Spec.Datastore.ConnectionString = conn
	})
	Expect(err).NotTo(HaveOccurred(), "patch SpireServer mysql datastore")
}

// CleanupMySQLDatastore removes test MySQL resources and restores sqlite.
func CleanupMySQLDatastore(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, namespace string) {
	RestoreSpireServerSQLiteDatastore(ctx, k8sClient)
	deleteDatastoreE2EResources(ctx, k8sClient, clientset, namespace, "mysql")
}
