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

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

const (
	TLSScannerPodName   = "tls-scanner"
	TLSScannerContainer = "scanner"

	tlsScannerCRBName = "ztwim-e2e-tls-scanner-cluster-admin"
	tlsScannerSAName  = "ztwim-e2e-tls-scanner"

	// TLSScannerReadyTimeout is how long to wait for the scanner pod to become Ready.
	TLSScannerReadyTimeout = 5 * time.Minute
	// TLSScannerRunTimeout is the maximum time to wait for the scan to complete.
	TLSScannerRunTimeout = 30 * time.Minute
)

// TLSScannerImage returns the tls-scanner container image from TLS_SCANNER_IMAGE.
// Returns "" when the env var is unset or empty — callers should Skip or Fail.
func TLSScannerImage() string {
	return strings.TrimSpace(os.Getenv("TLS_SCANNER_IMAGE"))
}

// ScanLimitIPs returns the SCAN_LIMIT_IPS value (0 means no limit / unset).
func ScanLimitIPs() int {
	v := strings.TrimSpace(os.Getenv("SCAN_LIMIT_IPS"))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// EnsureScannerRBAC creates a dedicated ServiceAccount and grants it cluster-admin
// so the tls-scanner pod can discover and probe endpoints. Idempotent.
// Registers DeferCleanup to delete the binding after the suite.
func EnsureScannerRBAC(ctx context.Context, clientset kubernetes.Interface) {
	By(fmt.Sprintf("Ensuring tls-scanner ServiceAccount %s/%s", OperatorNamespace, tlsScannerSAName))

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsScannerSAName,
			Namespace: OperatorNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "ztwim-e2e",
			},
		},
	}
	_, err := clientset.CoreV1().ServiceAccounts(OperatorNamespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to create ServiceAccount for tls-scanner")
	}

	By("Granting cluster-admin to tls-scanner ServiceAccount")

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: tlsScannerCRBName,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "ztwim-e2e",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      tlsScannerSAName,
				Namespace: OperatorNamespace,
			},
		},
	}

	existing, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, tlsScannerCRBName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = clientset.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to create ClusterRoleBinding for tls-scanner")
	} else {
		Expect(err).NotTo(HaveOccurred(), "failed to get ClusterRoleBinding for tls-scanner")
		existing.RoleRef = crb.RoleRef
		existing.Subjects = crb.Subjects
		existing.Labels = crb.Labels
		_, err = clientset.RbacV1().ClusterRoleBindings().Update(ctx, existing, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to update ClusterRoleBinding for tls-scanner")
	}

	DeferCleanup(func(cleanupCtx context.Context) {
		err := clientset.RbacV1().ClusterRoleBindings().Delete(cleanupCtx, tlsScannerCRBName, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return
		}
		Expect(err).NotTo(HaveOccurred(), "failed to delete ClusterRoleBinding %s", tlsScannerCRBName)
	})

	fmt.Fprintf(GinkgoWriter, "ClusterRoleBinding %s created/updated for SA %s\n", tlsScannerCRBName, tlsScannerSAName)
}

