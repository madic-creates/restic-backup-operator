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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

// Executor wraps restic CLI operations.
type Executor interface {
	// Init initializes a new repository.
	Init(ctx context.Context, creds Credentials) error

	// Unlock removes stale locks from the repository.
	Unlock(ctx context.Context, creds Credentials) error

	// Check verifies the repository integrity.
	Check(ctx context.Context, creds Credentials) (*CheckResult, error)

	// Stats returns repository statistics.
	Stats(ctx context.Context, creds Credentials, opts StatsOptions) (*RepoStats, error)

	// Snapshots lists all snapshots.
	Snapshots(ctx context.Context, creds Credentials) ([]Snapshot, error)

	// Backup creates a new backup.
	Backup(ctx context.Context, creds Credentials, opts BackupOptions) (*BackupResult, error)

	// Restore restores data from a snapshot.
	Restore(ctx context.Context, creds Credentials, opts RestoreOptions) (*RestoreResult, error)

	// Forget removes snapshots according to the retention policy.
	Forget(ctx context.Context, creds Credentials, opts ForgetOptions) (*ForgetResult, error)

	// Prune removes unused data from the repository.
	Prune(ctx context.Context, creds Credentials) (*PruneResult, error)
}

// DefaultExecutor implements Executor using the restic binary.
type DefaultExecutor struct {
	binary string
	log    logr.Logger
}

// NewExecutor creates a new restic executor.
func NewExecutor(log logr.Logger) *DefaultExecutor {
	return &DefaultExecutor{
		binary: "restic",
		log:    log,
	}
}

// NewExecutorWithBinary creates a new restic executor with a custom binary path.
func NewExecutorWithBinary(binary string, log logr.Logger) *DefaultExecutor {
	if binary == "" {
		binary = "restic"
	}
	return &DefaultExecutor{
		binary: binary,
		log:    log,
	}
}

