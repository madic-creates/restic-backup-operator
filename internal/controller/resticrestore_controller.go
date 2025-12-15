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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1alpha1 "github.com/madic-creates/restic-backup-operator/api/v1alpha1"
	"github.com/madic-creates/restic-backup-operator/internal/conditions"
)

const (
	resticRestoreFinalizer = "backup.resticbackup.io/resticrestore-finalizer"
)

// ResticRestoreReconciler reconciles a ResticRestore object
type ResticRestoreReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=resticrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=resticrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=resticrestores/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop.
func (r *ResticRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling ResticRestore")

	// Fetch the ResticRestore instance
	restore := &backupv1alpha1.ResticRestore{}
	if err := r.Get(ctx, req.NamespacedName, restore); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ResticRestore resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ResticRestore")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !restore.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, restore)
	}

	// Add finalizer if missing
	if !controllerutil.ContainsFinalizer(restore, resticRestoreFinalizer) {
		controllerutil.AddFinalizer(restore, resticRestoreFinalizer)
		if err := r.Update(ctx, restore); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Initialize phase if not set
	if restore.Status.Phase == "" {
		restore.Status.Phase = backupv1alpha1.RestorePhasePending
		if err := r.Status().Update(ctx, restore); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Handle restore based on phase
	switch restore.Status.Phase {
	case backupv1alpha1.RestorePhasePending:
		return r.handlePending(ctx, restore)
	case backupv1alpha1.RestorePhaseInProgress:
		return r.handleInProgress(ctx, restore)
	case backupv1alpha1.RestorePhaseCompleted, backupv1alpha1.RestorePhaseFailed:
		// Nothing to do for completed/failed restores
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ResticRestoreReconciler) handleDeletion(ctx context.Context, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(restore, resticRestoreFinalizer) {
		log.Info("Performing finalizer cleanup for ResticRestore")

		controllerutil.RemoveFinalizer(restore, resticRestoreFinalizer)
		if err := r.Update(ctx, restore); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ResticRestoreReconciler) handlePending(ctx context.Context, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get the backup reference to find repository
	backup, err := r.getBackup(ctx, restore)
	if err != nil {
		log.Error(err, "Failed to get backup")
		r.setCondition(restore, conditions.NotReadyCondition("BackupNotFound", err.Error()))
		r.Recorder.Event(restore, corev1.EventTypeWarning, "BackupNotFound", err.Error())
		restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
		if updateErr := r.Status().Update(ctx, restore); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	// Get the repository
	repository, err := r.getRepository(ctx, backup)
	if err != nil {
		log.Error(err, "Failed to get repository")
		r.setCondition(restore, conditions.NotReadyCondition("RepositoryNotFound", err.Error()))
		restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
		if updateErr := r.Status().Update(ctx, restore); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	// Determine snapshot ID
	snapshotID := restore.Spec.SnapshotID
	if snapshotID == "" && restore.Spec.SnapshotSelector != nil {
		// For now, just use "latest" - full implementation would query restic
		snapshotID = "latest"
	}
	if snapshotID == "" {
		snapshotID = "latest"
	}

	// Create restore job
	job := r.buildRestoreJob(restore, backup, repository, snapshotID)

	// Set owner reference
	if err := controllerutil.SetControllerReference(restore, job, r.Scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Create the job
	if err := r.Create(ctx, job); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create restore job")
			r.setCondition(restore, conditions.NotReadyCondition("JobCreationFailed", err.Error()))
			restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
			if updateErr := r.Status().Update(ctx, restore); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, nil
		}
	}

	// Update status
	now := metav1.NewTime(time.Now())
	restore.Status.Phase = backupv1alpha1.RestorePhaseInProgress
	restore.Status.StartTime = &now
	restore.Status.RestoredSnapshot = snapshotID
	restore.Status.JobRef = &backupv1alpha1.ObjectReference{
		Name:      job.Name,
		Namespace: job.Namespace,
	}
	r.setCondition(restore, conditions.NewCondition("Ready", metav1.ConditionUnknown, "RestoreInProgress", "Restore job is running"))

	if err := r.Status().Update(ctx, restore); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(restore, corev1.EventTypeNormal, "RestoreStarted", fmt.Sprintf("Restore job %s created", job.Name))

	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *ResticRestoreReconciler) handleInProgress(ctx context.Context, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if restore.Status.JobRef == nil {
		restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
		r.setCondition(restore, conditions.NotReadyCondition("JobNotFound", "No job reference in status"))
		if err := r.Status().Update(ctx, restore); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get the job
	job := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      restore.Status.JobRef.Name,
		Namespace: restore.Status.JobRef.Namespace,
	}, job); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Restore job not found, marking as failed")
			restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
			r.setCondition(restore, conditions.NotReadyCondition("JobNotFound", "Restore job was not found"))
			if updateErr := r.Status().Update(ctx, restore); updateErr != nil {
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check job status
	if job.Status.Succeeded > 0 {
		now := metav1.NewTime(time.Now())
		restore.Status.Phase = backupv1alpha1.RestorePhaseCompleted
		restore.Status.CompletionTime = &now
		r.setCondition(restore, conditions.ReadyCondition("RestoreCompleted", "Restore completed successfully"))
		r.Recorder.Event(restore, corev1.EventTypeNormal, "RestoreCompleted", "Restore completed successfully")
		if err := r.Status().Update(ctx, restore); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		now := metav1.NewTime(time.Now())
		restore.Status.Phase = backupv1alpha1.RestorePhaseFailed
		restore.Status.CompletionTime = &now
		r.setCondition(restore, conditions.NotReadyCondition("RestoreFailed", "Restore job failed"))
		r.Recorder.Event(restore, corev1.EventTypeWarning, "RestoreFailed", "Restore job failed")
		if err := r.Status().Update(ctx, restore); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Job still running
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *ResticRestoreReconciler) getBackup(ctx context.Context, restore *backupv1alpha1.ResticRestore) (*backupv1alpha1.ResticBackup, error) {
	backup := &backupv1alpha1.ResticBackup{}
	ns := restore.Spec.BackupRef.Namespace
	if ns == "" {
		ns = restore.Namespace
	}

	name := types.NamespacedName{
		Name:      restore.Spec.BackupRef.Name,
		Namespace: ns,
	}

	if err := r.Get(ctx, name, backup); err != nil {
		return nil, fmt.Errorf("failed to get backup: %w", err)
	}

	return backup, nil
}

func (r *ResticRestoreReconciler) getRepository(ctx context.Context, backup *backupv1alpha1.ResticBackup) (*backupv1alpha1.ResticRepository, error) {
	repository := &backupv1alpha1.ResticRepository{}
	ns := backup.Spec.RepositoryRef.Namespace
	if ns == "" {
		ns = backup.Namespace
	}

	name := types.NamespacedName{
		Name:      backup.Spec.RepositoryRef.Name,
		Namespace: ns,
	}

	if err := r.Get(ctx, name, repository); err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return repository, nil
}

func (r *ResticRestoreReconciler) buildRestoreJob(restore *backupv1alpha1.ResticRestore, backup *backupv1alpha1.ResticBackup, repository *backupv1alpha1.ResticRepository, snapshotID string) *batchv1.Job {
	jobName := fmt.Sprintf("resticrestore-%s", restore.Name)

	// Build restic image
	resticImage := "ghcr.io/restic/restic:0.18.0"
	if backup.Spec.Restic != nil && backup.Spec.Restic.Image != "" {
		resticImage = backup.Spec.Restic.Image
	}

	// Build restore command
	restoreCmd := []string{
		"restic", "restore",
		snapshotID,
		"--target", "/restore",
	}

	// Add include paths
	for _, path := range restore.Spec.IncludePaths {
		restoreCmd = append(restoreCmd, "--include", path)
	}

	// Add exclude paths
	for _, path := range restore.Spec.ExcludePaths {
		restoreCmd = append(restoreCmd, "--exclude", path)
	}

	// Add verify flag
	if restore.Spec.Options != nil && restore.Spec.Options.Verify {
		restoreCmd = append(restoreCmd, "--verify")
	}

	// Build environment variables
	envVars := []corev1.EnvVar{
		{
			Name:  "RESTIC_REPOSITORY",
			Value: repository.Spec.RepositoryURL,
		},
		{
			Name: "RESTIC_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: repository.Spec.CredentialsSecretRef.Name,
					},
					Key: "RESTIC_PASSWORD",
				},
			},
		},
		{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: repository.Spec.CredentialsSecretRef.Name,
					},
					Key:      "AWS_ACCESS_KEY_ID",
					Optional: boolPtr(true),
				},
			},
		},
		{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: repository.Spec.CredentialsSecretRef.Name,
					},
					Key:      "AWS_SECRET_ACCESS_KEY",
					Optional: boolPtr(true),
				},
			},
		},
	}

	// Determine target PVC
	var targetPVC string
	if restore.Spec.Target.PVC != nil {
		targetPVC = restore.Spec.Target.PVC.ClaimName
	} else if restore.Spec.Target.NewPVC != nil {
		targetPVC = restore.Spec.Target.NewPVC.Name
	}

	var backoffLimit int32 = 0
	var activeDeadline int64 = 3600

	if restore.Spec.JobConfig != nil {
		if restore.Spec.JobConfig.BackoffLimit != nil {
			backoffLimit = *restore.Spec.JobConfig.BackoffLimit
		}
		if restore.Spec.JobConfig.ActiveDeadlineSeconds != nil {
			activeDeadline = *restore.Spec.JobConfig.ActiveDeadlineSeconds
		}
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: restore.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":         "restic-backup-operator",
				"app.kubernetes.io/component":    "restore",
				"app.kubernetes.io/managed-by":   "restic-backup-operator",
				"backup.resticbackup.io/restore": restore.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &activeDeadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":         "restic-backup-operator",
						"app.kubernetes.io/component":    "restore",
						"backup.resticbackup.io/restore": restore.Name,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
						RunAsUser:    int64Ptr(65532),
						FSGroup:      int64Ptr(65532),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "restic",
							Image:           resticImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         restoreCmd,
							Env:             envVars,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								ReadOnlyRootFilesystem:   boolPtr(false),
								RunAsNonRoot:             boolPtr(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "restore-target",
									MountPath: "/restore",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "restore-target",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: targetPVC,
								},
							},
						},
					},
				},
			},
		},
	}

	return job
}

func (r *ResticRestoreReconciler) setCondition(restore *backupv1alpha1.ResticRestore, condition metav1.Condition) {
	conditions.SetCondition(&restore.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResticRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.ResticRestore{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
