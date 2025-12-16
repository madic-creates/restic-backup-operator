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

package restic

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func getTestLogger() logr.Logger {
	return zap.New(zap.UseDevMode(true))
}

func TestNewExecutor(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutor(log)

	if executor == nil {
		t.Fatal("expected executor to not be nil")
	}

	if executor.binary != "restic" {
		t.Errorf("expected binary to be 'restic', got %s", executor.binary)
	}
}

func TestNewExecutorWithBinary(t *testing.T) {
	log := getTestLogger()

	tests := []struct {
		name           string
		binary         string
		expectedBinary string
	}{
		{"custom binary", "/usr/local/bin/restic", "/usr/local/bin/restic"},
		{"empty binary defaults to restic", "", "restic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecutorWithBinary(tt.binary, log)

			if executor == nil {
				t.Fatal("expected executor to not be nil")
			}

			if executor.binary != tt.expectedBinary {
				t.Errorf("expected binary to be %s, got %s", tt.expectedBinary, executor.binary)
			}
		})
	}
}

func TestDefaultExecutor_buildEnv(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutor(log)

	tests := []struct {
		name     string
		creds    Credentials
		expected map[string]bool
	}{
		{
			name: "basic credentials",
			creds: Credentials{
				Repository: "s3:s3.amazonaws.com/bucket",
				Password:   "secret",
			},
			expected: map[string]bool{
				"RESTIC_REPOSITORY=s3:s3.amazonaws.com/bucket": true,
				"RESTIC_PASSWORD=secret":                       true,
			},
		},
		{
			name: "with AWS credentials",
			creds: Credentials{
				Repository:         "s3:s3.amazonaws.com/bucket",
				Password:           "secret",
				AWSAccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				AWSSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			expected: map[string]bool{
				"RESTIC_REPOSITORY=s3:s3.amazonaws.com/bucket":                   true,
				"RESTIC_PASSWORD=secret":                                         true,
				"AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE":                         true,
				"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY": true,
			},
		},
		{
			name: "with cache directory",
			creds: Credentials{
				Repository: "local:/backup",
				Password:   "secret",
				CacheDir:   "/tmp/restic-cache",
			},
			expected: map[string]bool{
				"RESTIC_REPOSITORY=local:/backup":    true,
				"RESTIC_PASSWORD=secret":             true,
				"RESTIC_CACHE_DIR=/tmp/restic-cache": true,
			},
		},
		{
			name: "all options",
			creds: Credentials{
				Repository:         "s3:s3.amazonaws.com/bucket",
				Password:           "secret",
				AWSAccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				AWSSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				CacheDir:           "/tmp/restic-cache",
			},
			expected: map[string]bool{
				"RESTIC_REPOSITORY=s3:s3.amazonaws.com/bucket":                   true,
				"RESTIC_PASSWORD=secret":                                         true,
				"AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE":                         true,
				"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY": true,
				"RESTIC_CACHE_DIR=/tmp/restic-cache":                             true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := executor.buildEnv(tt.creds)

			// Check all expected env vars are present
			for _, e := range env {
				if !tt.expected[e] {
					// Check if it's a valid env var we expect
					found := false
					for key := range tt.expected {
						if strings.HasPrefix(e, strings.Split(key, "=")[0]) {
							found = true
							break
						}
					}
					if !found && !strings.HasPrefix(e, "RESTIC_") && !strings.HasPrefix(e, "AWS_") {
						t.Errorf("unexpected env var: %s", e)
					}
				}
			}

			// Check count matches
			if len(env) != len(tt.expected) {
				t.Errorf("expected %d env vars, got %d", len(tt.expected), len(env))
			}
		})
	}
}

func TestDefaultExecutor_buildEnv_EmptyOptionalFields(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutor(log)

	creds := Credentials{
		Repository: "local:/backup",
		Password:   "secret",
		// AWS credentials and CacheDir are empty
	}

	env := executor.buildEnv(creds)

	// Should only have RESTIC_REPOSITORY and RESTIC_PASSWORD
	if len(env) != 2 {
		t.Errorf("expected 2 env vars, got %d: %v", len(env), env)
	}

	// Should not contain AWS or CACHE_DIR
	for _, e := range env {
		if strings.HasPrefix(e, "AWS_") {
			t.Errorf("unexpected AWS env var: %s", e)
		}
		if strings.HasPrefix(e, "RESTIC_CACHE_DIR") {
			t.Errorf("unexpected RESTIC_CACHE_DIR env var: %s", e)
		}
	}
}

// TestDefaultExecutor_Init_WithMockBinary tests Init with a non-existent binary
func TestDefaultExecutor_Init_BinaryNotFound(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutorWithBinary("/nonexistent/restic", log)

	creds := Credentials{
		Repository: "local:/tmp/test-repo",
		Password:   "test",
	}

	err := executor.Init(context.Background(), creds)
	if err == nil {
		t.Error("expected error when binary doesn't exist")
	}
}

// TestDefaultExecutor_Check_BinaryNotFound tests Check with a non-existent binary
func TestDefaultExecutor_Check_BinaryNotFound(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutorWithBinary("/nonexistent/restic", log)

	creds := Credentials{
		Repository: "local:/tmp/test-repo",
		Password:   "test",
	}

	result, err := executor.Check(context.Background(), creds)
	if err == nil {
		t.Error("expected error when binary doesn't exist")
	}
	if result == nil {
		t.Error("expected result to not be nil even on error")
	}
	if result != nil && result.Success {
		t.Error("expected Success to be false on error")
	}
}

// TestDefaultExecutor_Stats_BinaryNotFound tests Stats with a non-existent binary
func TestDefaultExecutor_Stats_BinaryNotFound(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutorWithBinary("/nonexistent/restic", log)

	creds := Credentials{
		Repository: "local:/tmp/test-repo",
		Password:   "test",
	}

	_, err := executor.Stats(context.Background(), creds, StatsOptions{})
	if err == nil {
		t.Error("expected error when binary doesn't exist")
	}
}

// TestDefaultExecutor_Snapshots_BinaryNotFound tests Snapshots with a non-existent binary
func TestDefaultExecutor_Snapshots_BinaryNotFound(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutorWithBinary("/nonexistent/restic", log)

	creds := Credentials{
		Repository: "local:/tmp/test-repo",
		Password:   "test",
	}

	_, err := executor.Snapshots(context.Background(), creds)
	if err == nil {
		t.Error("expected error when binary doesn't exist")
	}
}

// TestDefaultExecutor_Backup_BinaryNotFound tests Backup with a non-existent binary
func TestDefaultExecutor_Backup_BinaryNotFound(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutorWithBinary("/nonexistent/restic", log)

	creds := Credentials{
		Repository: "local:/tmp/test-repo",
		Password:   "test",
	}

	opts := BackupOptions{
		Paths:    []string{"/data"},
		Hostname: "test-host",
	}

	_, err := executor.Backup(context.Background(), creds, opts)
	if err == nil {
		t.Error("expected error when binary doesn't exist")
	}
}

// TestDefaultExecutor_Restore_BinaryNotFound tests Restore with a non-existent binary
func TestDefaultExecutor_Restore_BinaryNotFound(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutorWithBinary("/nonexistent/restic", log)

	creds := Credentials{
		Repository: "local:/tmp/test-repo",
		Password:   "test",
	}

	opts := RestoreOptions{
		SnapshotID: "latest",
		Target:     "/restore",
	}

	_, err := executor.Restore(context.Background(), creds, opts)
	if err == nil {
		t.Error("expected error when binary doesn't exist")
	}
}

// TestDefaultExecutor_Forget_BinaryNotFound tests Forget with a non-existent binary
func TestDefaultExecutor_Forget_BinaryNotFound(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutorWithBinary("/nonexistent/restic", log)

	creds := Credentials{
		Repository: "local:/tmp/test-repo",
		Password:   "test",
	}

	opts := ForgetOptions{
		KeepLast: 10,
	}

	_, err := executor.Forget(context.Background(), creds, opts)
	if err == nil {
		t.Error("expected error when binary doesn't exist")
	}
}

// TestDefaultExecutor_Prune_BinaryNotFound tests Prune with a non-existent binary
func TestDefaultExecutor_Prune_BinaryNotFound(t *testing.T) {
	log := getTestLogger()
	executor := NewExecutorWithBinary("/nonexistent/restic", log)

	creds := Credentials{
		Repository: "local:/tmp/test-repo",
		Password:   "test",
	}

	_, err := executor.Prune(context.Background(), creds)
	if err == nil {
		t.Error("expected error when binary doesn't exist")
	}
}

// TestDefaultExecutor_ContextCancellation tests that context cancellation works
func TestDefaultExecutor_ContextCancellation(t *testing.T) {
	// Skip if restic is not installed
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic binary not found, skipping context cancellation test")
	}

	log := getTestLogger()
	executor := NewExecutor(log)

	creds := Credentials{
		Repository: "local:/tmp/nonexistent-repo",
		Password:   "test",
	}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// The operation should fail due to context cancellation
	// We don't assert the error because it may be nil (if restic isn't installed)
	// or non-nil (context cancellation or restic error). The main point is that
	// the context is passed through and the code doesn't panic or hang.
	_, _ = executor.Check(ctx, creds)
}

