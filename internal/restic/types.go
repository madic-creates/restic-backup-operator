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

import "time"

// Credentials contains the credentials needed to access a restic repository.
type Credentials struct {
	// Repository URL
	Repository string
	// Password for the repository
	Password string
	// AWS access key ID (for S3 repositories)
	AWSAccessKeyID string
	// AWS secret access key (for S3 repositories)
	AWSSecretAccessKey string
	// Cache directory (optional)
	CacheDir string
}

// Snapshot represents a restic snapshot.
type Snapshot struct {
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
	Time     time.Time `json:"time"`
	Hostname string    `json:"hostname"`
	Username string    `json:"username"`
	Tree     string    `json:"tree"`
	Paths    []string  `json:"paths"`
	Tags     []string  `json:"tags"`
	Parent   string    `json:"parent,omitempty"`
}

// RepoStats contains repository statistics.
type RepoStats struct {
	TotalSize      uint64 `json:"total_size"`
	TotalFileCount uint64 `json:"total_file_count"`
	SnapshotCount  int    `json:"snapshot_count"`
}

// BackupResult contains the result of a backup operation.
type BackupResult struct {
	SnapshotID      string
	FilesNew        int64
	FilesChanged    int64
	FilesUnmodified int64
	DirsNew         int64
	DirsChanged     int64
	DirsUnmodified  int64
	DataAdded       uint64
	TotalFiles      int64
	TotalBytes      uint64
	Duration        time.Duration
}

// RestoreResult contains the result of a restore operation.
type RestoreResult struct {
	RestoredFiles int64
	RestoredBytes uint64
	Duration      time.Duration
}

// ForgetResult contains the result of a forget operation.
type ForgetResult struct {
	SnapshotsRemoved int
	SnapshotsKept    int
}

// PruneResult contains the result of a prune operation.
type PruneResult struct {
	PacksDeleted int
	BytesFreed   uint64
	Duration     time.Duration
}

// CheckResult contains the result of a check operation.
type CheckResult struct {
	Success  bool
	Message  string
	Duration time.Duration
}

// BackupOptions contains options for a backup operation.
type BackupOptions struct {
	// Source paths to backup
	Paths []string
	// Exclude patterns
	Excludes []string
	// Hostname for the snapshot
	Hostname string
	// Tags for the snapshot
	Tags []string
	// Extra arguments to pass to restic
	ExtraArgs []string
}

// RestoreOptions contains options for a restore operation.
type RestoreOptions struct {
	// Snapshot ID to restore
	SnapshotID string
	// Target directory
	Target string
	// Include paths (relative to snapshot)
	Include []string
	// Exclude paths
	Exclude []string
	// Overwrite existing files
	Overwrite bool
	// Verify restored files
	Verify bool
}

// ForgetOptions contains options for a forget operation.
type ForgetOptions struct {
	// Keep policies
	KeepLast    int
	KeepHourly  int
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
	KeepYearly  int
	// Filter by tags
	Tags []string
	// Filter by hostname
	Hostname string
	// Group by
	GroupBy []string
	// Prune after forget
	Prune bool
	// Dry run
	DryRun bool
}

// StatsOptions contains options for a stats operation.
type StatsOptions struct {
	// Mode: raw-data, files-by-contents, blobs-per-file, restore-size
	Mode string
}
