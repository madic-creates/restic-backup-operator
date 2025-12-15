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

	backupv1alpha1 "github.com/madic-creates/restic-backup-operator/api/v1alpha1"
	"github.com/madic-creates/restic-backup-operator/internal/conditions"
	"github.com/robfig/cron/v3"
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
)

const (
	resticBackupFinalizer = "backup.resticbackup.io/resticbackup-finalizer"
)

// ResticBackupReconciler reconciles a ResticBackup object
type ResticBackupReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=resticbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=resticbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=resticbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop.
func (r *ResticBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling ResticBackup")

	// Fetch the ResticBackup instance
	backup := &backupv1alpha1.ResticBackup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ResticBackup resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ResticBackup")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !backup.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, backup)
	}

	// Add finalizer if missing
	if !controllerutil.ContainsFinalizer(backup, resticBackupFinalizer) {
		controllerutil.AddFinalizer(backup, resticBackupFinalizer)
		if err := r.Update(ctx, backup); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Validate and get referenced repository
	repository, err := r.getRepository(ctx, backup)
	if err != nil {
		log.Error(err, "Failed to get repository")
		r.setCondition(backup, conditions.NotReadyCondition("RepositoryNotFound", err.Error()))
		r.Recorder.Event(backup, corev1.EventTypeWarning, "RepositoryNotFound", err.Error())
		if updateErr := r.Status().Update(ctx, backup); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{RequeueAfter: errorRequeueInterval}, nil
	}

	// Check repository is ready
	if !conditions.IsConditionTrue(repository.Status.Conditions, "Ready") {
		log.Info("Repository not ready, requeuing")
		r.setCondition(backup, conditions.NotReadyCondition("RepositoryNotReady", "Referenced repository is not ready"))
		if err := r.Status().Update(ctx, backup); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Set RepositoryReady condition
	conditions.SetCondition(&backup.Status.Conditions, metav1.Condition{
		Type:    backupv1alpha1.ConditionRepositoryReady,
		Status:  metav1.ConditionTrue,
		Reason:  "RepositoryAccessible",
		Message: "Referenced repository is ready",
	})

	// Reconcile CronJob
	if err := r.reconcileCronJob(ctx, backup, repository); err != nil {
		log.Error(err, "Failed to reconcile CronJob")
		r.setCondition(backup, conditions.NotReadyCondition("CronJobFailed", err.Error()))
		r.Recorder.Event(backup, corev1.EventTypeWarning, "CronJobFailed", err.Error())
		if updateErr := r.Status().Update(ctx, backup); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{RequeueAfter: errorRequeueInterval}, nil
	}

	// Calculate next backup time
	nextBackup := r.calculateNextBackup(backup)
	if nextBackup != nil {
		backup.Status.NextBackup = nextBackup
	}

	// Set Ready condition
	r.setCondition(backup, conditions.ReadyCondition("BackupConfigured", "Backup CronJob is configured and running"))
	backup.Status.ObservedGeneration = backup.Generation

	if err := r.Status().Update(ctx, backup); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	r.Recorder.Event(backup, corev1.EventTypeNormal, "ReconcileSuccess", "Backup reconciled successfully")

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *ResticBackupReconciler) handleDeletion(ctx context.Context, backup *backupv1alpha1.ResticBackup) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(backup, resticBackupFinalizer) {
		log.Info("Performing finalizer cleanup for ResticBackup")

		// CronJob will be garbage collected due to owner reference

		controllerutil.RemoveFinalizer(backup, resticBackupFinalizer)
		if err := r.Update(ctx, backup); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ResticBackupReconciler) getRepository(ctx context.Context, backup *backupv1alpha1.ResticBackup) (*backupv1alpha1.ResticRepository, error) {
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

func (r *ResticBackupReconciler) reconcileCronJob(ctx context.Context, backup *backupv1alpha1.ResticBackup, repository *backupv1alpha1.ResticRepository) error {
	log := log.FromContext(ctx)

	cronJob := r.buildCronJob(backup, repository)

	// Set owner reference
	if err := controllerutil.SetControllerReference(backup, cronJob, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Check if CronJob exists
	existingCronJob := &batchv1.CronJob{}
	err := r.Get(ctx, types.NamespacedName{Name: cronJob.Name, Namespace: cronJob.Namespace}, existingCronJob)

	if apierrors.IsNotFound(err) {
		log.Info("Creating CronJob", "name", cronJob.Name)
		if err := r.Create(ctx, cronJob); err != nil {
			return fmt.Errorf("failed to create CronJob: %w", err)
		}
		r.Recorder.Event(backup, corev1.EventTypeNormal, "CronJobCreated", fmt.Sprintf("Created CronJob %s", cronJob.Name))
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to get CronJob: %w", err)
	}

	// Update existing CronJob
	existingCronJob.Spec = cronJob.Spec
	if err := r.Update(ctx, existingCronJob); err != nil {
		return fmt.Errorf("failed to update CronJob: %w", err)
	}

	// Update status with CronJob reference
	backup.Status.CronJobRef = &backupv1alpha1.ObjectReference{
		Name:      cronJob.Name,
		Namespace: cronJob.Namespace,
	}

	return nil
}

func (r *ResticBackupReconciler) buildCronJob(backup *backupv1alpha1.ResticBackup, repository *backupv1alpha1.ResticRepository) *batchv1.CronJob {
	cronJobName := fmt.Sprintf("resticbackup-%s", backup.Name)

	// Build restic image
	resticImage := "ghcr.io/restic/restic:0.18.0"
	if backup.Spec.Restic != nil && backup.Spec.Restic.Image != "" {
		resticImage = backup.Spec.Restic.Image
	}

	// Build hostname
	hostname := backup.Name
	if backup.Spec.Restic != nil && backup.Spec.Restic.Hostname != "" {
		hostname = backup.Spec.Restic.Hostname
	}

	// Build tags
	var tags []string
	if backup.Spec.Restic != nil {
		tags = backup.Spec.Restic.Tags
	}

	// Build backup command
	backupCmd := r.buildBackupCommand(backup, hostname, tags)

	// Build pod template
	podSpec := r.buildPodSpec(backup, repository, resticImage, backupCmd)

	// Job configuration
	var successLimit, failLimit int32 = 3, 3
	var backoffLimit int32 = 0
	var activeDeadline int64 = 3600

	if backup.Spec.JobConfig != nil {
		if backup.Spec.JobConfig.SuccessfulJobsHistoryLimit != nil {
			successLimit = *backup.Spec.JobConfig.SuccessfulJobsHistoryLimit
		}
		if backup.Spec.JobConfig.FailedJobsHistoryLimit != nil {
			failLimit = *backup.Spec.JobConfig.FailedJobsHistoryLimit
		}
		if backup.Spec.JobConfig.BackoffLimit != nil {
			backoffLimit = *backup.Spec.JobConfig.BackoffLimit
		}
		if backup.Spec.JobConfig.ActiveDeadlineSeconds != nil {
			activeDeadline = *backup.Spec.JobConfig.ActiveDeadlineSeconds
		}
	}

	// Concurrency policy
	concurrencyPolicy := batchv1.ForbidConcurrent
	if backup.Spec.JobConfig != nil && backup.Spec.JobConfig.ConcurrencyPolicy != "" {
		switch backup.Spec.JobConfig.ConcurrencyPolicy {
		case "Allow":
			concurrencyPolicy = batchv1.AllowConcurrent
		case "Replace":
			concurrencyPolicy = batchv1.ReplaceConcurrent
		}
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: backup.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":        "restic-backup-operator",
				"app.kubernetes.io/component":   "backup",
				"app.kubernetes.io/managed-by":  "restic-backup-operator",
				"backup.resticbackup.io/backup": backup.Name,
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   backup.Spec.Schedule,
			Suspend:                    &backup.Spec.Suspend,
			ConcurrencyPolicy:          concurrencyPolicy,
			SuccessfulJobsHistoryLimit: &successLimit,
			FailedJobsHistoryLimit:     &failLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":        "restic-backup-operator",
						"app.kubernetes.io/component":   "backup",
						"backup.resticbackup.io/backup": backup.Name,
					},
				},
				Spec: batchv1.JobSpec{
					BackoffLimit:          &backoffLimit,
					ActiveDeadlineSeconds: &activeDeadline,
					Template:              podSpec,
				},
			},
		},
	}

	// Add timezone if specified
	if backup.Spec.Timezone != "" && backup.Spec.Timezone != "UTC" {
		cronJob.Spec.TimeZone = &backup.Spec.Timezone
	}

	return cronJob
}