func (e *DefaultExecutor) buildEnv(creds Credentials) []string {
	env := []string{
		fmt.Sprintf("RESTIC_REPOSITORY=%s", creds.Repository),
		fmt.Sprintf("RESTIC_PASSWORD=%s", creds.Password),
	}

	if creds.AWSAccessKeyID != "" {
		env = append(env, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", creds.AWSAccessKeyID))
	}
	if creds.AWSSecretAccessKey != "" {
		env = append(env, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", creds.AWSSecretAccessKey))
	}
	if creds.CacheDir != "" {
		env = append(env, fmt.Sprintf("RESTIC_CACHE_DIR=%s", creds.CacheDir))
	}

	return env
}

func (e *DefaultExecutor) run(ctx context.Context, creds Credentials, args []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, e.binary, args...)
	cmd.Env = e.buildEnv(creds)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	e.log.V(1).Info("executing restic command", "args", strings.Join(args, " "))

	err := cmd.Run()
	if err != nil {
		e.log.Error(err, "restic command failed", "stderr", stderr.String())
	}

	return stdout.Bytes(), stderr.Bytes(), err
}

// Init initializes a new repository.
func (e *DefaultExecutor) Init(ctx context.Context, creds Credentials) error {
	args := NewCommand("init").Build()
	_, stderr, err := e.run(ctx, creds, args)
	if err != nil {
		stderrStr := string(stderr)
		// Check if repository already exists (check both error and stderr)
		if strings.Contains(stderrStr, "already exists") ||
			strings.Contains(stderrStr, "repository master key and config already initialized") ||
			strings.Contains(stderrStr, "config file already exists") {
			return nil
		}
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	return nil
}

// Unlock removes stale locks from the repository.
func (e *DefaultExecutor) Unlock(ctx context.Context, creds Credentials) error {
	args := NewCommand("unlock").Build()
	_, _, err := e.run(ctx, creds, args)
	if err != nil {
		return fmt.Errorf("failed to unlock repository: %w", err)
	}
	return nil
}

// Check verifies the repository integrity.
func (e *DefaultExecutor) Check(ctx context.Context, creds Credentials) (*CheckResult, error) {
	start := time.Now()
	args := NewCommand("check").Build()
	_, stderr, err := e.run(ctx, creds, args)

	stderrStr := string(stderr)
	result := &CheckResult{
		Success:  err == nil,
		Message:  stderrStr,
		Duration: time.Since(start),
	}

	if err != nil {
		// Include stderr in error message for better diagnostics
		return result, fmt.Errorf("repository check failed: %w: %s", err, stderrStr)
	}

	return result, nil
}

// Stats returns repository statistics.
func (e *DefaultExecutor) Stats(ctx context.Context, creds Credentials, opts StatsOptions) (*RepoStats, error) {
	cmd := NewCommand("stats").WithJSON()
	if opts.Mode != "" {
		cmd.WithMode(opts.Mode)
	}
	args := cmd.Build()

	stdout, _, err := e.run(ctx, creds, args)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository stats: %w", err)
	}

	var stats struct {
		TotalSize      uint64 `json:"total_size"`
		TotalFileCount uint64 `json:"total_file_count"`
	}
	if err := json.Unmarshal(stdout, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse stats output: %w", err)
	}

	// Get snapshot count
	snapshots, err := e.Snapshots(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot count: %w", err)
	}

	return &RepoStats{
		TotalSize:      stats.TotalSize,
		TotalFileCount: stats.TotalFileCount,
		SnapshotCount:  len(snapshots),
	}, nil
}

// Snapshots lists all snapshots.
func (e *DefaultExecutor) Snapshots(ctx context.Context, creds Credentials) ([]Snapshot, error) {
	args := NewCommand("snapshots").WithJSON().Build()

	stdout, _, err := e.run(ctx, creds, args)
	if err != nil {
		// If repository is empty or not found, return empty list
		if strings.Contains(err.Error(), "no snapshots") {
			return []Snapshot{}, nil
		}
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	var snapshots []Snapshot
	if err := json.Unmarshal(stdout, &snapshots); err != nil {
		// Handle empty repository case
		if len(stdout) == 0 || string(stdout) == "null" {
			return []Snapshot{}, nil
		}
		return nil, fmt.Errorf("failed to parse snapshots output: %w", err)
	}

	return snapshots, nil
}

// Backup creates a new backup.
func (e *DefaultExecutor) Backup(ctx context.Context, creds Credentials, opts BackupOptions) (*BackupResult, error) {
	start := time.Now()

	cmd := NewCommand("backup").
		WithJSON().
		WithHost(opts.Hostname).
		WithTags(opts.Tags).
		WithExcludes(opts.Excludes).
		WithArgs(opts.ExtraArgs).
		WithPaths(opts.Paths)

	args := cmd.Build()

	stdout, _, err := e.run(ctx, creds, args)
	if err != nil {
		return nil, fmt.Errorf("backup failed: %w", err)
	}

	// Parse the JSON output (last line contains summary)
	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("no output from backup command")
	}

	// Parse the summary message
	var summary struct {
		MessageType         string `json:"message_type"`
		SnapshotID          string `json:"snapshot_id"`
		FilesNew            int64  `json:"files_new"`
		FilesChanged        int64  `json:"files_changed"`
		FilesUnmodified     int64  `json:"files_unmodified"`
		DirsNew             int64  `json:"dirs_new"`
		DirsChanged         int64  `json:"dirs_changed"`
		DirsUnmodified      int64  `json:"dirs_unmodified"`
		DataAdded           uint64 `json:"data_added"`
		TotalFilesProcessed int64  `json:"total_files_processed"`
		TotalBytesProcessed uint64 `json:"total_bytes_processed"`
	}

	// Find the summary line
	for i := len(lines) - 1; i >= 0; i-- {
		if err := json.Unmarshal([]byte(lines[i]), &summary); err == nil {
			if summary.MessageType == "summary" {
				break
			}
		}
	}

	return &BackupResult{
		SnapshotID:      summary.SnapshotID,
		FilesNew:        summary.FilesNew,
		FilesChanged:    summary.FilesChanged,
		FilesUnmodified: summary.FilesUnmodified,
		DirsNew:         summary.DirsNew,
		DirsChanged:     summary.DirsChanged,
		DirsUnmodified:  summary.DirsUnmodified,
		DataAdded:       summary.DataAdded,
		TotalFiles:      summary.TotalFilesProcessed,
		TotalBytes:      summary.TotalBytesProcessed,
		Duration:        time.Since(start),
	}, nil
}

// Restore restores data from a snapshot.
func (e *DefaultExecutor) Restore(ctx context.Context, creds Credentials, opts RestoreOptions) (*RestoreResult, error) {
	start := time.Now()

	cmd := NewCommand("restore").
		WithTarget(opts.Target).
		WithIncludes(opts.Include).
		WithExcludes(opts.Exclude).
		WithArg(opts.SnapshotID)

	if opts.Verify {
		cmd.WithArg("--verify")
	}

	args := cmd.Build()

	_, _, err := e.run(ctx, creds, args)
	if err != nil {
		return nil, fmt.Errorf("restore failed: %w", err)
	}

	return &RestoreResult{
		Duration: time.Since(start),
	}, nil
}

// Forget removes snapshots according to the retention policy.
func (e *DefaultExecutor) Forget(ctx context.Context, creds Credentials, opts ForgetOptions) (*ForgetResult, error) {
	cmd := NewCommand("forget").
		WithJSON().
		WithHost(opts.Hostname).
		WithTags(opts.Tags).
		WithKeepLast(opts.KeepLast).
		WithKeepHourly(opts.KeepHourly).
		WithKeepDaily(opts.KeepDaily).
		WithKeepWeekly(opts.KeepWeekly).
		WithKeepMonthly(opts.KeepMonthly).
		WithKeepYearly(opts.KeepYearly)

	if len(opts.GroupBy) > 0 {
		cmd.WithGroupBy(strings.Join(opts.GroupBy, ","))
	}
	if opts.Prune {
		cmd.WithPrune()
	}
	if opts.DryRun {
		cmd.WithDryRun()
	}

	args := cmd.Build()

	stdout, _, err := e.run(ctx, creds, args)
	if err != nil {
		return nil, fmt.Errorf("forget failed: %w", err)
	}

	// Parse output to count removed/kept snapshots
	var forgetOutput []struct {
		Tags   []string `json:"tags"`
		Host   string   `json:"host"`
		Remove []struct {
			ID string `json:"id"`
		} `json:"remove"`
		Keep []struct {
			ID string `json:"id"`
		} `json:"keep"`
	}

	if err := json.Unmarshal(stdout, &forgetOutput); err != nil {
		// If parsing fails, return empty result
		return &ForgetResult{}, nil
	}

	result := &ForgetResult{}
	for _, group := range forgetOutput {
		result.SnapshotsRemoved += len(group.Remove)
		result.SnapshotsKept += len(group.Keep)
	}

	return result, nil
}

// Prune removes unused data from the repository.
func (e *DefaultExecutor) Prune(ctx context.Context, creds Credentials) (*PruneResult, error) {
	start := time.Now()
	args := NewCommand("prune").Build()

	_, _, err := e.run(ctx, creds, args)
	if err != nil {
		return nil, fmt.Errorf("prune failed: %w", err)
	}

	return &PruneResult{
		Duration: time.Since(start),
	}, nil
}
