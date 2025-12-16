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
	"encoding/json"
	"testing"
	"time"
)

func TestCredentials(t *testing.T) {
	creds := Credentials{
		Repository:         "s3:s3.amazonaws.com/bucket",
		Password:           "secret",
		AWSAccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		AWSSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		CacheDir:           "/tmp/cache",
	}

	if creds.Repository != "s3:s3.amazonaws.com/bucket" {
		t.Errorf("unexpected Repository: %s", creds.Repository)
	}
	if creds.Password != "secret" {
		t.Errorf("unexpected Password: %s", creds.Password)
	}
	if creds.AWSAccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("unexpected AWSAccessKeyID: %s", creds.AWSAccessKeyID)
	}
	if creds.AWSSecretAccessKey != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" {
		t.Errorf("unexpected AWSSecretAccessKey: %s", creds.AWSSecretAccessKey)
	}
	if creds.CacheDir != "/tmp/cache" {
		t.Errorf("unexpected CacheDir: %s", creds.CacheDir)
	}
}

func TestSnapshot_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": "abc12345def67890abc12345def67890abc12345def67890abc12345def67890",
		"short_id": "abc12345",
		"time": "2024-01-15T10:30:00Z",
		"hostname": "backup-host",
		"username": "root",
		"tree": "tree12345",
		"paths": ["/data", "/config"],
		"tags": ["daily", "app1"],
		"parent": "parent12345"
	}`

	var snapshot Snapshot
	if err := json.Unmarshal([]byte(jsonData), &snapshot); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if snapshot.ID != "abc12345def67890abc12345def67890abc12345def67890abc12345def67890" {
		t.Errorf("unexpected ID: %s", snapshot.ID)
	}
	if snapshot.ShortID != "abc12345" {
		t.Errorf("unexpected ShortID: %s", snapshot.ShortID)
	}
	if snapshot.Hostname != "backup-host" {
		t.Errorf("unexpected Hostname: %s", snapshot.Hostname)
	}
	if snapshot.Username != "root" {
		t.Errorf("unexpected Username: %s", snapshot.Username)
	}
	if snapshot.Tree != "tree12345" {
		t.Errorf("unexpected Tree: %s", snapshot.Tree)
	}
	if len(snapshot.Paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(snapshot.Paths))
	}
	if len(snapshot.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(snapshot.Tags))
	}
	if snapshot.Parent != "parent12345" {
		t.Errorf("unexpected Parent: %s", snapshot.Parent)
	}
}

func TestSnapshot_JSONUnmarshal_WithoutParent(t *testing.T) {
	jsonData := `{
		"id": "abc12345",
		"short_id": "abc1234",
		"time": "2024-01-15T10:30:00Z",
		"hostname": "backup-host",
		"username": "root",
		"tree": "tree12345",
		"paths": ["/data"],
		"tags": ["daily"]
	}`

	var snapshot Snapshot
	if err := json.Unmarshal([]byte(jsonData), &snapshot); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if snapshot.Parent != "" {
		t.Errorf("expected empty Parent, got: %s", snapshot.Parent)
	}
}

func TestRepoStats(t *testing.T) {
	stats := RepoStats{
		TotalSize:      1024 * 1024 * 100, // 100 MB
		TotalFileCount: 1000,
		SnapshotCount:  5,
	}

	if stats.TotalSize != 104857600 {
		t.Errorf("unexpected TotalSize: %d", stats.TotalSize)
	}
	if stats.TotalFileCount != 1000 {
		t.Errorf("unexpected TotalFileCount: %d", stats.TotalFileCount)
	}
	if stats.SnapshotCount != 5 {
		t.Errorf("unexpected SnapshotCount: %d", stats.SnapshotCount)
	}
}

func TestBackupResult(t *testing.T) {
	result := BackupResult{
		SnapshotID:      "abc12345",
		FilesNew:        100,
		FilesChanged:    50,
		FilesUnmodified: 850,
		DirsNew:         10,
		DirsChanged:     5,
		DirsUnmodified:  85,
		DataAdded:       1024 * 1024, // 1 MB
		TotalFiles:      1000,
		TotalBytes:      1024 * 1024 * 10, // 10 MB
		Duration:        5 * time.Second,
	}

	if result.SnapshotID != "abc12345" {
		t.Errorf("unexpected SnapshotID: %s", result.SnapshotID)
	}
	if result.FilesNew != 100 {
		t.Errorf("unexpected FilesNew: %d", result.FilesNew)
	}
	if result.FilesChanged != 50 {
		t.Errorf("unexpected FilesChanged: %d", result.FilesChanged)
	}
	if result.FilesUnmodified != 850 {
		t.Errorf("unexpected FilesUnmodified: %d", result.FilesUnmodified)
	}
	if result.DirsNew != 10 {
		t.Errorf("unexpected DirsNew: %d", result.DirsNew)
	}
	if result.DirsChanged != 5 {
		t.Errorf("unexpected DirsChanged: %d", result.DirsChanged)
	}
	if result.DirsUnmodified != 85 {
		t.Errorf("unexpected DirsUnmodified: %d", result.DirsUnmodified)
	}
	if result.DataAdded != 1048576 {
		t.Errorf("unexpected DataAdded: %d", result.DataAdded)
	}
	if result.TotalFiles != 1000 {
		t.Errorf("unexpected TotalFiles: %d", result.TotalFiles)
	}
	if result.TotalBytes != 10485760 {
		t.Errorf("unexpected TotalBytes: %d", result.TotalBytes)
	}
	if result.Duration != 5*time.Second {
		t.Errorf("unexpected Duration: %v", result.Duration)
	}
}

func TestRestoreResult(t *testing.T) {
	result := RestoreResult{
		RestoredFiles: 500,
		RestoredBytes: 1024 * 1024 * 50, // 50 MB
		Duration:      10 * time.Second,
	}

	if result.RestoredFiles != 500 {
		t.Errorf("unexpected RestoredFiles: %d", result.RestoredFiles)
	}
	if result.RestoredBytes != 52428800 {
		t.Errorf("unexpected RestoredBytes: %d", result.RestoredBytes)
	}
	if result.Duration != 10*time.Second {
		t.Errorf("unexpected Duration: %v", result.Duration)
	}
}

func TestForgetResult(t *testing.T) {
	result := ForgetResult{
		SnapshotsRemoved: 5,
		SnapshotsKept:    10,
	}

	if result.SnapshotsRemoved != 5 {
		t.Errorf("unexpected SnapshotsRemoved: %d", result.SnapshotsRemoved)
	}
	if result.SnapshotsKept != 10 {
		t.Errorf("unexpected SnapshotsKept: %d", result.SnapshotsKept)
	}
}

func TestPruneResult(t *testing.T) {
	result := PruneResult{
		PacksDeleted: 20,
		BytesFreed:   1024 * 1024 * 100, // 100 MB
		Duration:     30 * time.Second,
	}

	if result.PacksDeleted != 20 {
		t.Errorf("unexpected PacksDeleted: %d", result.PacksDeleted)
	}
	if result.BytesFreed != 104857600 {
		t.Errorf("unexpected BytesFreed: %d", result.BytesFreed)
	}
	if result.Duration != 30*time.Second {
		t.Errorf("unexpected Duration: %v", result.Duration)
	}
}

func TestCheckResult(t *testing.T) {
	result := CheckResult{
		Success:  true,
		Message:  "no errors found",
		Duration: 2 * time.Second,
	}

	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.Message != "no errors found" {
		t.Errorf("unexpected Message: %s", result.Message)
	}
	if result.Duration != 2*time.Second {
		t.Errorf("unexpected Duration: %v", result.Duration)
	}
}

func TestBackupOptions(t *testing.T) {
	opts := BackupOptions{
		Paths:     []string{"/data", "/config"},
		Excludes:  []string{"*.tmp", "*.log"},
		Hostname:  "backup-host",
		Tags:      []string{"daily", "app1"},
		ExtraArgs: []string{"--verbose", "--dry-run"},
	}

	if len(opts.Paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(opts.Paths))
	}
	if len(opts.Excludes) != 2 {
		t.Errorf("expected 2 excludes, got %d", len(opts.Excludes))
	}
	if opts.Hostname != "backup-host" {
		t.Errorf("unexpected Hostname: %s", opts.Hostname)
	}
	if len(opts.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(opts.Tags))
	}
	if len(opts.ExtraArgs) != 2 {
		t.Errorf("expected 2 extra args, got %d", len(opts.ExtraArgs))
	}
}

func TestRestoreOptions(t *testing.T) {
	opts := RestoreOptions{
		SnapshotID: "abc12345",
		Target:     "/restore",
		Include:    []string{"/data"},
		Exclude:    []string{"*.tmp"},
		Overwrite:  true,
		Verify:     true,
	}

	if opts.SnapshotID != "abc12345" {
		t.Errorf("unexpected SnapshotID: %s", opts.SnapshotID)
	}
	if opts.Target != "/restore" {
		t.Errorf("unexpected Target: %s", opts.Target)
	}
	if len(opts.Include) != 1 {
		t.Errorf("expected 1 include, got %d", len(opts.Include))
	}
	if len(opts.Exclude) != 1 {
		t.Errorf("expected 1 exclude, got %d", len(opts.Exclude))
	}
	if !opts.Overwrite {
		t.Error("expected Overwrite to be true")
	}
	if !opts.Verify {
		t.Error("expected Verify to be true")
	}
}

func TestForgetOptions(t *testing.T) {
	opts := ForgetOptions{
		KeepLast:    10,
		KeepHourly:  24,
		KeepDaily:   7,
		KeepWeekly:  4,
		KeepMonthly: 12,
		KeepYearly:  3,
		Tags:        []string{"daily"},
		Hostname:    "backup-host",
		GroupBy:     []string{"host", "tags"},
		Prune:       true,
		DryRun:      false,
	}

	if opts.KeepLast != 10 {
		t.Errorf("unexpected KeepLast: %d", opts.KeepLast)
	}
	if opts.KeepHourly != 24 {
		t.Errorf("unexpected KeepHourly: %d", opts.KeepHourly)
	}
	if opts.KeepDaily != 7 {
		t.Errorf("unexpected KeepDaily: %d", opts.KeepDaily)
	}
	if opts.KeepWeekly != 4 {
		t.Errorf("unexpected KeepWeekly: %d", opts.KeepWeekly)
	}
	if opts.KeepMonthly != 12 {
		t.Errorf("unexpected KeepMonthly: %d", opts.KeepMonthly)
	}
	if opts.KeepYearly != 3 {
		t.Errorf("unexpected KeepYearly: %d", opts.KeepYearly)
	}
	if len(opts.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(opts.Tags))
	}
	if opts.Hostname != "backup-host" {
		t.Errorf("unexpected Hostname: %s", opts.Hostname)
	}
	if len(opts.GroupBy) != 2 {
		t.Errorf("expected 2 group by fields, got %d", len(opts.GroupBy))
	}
	if !opts.Prune {
		t.Error("expected Prune to be true")
	}
	if opts.DryRun {
		t.Error("expected DryRun to be false")
	}
}

func TestStatsOptions(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{"raw-data mode", "raw-data"},
		{"files-by-contents mode", "files-by-contents"},
		{"blobs-per-file mode", "blobs-per-file"},
		{"restore-size mode", "restore-size"},
		{"empty mode", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := StatsOptions{Mode: tt.mode}
			if opts.Mode != tt.mode {
				t.Errorf("expected mode %s, got %s", tt.mode, opts.Mode)
			}
		})
	}
}

func TestSnapshotList_JSONUnmarshal(t *testing.T) {
	jsonData := `[
		{
			"id": "snap1",
			"short_id": "snap1",
			"time": "2024-01-15T10:30:00Z",
			"hostname": "host1",
			"username": "root",
			"tree": "tree1",
			"paths": ["/data"],
			"tags": ["daily"]
		},
		{
			"id": "snap2",
			"short_id": "snap2",
			"time": "2024-01-14T10:30:00Z",
			"hostname": "host2",
			"username": "admin",
			"tree": "tree2",
			"paths": ["/config"],
			"tags": ["weekly"]
		}
	]`

	var snapshots []Snapshot
	if err := json.Unmarshal([]byte(jsonData), &snapshots); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(snapshots) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snapshots))
	}

	if snapshots[0].ID != "snap1" {
		t.Errorf("unexpected first snapshot ID: %s", snapshots[0].ID)
	}
	if snapshots[1].ID != "snap2" {
		t.Errorf("unexpected second snapshot ID: %s", snapshots[1].ID)
	}
}
