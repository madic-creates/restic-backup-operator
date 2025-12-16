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

var _ = Describe("ResticRestore Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a ResticRestore", func() {
		var (
			testNamespace string
			restoreKey    types.NamespacedName
			backupKey     types.NamespacedName
			repositoryKey types.NamespacedName
			secretKey     types.NamespacedName
			targetPVCKey  types.NamespacedName
		)

		BeforeEach(func() {
			testNamespace = "test-restore-" + randString(5)
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: testNamespace},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			restoreKey = types.NamespacedName{
				Name:      "test-restore",
				Namespace: testNamespace,
			}
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
			targetPVCKey = types.NamespacedName{
				Name:      "test-target-pvc",
				Namespace: testNamespace,
			}
		})

		AfterEach(func() {
			// Clean up resources in order
			restore := &backupv1alpha1.ResticRestore{}
			if err := k8sClient.Get(ctx, restoreKey, restore); err == nil {
				controllerutil.RemoveFinalizer(restore, resticRestoreFinalizer)
				_ = k8sClient.Update(ctx, restore)
				_ = k8sClient.Delete(ctx, restore)
			}

			// Delete Job if exists
			job := &batchv1.Job{}
			jobKey := types.NamespacedName{
				Name:      "resticrestore-" + restoreKey.Name,
				Namespace: testNamespace,
			}
			if err := k8sClient.Get(ctx, jobKey, job); err == nil {
				_ = k8sClient.Delete(ctx, job)
			}

			backup := &backupv1alpha1.ResticBackup{}
			if err := k8sClient.Get(ctx, backupKey, backup); err == nil {
				controllerutil.RemoveFinalizer(backup, resticBackupFinalizer)
				_ = k8sClient.Update(ctx, backup)
				_ = k8sClient.Delete(ctx, backup)
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
			// Create the ResticRestore (backup doesn't need to exist for finalizer test)
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreKey.Name,
					Namespace: restoreKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					BackupRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: backupKey.Name,
					},
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: targetPVCKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).To(Succeed())

			// Verify finalizer is added
			Eventually(func() bool {
				r := &backupv1alpha1.ResticRestore{}
				if err := k8sClient.Get(ctx, restoreKey, r); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(r, resticRestoreFinalizer)
			}, timeout, interval).Should(BeTrue())
		})

		It("should transition from Pending to Failed when backup does not exist", func() {
			// Create the ResticRestore without backup existing
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreKey.Name,
					Namespace: restoreKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					BackupRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: backupKey.Name,
					},
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: targetPVCKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).To(Succeed())

			// Since backup doesn't exist, controller transitions from Pending to Failed
			Eventually(func() backupv1alpha1.RestorePhase {
				r := &backupv1alpha1.ResticRestore{}
				if err := k8sClient.Get(ctx, restoreKey, r); err != nil {
					return ""
				}
				return r.Status.Phase
			}, timeout, interval).Should(Equal(backupv1alpha1.RestorePhaseFailed))
		})

		It("should create restore job when all references exist", func() {
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
							ClaimName: "source-pvc",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Create the ResticRestore
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreKey.Name,
					Namespace: restoreKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					BackupRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: backupKey.Name,
					},
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: targetPVCKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).To(Succeed())

			// Verify restore job is created
			jobKey := types.NamespacedName{
				Name:      "resticrestore-" + restoreKey.Name,
				Namespace: testNamespace,
			}
			Eventually(func() bool {
				job := &batchv1.Job{}
				return k8sClient.Get(ctx, jobKey, job) == nil
			}, timeout, interval).Should(BeTrue())

			// Verify phase changes to InProgress
			Eventually(func() backupv1alpha1.RestorePhase {
				r := &backupv1alpha1.ResticRestore{}
				if err := k8sClient.Get(ctx, restoreKey, r); err != nil {
					return ""
				}
				return r.Status.Phase
			}, timeout, interval).Should(Equal(backupv1alpha1.RestorePhaseInProgress))

			// Verify job reference is set
			Eventually(func() bool {
				r := &backupv1alpha1.ResticRestore{}
				if err := k8sClient.Get(ctx, restoreKey, r); err != nil {
					return false
				}
				return r.Status.JobRef != nil && r.Status.JobRef.Name == jobKey.Name
			}, timeout, interval).Should(BeTrue())
		})

		It("should use snapshot ID when specified", func() {
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
							ClaimName: "source-pvc",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Create the ResticRestore with specific snapshot ID
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreKey.Name,
					Namespace: restoreKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					BackupRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: backupKey.Name,
					},
					SnapshotID: "abc12345",
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: targetPVCKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).To(Succeed())

			// Verify restored snapshot is set in status
			Eventually(func() string {
				r := &backupv1alpha1.ResticRestore{}
				if err := k8sClient.Get(ctx, restoreKey, r); err != nil {
					return ""
				}
				return r.Status.RestoredSnapshot
			}, timeout, interval).Should(Equal("abc12345"))
		})

		It("should remove finalizer on deletion", func() {
			// Create the ResticRestore
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreKey.Name,
					Namespace: restoreKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					BackupRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: backupKey.Name,
					},
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: targetPVCKey.Name,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).To(Succeed())

			// Wait for finalizer to be added
			Eventually(func() bool {
				r := &backupv1alpha1.ResticRestore{}
				if err := k8sClient.Get(ctx, restoreKey, r); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(r, resticRestoreFinalizer)
			}, timeout, interval).Should(BeTrue())

			// Delete the restore
			Expect(k8sClient.Delete(ctx, restore)).To(Succeed())

			// Verify restore is eventually deleted
			Eventually(func() bool {
				r := &backupv1alpha1.ResticRestore{}
				err := k8sClient.Get(ctx, restoreKey, r)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})

		It("should include verify flag when options.verify is true", func() {
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
							ClaimName: "source-pvc",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			// Create the ResticRestore with verify option
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      restoreKey.Name,
					Namespace: restoreKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					BackupRef: backupv1alpha1.CrossNamespaceObjectReference{
						Name: backupKey.Name,
					},
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: targetPVCKey.Name,
						},
					},
					Options: &backupv1alpha1.RestoreOptions{
						Verify: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, restore)).To(Succeed())

			// Verify restore job is created
			jobKey := types.NamespacedName{
				Name:      "resticrestore-" + restoreKey.Name,
				Namespace: testNamespace,
			}
			Eventually(func() bool {
				job := &batchv1.Job{}
				return k8sClient.Get(ctx, jobKey, job) == nil
			}, timeout, interval).Should(BeTrue())

			// Verify the job command contains --verify
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, jobKey, job)).To(Succeed())
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElement("--verify"))
		})
	})

	Context("buildRestoreJob helper function", func() {
		var reconciler *ResticRestoreReconciler

		BeforeEach(func() {
			reconciler = &ResticRestoreReconciler{}
		})

		It("should build basic restore job", func() {
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore",
					Namespace: "default",
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: "target-pvc",
						},
					},
				},
			}
			backup := &backupv1alpha1.ResticBackup{
				Spec: backupv1alpha1.ResticBackupSpec{
					Source: backupv1alpha1.BackupSource{
						PVC: &backupv1alpha1.PVCSource{
							ClaimName: "source-pvc",
						},
					},
				},
			}
			repository := &backupv1alpha1.ResticRepository{
				Spec: backupv1alpha1.ResticRepositorySpec{
					RepositoryURL: "local:/tmp/test-repo",
					CredentialsSecretRef: backupv1alpha1.SecretKeySelector{
						Name: "test-credentials",
					},
				},
			}

			job := reconciler.buildRestoreJob(restore, backup, repository, "latest")
			Expect(job.Name).To(Equal("resticrestore-test-restore"))
			Expect(job.Namespace).To(Equal("default"))
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElements("restic", "restore", "latest", "--target", "/restore"))
		})

		It("should include include paths in restore command", func() {
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore",
					Namespace: "default",
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: "target-pvc",
						},
					},
					IncludePaths: []string{"/data", "/config"},
				},
			}
			backup := &backupv1alpha1.ResticBackup{}
			repository := &backupv1alpha1.ResticRepository{
				Spec: backupv1alpha1.ResticRepositorySpec{
					CredentialsSecretRef: backupv1alpha1.SecretKeySelector{
						Name: "test-credentials",
					},
				},
			}

			job := reconciler.buildRestoreJob(restore, backup, repository, "latest")
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElements("--include", "/data", "--include", "/config"))
		})

		It("should include exclude paths in restore command", func() {
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore",
					Namespace: "default",
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: "target-pvc",
						},
					},
					ExcludePaths: []string{"*.tmp", "*.log"},
				},
			}
			backup := &backupv1alpha1.ResticBackup{}
			repository := &backupv1alpha1.ResticRepository{
				Spec: backupv1alpha1.ResticRepositorySpec{
					CredentialsSecretRef: backupv1alpha1.SecretKeySelector{
						Name: "test-credentials",
					},
				},
			}

			job := reconciler.buildRestoreJob(restore, backup, repository, "latest")
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElements("--exclude", "*.tmp", "--exclude", "*.log"))
		})

		It("should use custom restic image from backup spec", func() {
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore",
					Namespace: "default",
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					Target: backupv1alpha1.RestoreTarget{
						PVC: &backupv1alpha1.PVCTarget{
							ClaimName: "target-pvc",
						},
					},
				},
			}
			backup := &backupv1alpha1.ResticBackup{
				Spec: backupv1alpha1.ResticBackupSpec{
					Restic: &backupv1alpha1.ResticConfig{
						Image: "custom/restic:1.0.0",
					},
				},
			}
			repository := &backupv1alpha1.ResticRepository{
				Spec: backupv1alpha1.ResticRepositorySpec{
					CredentialsSecretRef: backupv1alpha1.SecretKeySelector{
						Name: "test-credentials",
					},
				},
			}

			job := reconciler.buildRestoreJob(restore, backup, repository, "latest")
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal("custom/restic:1.0.0"))
		})

		It("should use NewPVC target when specified", func() {
			restore := &backupv1alpha1.ResticRestore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-restore",
					Namespace: "default",
				},
				Spec: backupv1alpha1.ResticRestoreSpec{
					Target: backupv1alpha1.RestoreTarget{
						NewPVC: &backupv1alpha1.NewPVCTarget{
							Name: "new-target-pvc",
							Size: "10Gi",
						},
					},
				},
			}
			backup := &backupv1alpha1.ResticBackup{}
			repository := &backupv1alpha1.ResticRepository{
				Spec: backupv1alpha1.ResticRepositorySpec{
					CredentialsSecretRef: backupv1alpha1.SecretKeySelector{
						Name: "test-credentials",
					},
				},
			}

			job := reconciler.buildRestoreJob(restore, backup, repository, "latest")
			// Check volume source uses new PVC name
			Expect(job.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName).To(Equal("new-target-pvc"))
		})
	})
})