func (r *ResticBackupReconciler) buildBackupCommand(backup *backupv1alpha1.ResticBackup, hostname string, tags []string) []string {
	cmd := []string{
		"restic", "backup",
		"--host", hostname,
	}

	for _, tag := range tags {
		cmd = append(cmd, "--tag", tag)
	}

	// Add excludes
	if backup.Spec.Source.PVC != nil {
		for _, exclude := range backup.Spec.Source.PVC.Excludes {
			cmd = append(cmd, "--exclude", exclude)
		}
	}

	// Add extra args
	if backup.Spec.Restic != nil {
		cmd = append(cmd, backup.Spec.Restic.ExtraArgs...)
	}

	// Add source paths
	if backup.Spec.Source.PVC != nil {
		if len(backup.Spec.Source.PVC.Paths) > 0 {
			for _, path := range backup.Spec.Source.PVC.Paths {
				cmd = append(cmd, "/backup"+path)
			}
		} else {
			cmd = append(cmd, "/backup")
		}
	}

	return cmd
}

func (r *ResticBackupReconciler) buildPodSpec(backup *backupv1alpha1.ResticBackup, repository *backupv1alpha1.ResticRepository, image string, command []string) corev1.PodTemplateSpec {
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
	}

	// Add AWS credentials if using S3
	envVars = append(envVars,
		corev1.EnvVar{
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
		corev1.EnvVar{
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
	)

	// Build volumes
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	// Add PVC volume if source is PVC
	if backup.Spec.Source.PVC != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "backup-source",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: backup.Spec.Source.PVC.ClaimName,
					ReadOnly:  true,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "backup-source",
			MountPath: "/backup",
			ReadOnly:  true,
		})
	}

	// Build security context
	securityContext := &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
		RunAsUser:    int64Ptr(65532),
		FSGroup:      int64Ptr(65532),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}

	if backup.Spec.JobConfig != nil && backup.Spec.JobConfig.SecurityContext != nil {
		securityContext = backup.Spec.JobConfig.SecurityContext
	}

	// Build container security context
	containerSecurityContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: boolPtr(false),
		ReadOnlyRootFilesystem:   boolPtr(false), // restic needs to write cache
		RunAsNonRoot:             boolPtr(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	// Build resources
	resources := corev1.ResourceRequirements{}
	if backup.Spec.JobConfig != nil && backup.Spec.JobConfig.Resources != nil {
		resources = *backup.Spec.JobConfig.Resources
	}

	container := corev1.Container{
		Name:            "restic",
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         command,
		Env:             envVars,
		VolumeMounts:    volumeMounts,
		SecurityContext: containerSecurityContext,
		Resources:       resources,
	}

	podSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/name":        "restic-backup-operator",
				"app.kubernetes.io/component":   "backup",
				"backup.resticbackup.io/backup": backup.Name,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:   corev1.RestartPolicyNever,
			SecurityContext: securityContext,
			Containers:      []corev1.Container{container},
			Volumes:         volumes,
		},
	}

	// Add node selector
	if backup.Spec.JobConfig != nil && backup.Spec.JobConfig.NodeSelector != nil {
		podSpec.Spec.NodeSelector = backup.Spec.JobConfig.NodeSelector
	}

	// Add tolerations
	if backup.Spec.JobConfig != nil && backup.Spec.JobConfig.Tolerations != nil {
		podSpec.Spec.Tolerations = backup.Spec.JobConfig.Tolerations
	}

	// Add affinity
	if backup.Spec.JobConfig != nil && backup.Spec.JobConfig.Affinity != nil {
		podSpec.Spec.Affinity = backup.Spec.JobConfig.Affinity
	}

	// Add service account
	if backup.Spec.JobConfig != nil && backup.Spec.JobConfig.ServiceAccountName != "" {
		podSpec.Spec.ServiceAccountName = backup.Spec.JobConfig.ServiceAccountName
	}

	return podSpec
}

func (r *ResticBackupReconciler) calculateNextBackup(backup *backupv1alpha1.ResticBackup) *metav1.Time {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(backup.Spec.Schedule)
	if err != nil {
		return nil
	}

	next := schedule.Next(time.Now())
	return &metav1.Time{Time: next}
}

func (r *ResticBackupReconciler) setCondition(backup *backupv1alpha1.ResticBackup, condition metav1.Condition) {
	conditions.SetCondition(&backup.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResticBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.ResticBackup{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
