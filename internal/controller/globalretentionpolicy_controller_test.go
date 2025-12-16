/*
Copyright 2024 madic-creates.

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

package controller

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	backupv1alpha1 "github.com/madic-creates/restic-backup-operator/api/v1alpha1"
)

var _ = Describe("GlobalRetentionPolicy Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a GlobalRetentionPolicy", func() {
		var (
			testNamespace string
			policyKey     types.NamespacedName
			repositoryKey types.NamespacedName
			secretKey     types.NamespacedName
		)

		BeforeEach(func() {
			testNamespace = "test-grp-" + randString(5)
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: testNamespace},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			policyKey = types.NamespacedName{
				Name:      "test-retention-policy",
				Namespace: testNamespace,
			}
			repositoryKey = types.NamespacedName{
				Name:      "test-repository",
				Namespace: testNamespace,
			}
			secretKey = types.NamespacedName{
				Name:      "test-credentials",
				Namespace: testNamespace,
			}
		})

		AfterEach(func() {
			// Clean up resources in order
			policy := &backupv1alpha1.GlobalRetentionPolicy{}
			if err := k8sClient.Get(ctx, policyKey, policy); err == nil {
				controllerutil.RemoveFinalizer(policy, globalRetentionPolicyFinalizer)
				_ = k8sClient.Update(ctx, policy)
				_ = k8sClient.Delete(ctx, policy)
			}

			// Delete CronJob if exists
			cronJob := &batchv1.CronJob{}
			cronJobKey := types.NamespacedName{
				Name:      "globalretention-" + policyKey.Name,
				Namespace: testNamespace,
			}
			if err := k8sClient.Get(ctx, cronJobKey, cronJob); err == nil {
				_ = k8sClient.Delete(ctx, cronJob)
			}

			repository := &backupv1alpha1.ResticRepository{}
			if err := k8sClient.Get(ctx, repositoryKey, repository); err == nil {
				controllerutil.RemoveFinalizer(repository, resticRepositoryFinalizer)
				_ = k8sClient.Update(ctx, repository)
				_ = k8sClient.Delete(ctx, repository)
			}

			secret := &corev1.Secret{}
			if err := k8sClient.Get(ctx, secretKey, secret); err == nil {
				_ = k8sClient.Delete(ctx, secret)
			}

			ns := &corev1.Namespace{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace}, ns); err == nil {
				_ = k8sClient.Delete(ctx, ns)
			}
		})

		It("should add finalizer when created", func() {
			keepLast := int32(10)
			// Create the GlobalRetentionPolicy
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyKey.Name,
					Namespace: policyKey.Namespace,
				},
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 3 * * *",
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"daily"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			// Verify finalizer is added
			Eventually(func() bool {
				p := &backupv1alpha1.GlobalRetentionPolicy{}
				if err := k8sClient.Get(ctx, policyKey, p); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(p, globalRetentionPolicyFinalizer)
			}, timeout, interval).Should(BeTrue())
		})

		It("should set NotReady condition when repository does not exist", func() {
			keepLast := int32(10)
			// Create the GlobalRetentionPolicy without the repository
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyKey.Name,
					Namespace: policyKey.Namespace,
				},
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: "nonexistent-repository",
					},
					Schedule: "0 3 * * *",
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"daily"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			// Verify NotReady condition is set
			Eventually(func() bool {
				p := &backupv1alpha1.GlobalRetentionPolicy{}
				if err := k8sClient.Get(ctx, policyKey, p); err != nil {
					return false
				}
				for _, cond := range p.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == metav1.ConditionFalse {
						return cond.Reason == "RepositoryNotFound"
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("should set NotReady condition when repository is not ready", func() {
			keepLast := int32(10)
			// Create the credentials secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretKey.Name,
					Namespace: secretKey.Namespace,
				},
				Data: map[string][]byte{
					"RESTIC_PASSWORD": []byte("test-password"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			// Create the ResticRepository
			repository := &backupv1alpha1.ResticRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repositoryKey.Name,
					Namespace: repositoryKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRepositorySpec{
					RepositoryURL: "local:/tmp/test-repo",
					CredentialsSecretRef: backupv1alpha1.SecretKeySelector{
						Name: secretKey.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, repository)).To(Succeed())

			// Wait for the repository to get a condition (but not Ready=True)
			Eventually(func() bool {
				repo := &backupv1alpha1.ResticRepository{}
				if err := k8sClient.Get(ctx, repositoryKey, repo); err != nil {
					return false
				}
				return len(repo.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Create the GlobalRetentionPolicy
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyKey.Name,
					Namespace: policyKey.Namespace,
				},
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 3 * * *",
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"daily"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			// The policy should have some condition set
			Eventually(func() bool {
				p := &backupv1alpha1.GlobalRetentionPolicy{}
				if err := k8sClient.Get(ctx, policyKey, p); err != nil {
					return false
				}
				return len(p.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())
		})

		It("should create CronJob when repository is ready", func() {
			keepLast := int32(10)
			// Create the credentials secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretKey.Name,
					Namespace: secretKey.Namespace,
				},
				Data: map[string][]byte{
					"RESTIC_PASSWORD": []byte("test-password"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			// Create the ResticRepository
			repository := &backupv1alpha1.ResticRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repositoryKey.Name,
					Namespace: repositoryKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRepositorySpec{
					RepositoryURL: "local:/tmp/test-repo",
					CredentialsSecretRef: backupv1alpha1.SecretKeySelector{
						Name: secretKey.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, repository)).To(Succeed())

			// Manually set the repository to Ready status
			Eventually(func() error {
				repo := &backupv1alpha1.ResticRepository{}
				if err := k8sClient.Get(ctx, repositoryKey, repo); err != nil {
					return err
				}
				repo.Status.Conditions = []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionTrue,
						Reason:             "RepositoryAccessible",
						Message:            "Repository is ready",
						LastTransitionTime: metav1.Now(),
					},
				}
				return k8sClient.Status().Update(ctx, repo)
			}, timeout, interval).Should(Succeed())

			// Create the GlobalRetentionPolicy
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyKey.Name,
					Namespace: policyKey.Namespace,
				},
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 3 * * *",
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"daily"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			// Verify CronJob is created
			cronJobKey := types.NamespacedName{
				Name:      "globalretention-" + policyKey.Name,
				Namespace: testNamespace,
			}
			Eventually(func() bool {
				cronJob := &batchv1.CronJob{}
				return k8sClient.Get(ctx, cronJobKey, cronJob) == nil
			}, timeout, interval).Should(BeTrue())

			// Verify CronJob has correct schedule
			cronJob := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, cronJobKey, cronJob)).To(Succeed())
			Expect(cronJob.Spec.Schedule).To(Equal("0 3 * * *"))

			// Verify CronJob has correct labels
			Expect(cronJob.Labels["app.kubernetes.io/name"]).To(Equal("restic-backup-operator"))
			Expect(cronJob.Labels["backup.resticbackup.io/retentionpolicy"]).To(Equal(policyKey.Name))
		})

		It("should calculate next run time", func() {
			keepLast := int32(10)
			// Create the credentials secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretKey.Name,
					Namespace: secretKey.Namespace,
				},
				Data: map[string][]byte{
					"RESTIC_PASSWORD": []byte("test-password"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			// Create the ResticRepository
			repository := &backupv1alpha1.ResticRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repositoryKey.Name,
					Namespace: repositoryKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRepositorySpec{
					RepositoryURL: "local:/tmp/test-repo",
					CredentialsSecretRef: backupv1alpha1.SecretKeySelector{
						Name: secretKey.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, repository)).To(Succeed())

			// Manually set the repository to Ready status
			Eventually(func() error {
				repo := &backupv1alpha1.ResticRepository{}
				if err := k8sClient.Get(ctx, repositoryKey, repo); err != nil {
					return err
				}
				repo.Status.Conditions = []metav1.Condition{
					{
						Type:               "Ready",
						Status:             metav1.ConditionTrue,
						Reason:             "RepositoryAccessible",
						Message:            "Repository is ready",
						LastTransitionTime: metav1.Now(),
					},
				}
				return k8sClient.Status().Update(ctx, repo)
			}, timeout, interval).Should(Succeed())

			// Create the GlobalRetentionPolicy
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyKey.Name,
					Namespace: policyKey.Namespace,
				},
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 3 * * *",
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"daily"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			// Verify NextRun is set
			Eventually(func() bool {
				p := &backupv1alpha1.GlobalRetentionPolicy{}
				if err := k8sClient.Get(ctx, policyKey, p); err != nil {
					return false
				}
				return p.Status.NextRun != nil && p.Status.NextRun.Time.After(time.Now())
			}, timeout, interval).Should(BeTrue())
		})

		It("should remove finalizer on deletion", func() {
			keepLast := int32(10)
			// Create the GlobalRetentionPolicy
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyKey.Name,
					Namespace: policyKey.Namespace,
				},
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 3 * * *",
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"daily"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			// Wait for finalizer to be added
			Eventually(func() bool {
				p := &backupv1alpha1.GlobalRetentionPolicy{}
				if err := k8sClient.Get(ctx, policyKey, p); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(p, globalRetentionPolicyFinalizer)
			}, timeout, interval).Should(BeTrue())

			// Delete the policy
			Expect(k8sClient.Delete(ctx, policy)).To(Succeed())

			// Verify policy is eventually deleted
			Eventually(func() bool {
				p := &backupv1alpha1.GlobalRetentionPolicy{}
				err := k8sClient.Get(ctx, policyKey, p)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("buildRetentionScript helper function", func() {
		var reconciler *GlobalRetentionPolicyReconciler

		BeforeEach(func() {
			reconciler = &GlobalRetentionPolicyReconciler{}
		})

		It("should build basic retention script", func() {
			keepLast := int32(10)
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"daily"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}

			script := reconciler.buildRetentionScript(policy)
			Expect(script).To(ContainSubstring("set -e"))
			Expect(script).To(ContainSubstring("restic forget"))
			Expect(script).To(ContainSubstring("--tag daily"))
			Expect(script).To(ContainSubstring("--keep-last 10"))
		})

		It("should include prune command when enabled", func() {
			keepLast := int32(10)
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					Prune: true,
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}

			script := reconciler.buildRetentionScript(policy)
			Expect(script).To(ContainSubstring("restic prune"))
		})

		It("should not include prune command when disabled", func() {
			keepLast := int32(10)
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					Prune: false,
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}

			script := reconciler.buildRetentionScript(policy)
			Expect(script).NotTo(ContainSubstring("restic prune"))
		})

		It("should include hostname filter when specified", func() {
			keepLast := int32(10)
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Hostname: "my-host",
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}

			script := reconciler.buildRetentionScript(policy)
			Expect(script).To(ContainSubstring("--host my-host"))
		})

		It("should include all retention options", func() {
			keepLast := int32(5)
			keepHourly := int32(24)
			keepDaily := int32(7)
			keepWeekly := int32(4)
			keepMonthly := int32(12)
			keepYearly := int32(3)

			policy := &backupv1alpha1.GlobalRetentionPolicy{
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast:    &keepLast,
								KeepHourly:  &keepHourly,
								KeepDaily:   &keepDaily,
								KeepWeekly:  &keepWeekly,
								KeepMonthly: &keepMonthly,
								KeepYearly:  &keepYearly,
							},
						},
					},
				},
			}

			script := reconciler.buildRetentionScript(policy)
			Expect(script).To(ContainSubstring("--keep-last 5"))
			Expect(script).To(ContainSubstring("--keep-hourly 24"))
			Expect(script).To(ContainSubstring("--keep-daily 7"))
			Expect(script).To(ContainSubstring("--keep-weekly 4"))
			Expect(script).To(ContainSubstring("--keep-monthly 12"))
			Expect(script).To(ContainSubstring("--keep-yearly 3"))
		})

		It("should handle multiple policies", func() {
			keepLast := int32(10)
			keepDaily := int32(7)
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"app1"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"app2"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepDaily: &keepDaily,
							},
						},
					},
				},
			}

			script := reconciler.buildRetentionScript(policy)
			Expect(strings.Count(script, "restic forget")).To(Equal(2))
			Expect(script).To(ContainSubstring("--tag app1"))
			Expect(script).To(ContainSubstring("--tag app2"))
			Expect(script).To(ContainSubstring("--keep-last 10"))
			Expect(script).To(ContainSubstring("--keep-daily 7"))
		})

		It("should handle multiple tags in selector", func() {
			keepLast := int32(10)
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					Policies: []backupv1alpha1.RetentionPolicyEntry{
						{
							Selector: backupv1alpha1.RetentionSelector{
								Tags: []string{"tag1", "tag2", "tag3"},
							},
							Retention: backupv1alpha1.RetentionPolicy{
								KeepLast: &keepLast,
							},
						},
					},
				},
			}

			script := reconciler.buildRetentionScript(policy)
			Expect(script).To(ContainSubstring("--tag tag1"))
			Expect(script).To(ContainSubstring("--tag tag2"))
			Expect(script).To(ContainSubstring("--tag tag3"))
		})
	})

	Context("calculateNextRun helper function", func() {
		var reconciler *GlobalRetentionPolicyReconciler

		BeforeEach(func() {
			reconciler = &GlobalRetentionPolicyReconciler{}
		})

		It("should calculate next run time for valid cron schedule", func() {
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					Schedule: "0 3 * * *", // Daily at 3am
				},
			}

			nextRun := reconciler.calculateNextRun(policy)
			Expect(nextRun).NotTo(BeNil())
			Expect(nextRun.Time.After(time.Now())).To(BeTrue())
		})

		It("should return nil for invalid cron schedule", func() {
			policy := &backupv1alpha1.GlobalRetentionPolicy{
				Spec: backupv1alpha1.GlobalRetentionPolicySpec{
					Schedule: "invalid-schedule",
				},
			}

			nextRun := reconciler.calculateNextRun(policy)
			Expect(nextRun).To(BeNil())
		})
	})
})
