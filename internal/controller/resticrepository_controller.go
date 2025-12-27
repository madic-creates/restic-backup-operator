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
	"fmt"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1alpha1 "github.com/madic-creates/restic-backup-operator/api/v1alpha1"
	"github.com/madic-creates/restic-backup-operator/internal/conditions"
	"github.com/madic-creates/restic-backup-operator/internal/restic"
)

const (
	defaultRequeueInterval = 1 * time.Hour
	errorRequeueInterval   = 30 * time.Second
	// DefaultStaleLockThreshold defines the default duration after which a lock is considered stale
	DefaultStaleLockThreshold = 30 * time.Minute
)

// lockAgeRegex matches the lock age in restic error messages like "(12h36m32.091009819s ago)"
var lockAgeRegex = regexp.MustCompile(`\((\d+h)?(\d+m)?[\d.]+s ago\)`)

// ResticRepositoryReconciler reconciles a ResticRepository object
type ResticRepositoryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	// Executor is optional - if nil, a default executor will be created
	Executor restic.Executor
	// StaleLockThreshold defines how old a lock must be to be considered stale.
	// If not set, DefaultStaleLockThreshold is used.
	StaleLockThreshold time.Duration
}

// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=resticrepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=resticrepositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop.
func (r *ResticRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling ResticRepository")

	// Fetch the ResticRepository instance
	repository := &backupv1alpha1.ResticRepository{}
	if err := r.Get(ctx, req.NamespacedName, repository); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ResticRepository resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ResticRepository")
		return ctrl.Result{}, err
	}

	// Get credentials from secret
	creds, err := r.getCredentials(ctx, repository)
	if err != nil {
		log.Error(err, "Failed to get credentials")
		r.setCondition(repository, conditions.NotReadyCondition("CredentialsNotFound", err.Error()))
		r.Recorder.Event(repository, corev1.EventTypeWarning, "CredentialsNotFound", err.Error())
		if err := r.Status().Update(ctx, repository); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: errorRequeueInterval}, nil
	}

	// Get restic executor (use injected one or create default)
	executor := r.Executor
	if executor == nil {
		executor = restic.NewExecutor(log)
	}

	// Check if repository exists and is accessible
	checkResult, err := executor.Check(ctx, creds)
	if err != nil {
		errStr := err.Error()

		// Check if repository is locked
		if strings.Contains(errStr, "repository is already locked") {
			// Only remove locks that are stale (older than threshold)
			lockAge := parseLockAge(errStr)
			threshold := r.getStaleLockThreshold()
			if lockAge >= threshold {
				log.Info("Repository has stale lock, attempting to remove", "lockAge", lockAge, "threshold", threshold)
				if unlockErr := executor.Unlock(ctx, creds); unlockErr != nil {
					log.Error(unlockErr, "Failed to unlock repository")
					r.setCondition(repository, conditions.NotReadyCondition("UnlockFailed", unlockErr.Error()))
					r.Recorder.Event(repository, corev1.EventTypeWarning, "UnlockFailed", unlockErr.Error())
					if updateErr := r.Status().Update(ctx, repository); updateErr != nil {
						return ctrl.Result{}, updateErr
					}
					return ctrl.Result{RequeueAfter: errorRequeueInterval}, nil
				}
				r.Recorder.Event(repository, corev1.EventTypeNormal, "RepositoryUnlocked", fmt.Sprintf("Stale lock (age: %s) was removed from repository", lockAge))
				log.Info("Repository unlocked successfully, retrying check")

				// Retry check after unlock
				checkResult, err = executor.Check(ctx, creds)
				if err == nil && checkResult != nil && checkResult.Success {
					log.Info("Repository check passed after unlock")
				}
			} else {
				// Lock is fresh - another operation might be in progress
				log.Info("Repository is locked by active operation, will retry later", "lockAge", lockAge, "threshold", threshold)
				r.setCondition(repository, conditions.NotReadyCondition("RepositoryLocked", fmt.Sprintf("Repository is locked by another operation (lock age: %s, threshold: %s)", lockAge, threshold)))
				r.Recorder.Event(repository, corev1.EventTypeWarning, "RepositoryLocked", fmt.Sprintf("Repository is locked by another operation, lock age: %s (threshold: %s)", lockAge, threshold))
				if updateErr := r.Status().Update(ctx, repository); updateErr != nil {
					return ctrl.Result{}, updateErr
				}
				return ctrl.Result{RequeueAfter: errorRequeueInterval}, nil
			}
		}

		// If still failing (not a lock issue, or lock removal didn't help), try to initialize
		if err != nil {
			log.Info("Repository check failed, attempting initialization", "error", err.Error())
			if initErr := executor.Init(ctx, creds); initErr != nil {
				log.Error(initErr, "Failed to initialize repository")
				r.setCondition(repository, conditions.NotReadyCondition("InitializationFailed", initErr.Error()))
				r.Recorder.Event(repository, corev1.EventTypeWarning, "InitializationFailed", initErr.Error())
				if updateErr := r.Status().Update(ctx, repository); updateErr != nil {
					return ctrl.Result{}, updateErr
				}
				return ctrl.Result{RequeueAfter: errorRequeueInterval}, nil
			}
			r.Recorder.Event(repository, corev1.EventTypeNormal, "RepositoryInitialized", "Repository was successfully initialized")
			log.Info("Repository initialized successfully")
		}
	} else if checkResult != nil && checkResult.Success {
		log.Info("Repository check passed")
	}

	// Repository is accessible - set Ready condition immediately
	// This ensures the repository is marked as ready even if stats retrieval is slow
	r.setCondition(repository, conditions.ReadyCondition("RepositoryAccessible", "Repository is initialized and accessible"))
	repository.Status.ObservedGeneration = repository.Generation

	if err := r.Status().Update(ctx, repository); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Get repository statistics (non-blocking for Ready status)
	// Stats can be slow for large repositories, so we run it after marking Ready
	stats, err := executor.Stats(ctx, creds, restic.StatsOptions{Mode: "restore-size"})
	if err != nil {
		log.Error(err, "Failed to get repository stats")
		// Don't fail the reconciliation just because stats failed
	} else {
		repository.Status.Statistics = &backupv1alpha1.RepositoryStatistics{
			TotalSize:      formatBytes(stats.TotalSize),
			TotalFileCount: int64(stats.TotalFileCount),
			SnapshotCount:  int32(stats.SnapshotCount),
		}
		// Update status with statistics
		if err := r.Status().Update(ctx, repository); err != nil {
			log.Error(err, "Failed to update status with statistics")
			return ctrl.Result{}, err
		}
	}

	r.Recorder.Event(repository, corev1.EventTypeNormal, "ReconcileSuccess", "Repository reconciled successfully")

	return ctrl.Result{RequeueAfter: defaultRequeueInterval}, nil
}