// RunTLSScanner creates an in-cluster tls-scanner pod, waits for the scan to
// complete, and returns nil on success or an error with the exit code and
// output tail on failure. The pod runs in pod-mode (no hostNetwork/hostPID)
// with restricted security context, matching the tls-scanner-run CI step.
func RunTLSScanner(ctx context.Context, clientset kubernetes.Interface, profileType string) error {
	image := TLSScannerImage()
	if image == "" {
		return fmt.Errorf("TLS_SCANNER_IMAGE is not set")
	}

	ns := OperatorNamespace
	By(fmt.Sprintf("Running tls-scanner for profile %q in %s", profileType, ns))

	// Delete any leftover scanner pod from a prior run.
	_ = clientset.CoreV1().Pods(ns).Delete(ctx, TLSScannerPodName, metav1.DeleteOptions{})
	Eventually(func() bool {
		_, err := clientset.CoreV1().Pods(ns).Get(ctx, TLSScannerPodName, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(ShortTimeout).WithPolling(ShortInterval).Should(BeTrue(),
		"previous tls-scanner pod should be deleted")

	// Build scanner arguments.
	// For Custom profiles, omit --tls-profile-type so the scanner auto-detects
	// the Custom TLS profile from the APIServer CR (extractAPIServerTLS handles
	// Custom correctly). The CLI flag only accepts Old, Intermediate, Modern.
	scanArgs := fmt.Sprintf("--all-pods --namespace-filter %s", ns)
	if profileType != APIServerTLSProfileCustom {
		scanArgs += fmt.Sprintf(" --tls-profile-type %s", profileType)
	}
	if limit := ScanLimitIPs(); limit > 0 {
		scanArgs += fmt.Sprintf(" --limit-ips %d", limit)
	}

	command := fmt.Sprintf(`mkdir -p /results
/usr/local/bin/tls-scanner -j 4 %s \
  --json-file /results/results.json \
  --csv-file /results/results.csv \
  --junit-file /results/junit_tls_scan.xml \
  --log-file /results/scan.log 2>&1 | tee /results/output.log
SCAN_EXIT_CODE=${PIPESTATUS[0]}
echo "Scan complete. Exit code: ${SCAN_EXIT_CODE}" | tee -a /results/output.log
touch /results/scan.done
sleep 120
exit ${SCAN_EXIT_CODE}`, scanArgs)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TLSScannerPodName,
			Namespace: ns,
			Labels: map[string]string{
				"app":                          "tls-scanner",
				"app.kubernetes.io/managed-by": "ztwim-e2e",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: tlsScannerSAName,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:    TLSScannerContainer,
				Image:   image,
				Command: []string{"/bin/bash", "-c", command},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsNonRoot:             ptr.To(true),
					RunAsUser:                ptr.To(int64(65532)),
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "results",
					MountPath: "/results",
				}},
			}},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   ptr.To(true),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Volumes: []corev1.Volume{{
				Name: "results",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
		},
	}

	_, err := clientset.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create tls-scanner pod: %w", err)
	}
	DeferCleanup(func(cleanupCtx context.Context) {
		_ = clientset.CoreV1().Pods(ns).Delete(cleanupCtx, TLSScannerPodName, metav1.DeleteOptions{})
	})

	// Wait for the pod to be Ready (image pull + container start).
	fmt.Fprintf(GinkgoWriter, "Waiting for tls-scanner pod to become Ready...\n")
	WaitForPodReady(ctx, clientset, TLSScannerPodName, ns, TLSScannerReadyTimeout)

	// Poll until scan.done marker or pod terminal phase.
	fmt.Fprintf(GinkgoWriter, "Waiting for tls-scanner scan to complete...\n")
	scanDone := false
	Eventually(func() bool {
		// Check scan.done marker first (pod is still alive for artifact collection).
		stdout, _, execErr := ExecInPod(ctx, ns, TLSScannerPodName, TLSScannerContainer,
			[]string{"test", "-f", "/results/scan.done"})
		_ = stdout
		if execErr == nil {
			scanDone = true
			return true
		}
		// Fallback: pod already exited.
		p, getErr := clientset.CoreV1().Pods(ns).Get(ctx, TLSScannerPodName, metav1.GetOptions{})
		if getErr != nil {
			return false
		}
		if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			fmt.Fprintf(GinkgoWriter, "tls-scanner pod phase=%s before scan.done marker\n", p.Status.Phase)
			return true
		}
		return false
	}).WithTimeout(TLSScannerRunTimeout).WithPolling(15*time.Second).Should(BeTrue(),
		"tls-scanner should complete within %v", TLSScannerRunTimeout)

	// Stream output to GinkgoWriter for visibility.
	if scanDone {
		output, _, _ := ExecInPod(ctx, ns, TLSScannerPodName, TLSScannerContainer,
			[]string{"cat", "/results/output.log"})
		if output != "" {
			fmt.Fprintf(GinkgoWriter, "--- tls-scanner output (profile=%s) ---\n%s\n--- end ---\n",
				profileType, output)
		}
	}

	// Determine exit code.
	p, err := clientset.CoreV1().Pods(ns).Get(ctx, TLSScannerPodName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get tls-scanner pod status: %w", err)
	}

	exitCode := int32(-1)
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Name == TLSScannerContainer && cs.State.Terminated != nil {
			exitCode = cs.State.Terminated.ExitCode
			break
		}
	}

	// Pod may still be in sleep 120 phase; wait briefly for termination if scan.done was found.
	if exitCode == -1 && scanDone {
		// Pod hasn't exited yet (still in sleep 120). Read exit code from output.log.
		output, _, _ := ExecInPod(ctx, ns, TLSScannerPodName, TLSScannerContainer,
			[]string{"sh", "-c", "grep 'Scan complete. Exit code:' /results/output.log | tail -1"})
		output = strings.TrimSpace(output)
		if strings.Contains(output, "Exit code: 0") {
			exitCode = 0
		} else if output != "" {
			// Parse exit code from "Scan complete. Exit code: N"
			parts := strings.Split(output, "Exit code: ")
			if len(parts) == 2 {
				if n, parseErr := strconv.Atoi(strings.TrimSpace(parts[1])); parseErr == nil {
					exitCode = int32(n)
				}
			}
		}
	}

	if exitCode != 0 {
		tail, _, _ := ExecInPod(ctx, ns, TLSScannerPodName, TLSScannerContainer,
			[]string{"sh", "-c", "tail -20 /results/scan.log 2>/dev/null || tail -20 /results/output.log"})
		return fmt.Errorf("tls-scanner exited with code %d for profile %s:\n%s",
			exitCode, profileType, strings.TrimSpace(tail))
	}

	fmt.Fprintf(GinkgoWriter, "tls-scanner passed for profile %s\n", profileType)
	return nil
}
