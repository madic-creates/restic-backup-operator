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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	backupv1alpha1 "github.com/madic-creates/restic-backup-operator/api/v1alpha1"
)

var _ = Describe("ResticRepository Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a ResticRepository", func() {
		var (
			testNamespace string
			repositoryKey types.NamespacedName
			secretKey     types.NamespacedName
		)

		BeforeEach(func() {
			testNamespace = "test-repo-" + randString(5)
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: testNamespace},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

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
			// Clean up resources
			repository := &backupv1alpha1.ResticRepository{}
			if err := k8sClient.Get(ctx, repositoryKey, repository); err == nil {
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

		It("should set NotReady condition when credentials secret does not exist", func() {
			// Create the ResticRepository without the secret
			repository := &backupv1alpha1.ResticRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repositoryKey.Name,
					Namespace: repositoryKey.Namespace,
				},
				Spec: backupv1alpha1.ResticRepositorySpec{
					RepositoryURL: "local:/tmp/test-repo",
					CredentialsSecretRef: backupv1alpha1.SecretKeySelector{
						Name: "nonexistent-secret",
					},
				},
			}
			Expect(k8sClient.Create(ctx, repository)).To(Succeed())

			// Verify NotReady condition is set
			Eventually(func() bool {
				repo := &backupv1alpha1.ResticRepository{}
				if err := k8sClient.Get(ctx, repositoryKey, repo); err != nil {
					return false
				}
				for _, cond := range repo.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == metav1.ConditionFalse {
						return cond.Reason == "CredentialsNotFound"
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("should set NotReady condition when secret is missing RESTIC_PASSWORD", func() {
			// Create the credentials secret without RESTIC_PASSWORD
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretKey.Name,
					Namespace: secretKey.Namespace,
				},
				Data: map[string][]byte{
					"OTHER_KEY": []byte("some-value"),
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

			// Verify NotReady condition is set
			Eventually(func() bool {
				repo := &backupv1alpha1.ResticRepository{}
				if err := k8sClient.Get(ctx, repositoryKey, repo); err != nil {
					return false
				}
				for _, cond := range repo.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == metav1.ConditionFalse {
						return cond.Reason == "CredentialsNotFound"
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

		It("should delete successfully", func() {
			// Create the credentials secret first
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

			// Delete the repository
			Expect(k8sClient.Delete(ctx, repository)).To(Succeed())

			// Verify repository is eventually deleted
			Eventually(func() bool {
				repo := &backupv1alpha1.ResticRepository{}
				err := k8sClient.Get(ctx, repositoryKey, repo)
				return err != nil
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("formatBytes helper function", func() {
		It("should format bytes correctly", func() {
			Expect(formatBytes(0)).To(Equal("0 B"))
			Expect(formatBytes(512)).To(Equal("512 B"))
			Expect(formatBytes(1024)).To(Equal("1.0 KiB"))
			Expect(formatBytes(1536)).To(Equal("1.5 KiB"))
			Expect(formatBytes(1048576)).To(Equal("1.0 MiB"))
			Expect(formatBytes(1073741824)).To(Equal("1.0 GiB"))
			Expect(formatBytes(1099511627776)).To(Equal("1.0 TiB"))
		})
	})
})

// randString generates a random string of lowercase letters
func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
