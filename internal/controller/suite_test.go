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
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	backupv1alpha1 "github.com/madic-creates/restic-backup-operator/api/v1alpha1"
	"github.com/madic-creates/restic-backup-operator/internal/restic"
)

// MockExecutor is a test executor that returns success for all operations
type MockExecutor struct{}

func (m *MockExecutor) Init(_ context.Context, _ restic.Credentials) error {
	return nil
}

func (m *MockExecutor) Unlock(_ context.Context, _ restic.Credentials) error {
	return nil
}

func (m *MockExecutor) Check(_ context.Context, _ restic.Credentials) (*restic.CheckResult, error) {
	return &restic.CheckResult{Success: true}, nil
}

func (m *MockExecutor) Stats(_ context.Context, _ restic.Credentials, _ restic.StatsOptions) (*restic.RepoStats, error) {
	return &restic.RepoStats{
		TotalSize:      1024,
		TotalFileCount: 10,
		SnapshotCount:  1,
	}, nil
}

func (m *MockExecutor) Snapshots(_ context.Context, _ restic.Credentials) ([]restic.Snapshot, error) {
	return []restic.Snapshot{}, nil
}

func (m *MockExecutor) Backup(_ context.Context, _ restic.Credentials, _ restic.BackupOptions) (*restic.BackupResult, error) {
	return &restic.BackupResult{}, nil
}

func (m *MockExecutor) Restore(_ context.Context, _ restic.Credentials, _ restic.RestoreOptions) (*restic.RestoreResult, error) {
	return &restic.RestoreResult{}, nil
}

func (m *MockExecutor) Forget(_ context.Context, _ restic.Credentials, _ restic.ForgetOptions) (*restic.ForgetResult, error) {
	return &restic.ForgetResult{}, nil
}

func (m *MockExecutor) Prune(_ context.Context, _ restic.Credentials) (*restic.PruneResult, error) {
	return &restic.PruneResult{}, nil
}

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = backupv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Start the manager
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&ResticRepositoryReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("resticrepository-controller"),
		Executor: &MockExecutor{},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ResticBackupReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("resticbackup-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&ResticRestoreReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("resticrestore-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&GlobalRetentionPolicyReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("globalretentionpolicy-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