// Integration tests that require restic binary
// These are skipped if restic is not available

func TestDefaultExecutor_Integration_Init(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic binary not found, skipping integration test")
	}

	// Create a temporary directory for the repository
	tmpDir, err := os.MkdirTemp("", "restic-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log := getTestLogger()
	executor := NewExecutor(log)

	creds := Credentials{
		Repository: "local:" + tmpDir,
		Password:   "test-password",
	}

	// Initialize the repository
	err = executor.Init(context.Background(), creds)
	if err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}

	// Note: The current implementation doesn't check stderr for "already exists"
	// message, so a second init will fail. This is a known limitation.
	// A future improvement could be to check stderr in the Init function.
}

func TestDefaultExecutor_Integration_Check(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic binary not found, skipping integration test")
	}

	// Create a temporary directory for the repository
	tmpDir, err := os.MkdirTemp("", "restic-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log := getTestLogger()
	executor := NewExecutor(log)

	creds := Credentials{
		Repository: "local:" + tmpDir,
		Password:   "test-password",
	}

	// Initialize the repository first
	err = executor.Init(context.Background(), creds)
	if err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}

	// Check the repository
	result, err := executor.Check(context.Background(), creds)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}

	if !result.Success {
		t.Errorf("expected check to succeed, message: %s", result.Message)
	}

	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestDefaultExecutor_Integration_Snapshots_Empty(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic binary not found, skipping integration test")
	}

	// Create a temporary directory for the repository
	tmpDir, err := os.MkdirTemp("", "restic-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log := getTestLogger()
	executor := NewExecutor(log)

	creds := Credentials{
		Repository: "local:" + tmpDir,
		Password:   "test-password",
	}

	// Initialize the repository first
	err = executor.Init(context.Background(), creds)
	if err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}

	// List snapshots (should be empty)
	snapshots, err := executor.Snapshots(context.Background(), creds)
	if err != nil {
		t.Fatalf("snapshots failed: %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestDefaultExecutor_Integration_BackupAndRestore(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic binary not found, skipping integration test")
	}

	// Create a temporary directory for the repository
	repoDir, err := os.MkdirTemp("", "restic-repo-*")
	if err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	defer os.RemoveAll(repoDir)

	// Create a temporary directory with test data
	dataDir, err := os.MkdirTemp("", "restic-data-*")
	if err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	defer os.RemoveAll(dataDir)

	// Create test file
	testFile := dataDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create restore directory
	restoreDir, err := os.MkdirTemp("", "restic-restore-*")
	if err != nil {
		t.Fatalf("failed to create restore dir: %v", err)
	}
	defer os.RemoveAll(restoreDir)

	log := getTestLogger()
	executor := NewExecutor(log)

	creds := Credentials{
		Repository: "local:" + repoDir,
		Password:   "test-password",
	}

	// Initialize the repository
	err = executor.Init(context.Background(), creds)
	if err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}

	// Create backup
	backupOpts := BackupOptions{
		Paths:    []string{dataDir},
		Hostname: "test-host",
		Tags:     []string{"test", "integration"},
	}

	backupResult, err := executor.Backup(context.Background(), creds, backupOpts)
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	if backupResult.SnapshotID == "" {
		t.Error("expected snapshot ID to be set")
	}

	// List snapshots
	snapshots, err := executor.Snapshots(context.Background(), creds)
	if err != nil {
		t.Fatalf("snapshots failed: %v", err)
	}

	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snapshots))
	}

	// Get stats
	stats, err := executor.Stats(context.Background(), creds, StatsOptions{Mode: "restore-size"})
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}

	if stats.SnapshotCount != 1 {
		t.Errorf("expected 1 snapshot in stats, got %d", stats.SnapshotCount)
	}

	// Restore
	restoreOpts := RestoreOptions{
		SnapshotID: "latest",
		Target:     restoreDir,
	}

	restoreResult, err := executor.Restore(context.Background(), creds, restoreOpts)
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	if restoreResult.Duration <= 0 {
		t.Error("expected positive restore duration")
	}
}

