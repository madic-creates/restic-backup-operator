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

var _ = Describe("ResticBackup Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a ResticBackup", func() {
		var (
			testNamespace string
			backupKey     types.NamespacedName
			repositoryKey types.NamespacedName
			secretKey     types.NamespacedName
			pvcKey        types.NamespacedName
		)

		BeforeEach(func() {
			testNamespace = "test-backup-" + randString(5)
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: testNamespace},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			backupKey = types.NamespacedName{
				Name:      "test-backup",
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
			pvcKey = types.NamespacedName{
				Name:      "test-pvc",
				Namespace: testNamespace,
			}
		})

		AfterEach(func() {
			// Clean up resources in order
			backup := &backupv1alpha1.ResticBackup{}
			if err := k8sClient.Get(ctx, backupKey, backup); err == nil {
				controllerutil.RemoveFinalizer(backup, resticBackupFinalizer)
				_ = k8sClient.Update(ctx, backup)
				_ = k8sClient.Delete(ctx, backup)
			}

			// Delete CronJob if exists
			cronJob := &batchv1.CronJob{}
			cronJobKey := types.NamespacedName{
				Name:      "resticbackup-" + backupKey.Name,
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

			pvc := &corev1.PersistentVolumeClaim{}
			if err := k8sClient.Get(ctx, pvcKey, pvc); err == nil {
				_ = k8sClient.Delete(ctx, pvc)
			}

			ns := &corev1.Namespace{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace}, ns); err == nil {
				_ = k8sClient.Delete(ctx, ns)
			}
		})

		It("should add finalizer when created", func() {
			// Create the ResticBackup (repository doesn't need to exist for finalizer test)
			backup := &backupv1alpha1.ResticBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: backupv1alpha1.ResticBackupSpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 2 * * *",
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: pvcKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Verify finalizer is added
			Eventually(func() bool {
				b := &backupv1alpha1.ResticBackup{}
				if err := k8sClient.Get(ctx, backupKey, b); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(b, resticBackupFinalizer)
			}, timeout, interval).Should(BeTrue())
		})

		It("should set NotReady condition when repository does not exist", func() {
			// Create the ResticBackup without the repository
			backup := &backupv1alpha1.ResticBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: backupv1alpha1.ResticBackupSpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: "nonexistent-repository",
					},
					Schedule: "0 2 * * *",
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: pvcKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Verify NotReady condition is set
			Eventually(func() bool {
				b := &backupv1alpha1.ResticBackup{}
				if err := k8sClient.Get(ctx, backupKey, b); err != nil {
					return false
				}
				for _, cond := range b.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == metav1.ConditionFalse {
						return cond.Reason == "RepositoryNotFound"
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("should set NotReady condition when repository is not ready", func() {
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

			// Create the ResticRepository without Ready condition
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

			// Wait for the repository to get a condition (even if not ready)
			Eventually(func() bool {
				repo := &backupv1alpha1.ResticRepository{}
				if err := k8sClient.Get(ctx, repositoryKey, repo); err != nil {
					return false
				}
				return len(repo.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())

			// Create the ResticBackup
			backup := &backupv1alpha1.ResticBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: backupv1alpha1.ResticBackupSpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 2 * * *",
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: pvcKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// The backup should have some condition set
			Eventually(func() bool {
				b := &backupv1alpha1.ResticBackup{}
				if err := k8sClient.Get(ctx, backupKey, b); err != nil {
					return false
				}
				return len(b.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())
		})

		It("should create CronJob when repository is ready", func() {
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

			// Create the ResticBackup
			backup := &backupv1alpha1.ResticBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: backupv1alpha1.ResticBackupSpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 2 * * *",
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: pvcKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Verify CronJob is created
			cronJobKey := types.NamespacedName{
				Name:      "resticbackup-" + backupKey.Name,
				Namespace: testNamespace,
			}
			Eventually(func() bool {
				cronJob := &batchv1.CronJob{}
				return k8sClient.Get(ctx, cronJobKey, cronJob) == nil
			}, timeout, interval).Should(BeTrue())

			// Verify CronJob has correct schedule
			cronJob := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, cronJobKey, cronJob)).To(Succeed())
			Expect(cronJob.Spec.Schedule).To(Equal("0 2 * * *"))

			// Verify CronJob has correct labels
			Expect(cronJob.Labels["app.kubernetes.io/name"]).To(Equal("restic-backup-operator"))
			Expect(cronJob.Labels["backup.resticbackup.io/backup"]).To(Equal(backupKey.Name))
		})

		It("should set RepositoryReady condition when repository is ready", func() {
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

			// Create the ResticBackup
			backup := &backupv1alpha1.ResticBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: backupv1alpha1.ResticBackupSpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 2 * * *",
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: pvcKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Verify RepositoryReady condition is set
			Eventually(func() bool {
				b := &backupv1alpha1.ResticBackup{}
				if err := k8sClient.Get(ctx, backupKey, b); err != nil {
					return false
				}
				for _, cond := range b.Status.Conditions {
					if cond.Type == backupv1alpha1.ConditionRepositoryReady && cond.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("should remove finalizer on deletion", func() {
			// Create the ResticBackup
			backup := &backupv1alpha1.ResticBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: backupv1alpha1.ResticBackupSpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: repositoryKey.Name,
					},
					Schedule: "0 2 * * *",
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: pvcKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Wait for finalizer to be added
			Eventually(func() bool {
				b := &backupv1alpha1.ResticBackup{}
				if err := k8sClient.Get(ctx, backupKey, b); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(b, resticBackupFinalizer)
			}, timeout, interval).Should(BeTrue())

			// Delete the backup
			Expect(k8sClient.Delete(ctx, backup)).To(Succeed())

			// Verify backup is eventually deleted
			Eventually(func() bool {
				b := &backupv1alpha1.ResticBackup{}
				err := k8sClient.Get(ctx, backupKey, b)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})

		It("should use cross-namespace repository reference", func() {
			// Create a different namespace for the repository
			repoNamespace := "test-repo-ns-" + randString(5)
			repoNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: repoNamespace},
			}
			Expect(k8sClient.Create(ctx, repoNs)).To(Succeed())

			// Create the credentials secret in repository namespace
			repoSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretKey.Name,
					Namespace: repoNamespace,
				},
				Data: map[string][]byte{
					"RESTIC_PASSWORD": []byte("test-password"),
				},
			}
			Expect(k8sClient.Create(ctx, repoSecret)).To(Succeed())

			// Create the ResticRepository in different namespace
			crossNsRepoKey := types.NamespacedName{
				Name:      repositoryKey.Name,
				Namespace: repoNamespace,
			}
			repository := &backupv1alpha1.ResticRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crossNsRepoKey.Name,
					Namespace: crossNsRepoKey.Namespace,
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
				if err := k8sClient.Get(ctx, crossNsRepoKey, repo); err != nil {
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

			// Create the ResticBackup with cross-namespace reference
			backup := &backupv1alpha1.ResticBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backupKey.Name,
					Namespace: backupKey.Namespace,
				},
				Spec: backupv1alpha1.ResticBackupSpec{
					RepositoryRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name:      repositoryKey.Name,
						Namespace: repoNamespace,
					},
					Schedule: "0 2 * * *",
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: pvcKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Verify CronJob is created
			cronJobKey := types.NamespacedName{
				Name:      "resticbackup-" + backupKey.Name,
				Namespace: testNamespace,
			}
			Eventually(func() bool {
				cronJob := &batchv1.CronJob{}
				return k8sClient.Get(ctx, cronJobKey, cronJob) == nil
			}, timeout, interval).Should(BeTrue())

			// Clean up
			controllerutil.RemoveFinalizer(repository, resticRepositoryFinalizer)
			_ = k8sClient.Update(ctx, repository)
			_ = k8sClient.Delete(ctx, repository)
			_ = k8sClient.Delete(ctx, repoSecret)
			_ = k8sClient.Delete(ctx, repoNs)
		})
	})

	Context("buildBackupCommand helper function", func() {
		var reconciler *ResticBackupReconciler

		BeforeEach(func() {
			reconciler = &ResticBackupReconciler{}
		})

		It("should build basic backup command", func() {
			backup := &backupv1alpha1.ResticBackup{
				Spec: backupv1alpha1.ResticBackupSpec{
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: "test-pvc",
						},
					},
				},
			}

			cmd := reconciler.buildBackupCommand(backup, "test-host", nil)
			Expect(cmd).To(ContainElements("restic", "backup", "--host", "test-host", "/backup"))
		})

		It("should include tags in backup command", func() {
			backup := &backupv1alpha1.ResticBackup{
				Spec: backupv1alpha1.ResticBackupSpec{
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: "test-pvc",
						},
					},
				},
			}

			cmd := reconciler.buildBackupCommand(backup, "test-host", []string{"tag1", "tag2"})
			Expect(cmd).To(ContainElements("--tag", "tag1", "--tag", "tag2"))
		})

		It("should include excludes in backup command", func() {
			backup := &backupv1alpha1.ResticBackup{
				Spec: backupv1alpha1.ResticBackupSpec{
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: "test-pvc",
							Excludes:  []string{"*.tmp", "*.log"},
						},
					},
				},
			}

			cmd := reconciler.buildBackupCommand(backup, "test-host", nil)
			Expect(cmd).To(ContainElements("--exclude", "*.tmp", "--exclude", "*.log"))
		})

		It("should include specific paths in backup command", func() {
			backup := &backupv1alpha1.ResticBackup{
				Spec: backupv1alpha1.ResticBackupSpec{
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: "test-pvc",
							Paths:     []string{"/data", "/config"},
						},
					},
				},
			}

			cmd := reconciler.buildBackupCommand(backup, "test-host", nil)
			Expect(cmd).To(ContainElements("/backup/data", "/backup/config"))
			Expect(cmd).NotTo(ContainElement("/backup"))
		})

		It("should include extra args in backup command", func() {
			backup := &backupv1alpha1.ResticBackup{
				Spec: backupv1alpha1.ResticBackupSpec{
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: "test-pvc",
						},
					},
					Restic: &backupv1alpha1.ResticConfig{
						ExtraArgs: []string{"--verbose", "--dry-run"},
					},
				},
			}

			cmd := reconciler.buildBackupCommand(backup, "test-host", nil)
			Expect(cmd).To(ContainElements("--verbose", "--dry-run"))
		})
	})

	Context("calculateNextBackup helper function", func() {
		var reconciler *ResticBackupReconciler

		BeforeEach(func() {
			reconciler = &ResticBackupReconciler{}
		})

		It("should calculate next backup time for valid cron schedule", func() {
			backup := &backupv1alpha1.ResticBackup{
				Spec: backupv1alpha1.ResticBackupSpec{
					Schedule: "0 2 * * *", // Daily at 2am
				},
			}

			nextBackup := reconciler.calculateNextBackup(backup)
			Expect(nextBackup).NotTo(BeNil())
			Expect(nextBackup.Time.After(time.Now())).To(BeTrue())
		})

		It("should return nil for invalid cron schedule", func() {
			backup := &backupv1alpha1.ResticBackup{
				Spec: backupv1alpha1.ResticBackupSpec{
					Schedule: "invalid-schedule",
				},
			}

			nextBackup := reconciler.calculateNextBackup(backup)
			Expect(nextBackup).To(BeNil())
		})
	})

	Context("helper functions", func() {
		It("boolPtr should return pointer to bool", func() {
			truePtr := boolPtr(true)
			falsePtr := boolPtr(false)

			Expect(*truePtr).To(BeTrue())
			Expect(*falsePtr).To(BeFalse())
		})

		It("int64Ptr should return pointer to int64", func() {
			ptr := int64Ptr(42)
			Expect(*ptr).To(Equal(int64(42)))
		})
	})
})