func (r *ResticRepositoryReconciler) getCredentials(ctx context.Context, repository *backupv1alpha1.ResticRepository) (restic.Credentials, error) {
	secret := &corev1.Secret{}
	secretName := types.NamespacedName{
		Name:      repository.Spec.CredentialsSecretRef.Name,
		Namespace: repository.Namespace,
	}

	if err := r.Get(ctx, secretName, secret); err != nil {
		return restic.Credentials{}, fmt.Errorf("failed to get credentials secret: %w", err)
	}

	password, ok := secret.Data["RESTIC_PASSWORD"]
	if !ok {
		return restic.Credentials{}, fmt.Errorf("RESTIC_PASSWORD not found in secret")
	}

	creds := restic.Credentials{
		Repository: repository.Spec.RepositoryURL,
		Password:   string(password),
	}

	// Optional AWS credentials
	if awsKeyID, ok := secret.Data["AWS_ACCESS_KEY_ID"]; ok {
		creds.AWSAccessKeyID = string(awsKeyID)
	}
	if awsSecret, ok := secret.Data["AWS_SECRET_ACCESS_KEY"]; ok {
		creds.AWSSecretAccessKey = string(awsSecret)
	}

	return creds, nil
}

func (r *ResticRepositoryReconciler) setCondition(repository *backupv1alpha1.ResticRepository, condition metav1.Condition) {
	conditions.SetCondition(&repository.Status.Conditions, condition)
}

// getStaleLockThreshold returns the configured stale lock threshold or the default.
func (r *ResticRepositoryReconciler) getStaleLockThreshold() time.Duration {
	if r.StaleLockThreshold > 0 {
		return r.StaleLockThreshold
	}
	return DefaultStaleLockThreshold
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResticRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.ResticRepository{}).
		Complete(r)
}

// formatBytes formats bytes as a human-readable string.
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// parseLockAge extracts the lock age from a restic error message.
// Example: "lock was created at 2025-12-26 21:32:34 (12h36m32.091009819s ago)"
// Returns 0 if the age cannot be parsed.
func parseLockAge(errMsg string) time.Duration {
	match := lockAgeRegex.FindString(errMsg)
	if match == "" {
		return 0
	}

	// Remove parentheses and " ago" suffix: "(12h36m32.091009819s ago)" -> "12h36m32.091009819s"
	durationStr := strings.TrimSuffix(strings.TrimPrefix(match, "("), " ago)")

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0
	}

	return duration
}