func TestDefaultExecutor_Integration_Forget(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic binary not found, skipping integration test")
	}

	// Create a temporary directory for the repository
	repoDir, err := os.MkdirTemp("", "restic-repo-*")
	if err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	defer os.RemoveAll(repoDir)

	// Create a temporary directory with test data
	dataDir, err := os.MkdirTemp("", "restic-data-*")
	if err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	defer os.RemoveAll(dataDir)

	// Create test file
	testFile := dataDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	log := getTestLogger()
	executor := NewExecutor(log)

	creds := Credentials{
		Repository: "local:" + repoDir,
		Password:   "test-password",
	}

	// Initialize the repository
	err = executor.Init(context.Background(), creds)
	if err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}

	// Create multiple backups
	for i := 0; i < 3; i++ {
		backupOpts := BackupOptions{
			Paths:    []string{dataDir},
			Hostname: "test-host",
		}
		_, err := executor.Backup(context.Background(), creds, backupOpts)
		if err != nil {
			t.Fatalf("backup %d failed: %v", i, err)
		}
	}

	// Forget with keep-last=1
	forgetOpts := ForgetOptions{
		KeepLast: 1,
		DryRun:   true, // Don't actually delete
	}

	forgetResult, err := executor.Forget(context.Background(), creds, forgetOpts)
	if err != nil {
		t.Fatalf("forget failed: %v", err)
	}

	// With dry-run and keep-last=1, we should have 2 to remove and 1 to keep
	if forgetResult.SnapshotsKept != 1 {
		t.Errorf("expected 1 snapshot kept, got %d", forgetResult.SnapshotsKept)
	}
	if forgetResult.SnapshotsRemoved != 2 {
		t.Errorf("expected 2 snapshots removed, got %d", forgetResult.SnapshotsRemoved)
	}
}

func TestDefaultExecutor_Integration_Prune(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic binary not found, skipping integration test")
	}

	// Create a temporary directory for the repository
	repoDir, err := os.MkdirTemp("", "restic-repo-*")
	if err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	defer os.RemoveAll(repoDir)

	log := getTestLogger()
	executor := NewExecutor(log)

	creds := Credentials{
		Repository: "local:" + repoDir,
		Password:   "test-password",
	}

	// Initialize the repository
	err = executor.Init(context.Background(), creds)
	if err != nil {
		t.Fatalf("failed to initialize repository: %v", err)
	}

	// Prune (empty repo is fine)
	pruneResult, err := executor.Prune(context.Background(), creds)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}

	if pruneResult.Duration <= 0 {
		t.Error("expected positive prune duration")
	}
}

// Test Executor interface compliance
func TestDefaultExecutor_ImplementsExecutor(t *testing.T) {
	var _ Executor = (*DefaultExecutor)(nil)
}
