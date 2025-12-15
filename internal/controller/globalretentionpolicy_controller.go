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
	"strings"
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

	"github.com/robfig/cron/v3"

	backupv1alpha1 "github.com/madic-creates/restic-backup-operator/api/v1alpha1"
	"github.com/madic-creates/restic-backup-operator/internal/conditions"
)

const (
	globalRetentionPolicyFinalizer = "backup.resticbackup.io/globalretentionpolicy-finalizer"
)

// GlobalRetentionPolicyReconciler reconciles a GlobalRetentionPolicy object
type GlobalRetentionPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=globalretentionpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=globalretentionpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.resticbackup.io,resources=globalretentionpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop.
func (r *GlobalRetentionPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling GlobalRetentionPolicy")

	// Fetch the GlobalRetentionPolicy instance
	policy := &backupv1alpha1.GlobalRetentionPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("GlobalRetentionPolicy resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GlobalRetentionPolicy")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !policy.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, policy)
	}

	// Add finalizer if missing
	if !controllerutil.ContainsFinalizer(policy, globalRetentionPolicyFinalizer) {
		controllerutil.AddFinalizer(policy, globalRetentionPolicyFinalizer)
		if err := r.Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Get the repository
	repository, err := r.getRepository(ctx, policy)
	if err != nil {
		log.Error(err, "Failed to get repository")
		r.setCondition(policy, conditions.NotReadyCondition("RepositoryNotFound", err.Error()))
		r.Recorder.Event(policy, corev1.EventTypeWarning, "RepositoryNotFound", err.Error())
		if updateErr := r.Status().Update(ctx, policy); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{RequeueAfter: errorRequeueInterval}, nil
	}

	// Check repository is ready
	if !conditions.IsConditionTrue(repository.Status.Conditions, "Ready") {
		log.Info("Repository not ready, requeuing")
		r.setCondition(policy, conditions.NotReadyCondition("RepositoryNotReady", "Referenced repository is not ready"))
		if err := r.Status().Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Reconcile CronJob
	if err := r.reconcileCronJob(ctx, policy, repository); err != nil {
		log.Error(err, "Failed to reconcile CronJob")
		r.setCondition(policy, conditions.NotReadyCondition("CronJobFailed", err.Error()))
		r.Recorder.Event(policy, corev1.EventTypeWarning, "CronJobFailed", err.Error())
		if updateErr := r.Status().Update(ctx, policy); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{RequeueAfter: errorRequeueInterval}, nil
	}

	// Calculate next run time
	nextRun := r.calculateNextRun(policy)
	if nextRun != nil {
		policy.Status.NextRun = nextRun
	}

	// Set Ready condition
	r.setCondition(policy, conditions.ReadyCondition("RetentionPolicyConfigured", "Retention policy CronJob is configured"))
	policy.Status.ObservedGeneration = policy.Generation

	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	r.Recorder.Event(policy, corev1.EventTypeNormal, "ReconcileSuccess", "Retention policy reconciled successfully")

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *GlobalRetentionPolicyReconciler) handleDeletion(ctx context.Context, policy *backupv1alpha1.GlobalRetentionPolicy) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(policy, globalRetentionPolicyFinalizer) {
		log.Info("Performing finalizer cleanup for GlobalRetentionPolicy")

		controllerutil.RemoveFinalizer(policy, globalRetentionPolicyFinalizer)
		if err := r.Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *GlobalRetentionPolicyReconciler) getRepository(ctx context.Context, policy *backupv1alpha1.GlobalRetentionPolicy) (*backupv1alpha1.ResticRepository, error) {
	repository := &backupv1alpha1.ResticRepository{}
	ns := policy.Spec.RepositoryRef.Namespace
	if ns == "" {
		ns = policy.Namespace
	}

	name := types.NamespacedName{
		Name:      policy.Spec.RepositoryRef.Name,
		Namespace: ns,
	}

	if err := r.Get(ctx, name, repository); err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return repository, nil
}

func (r *GlobalRetentionPolicyReconciler) reconcileCronJob(ctx context.Context, policy *backupv1alpha1.GlobalRetentionPolicy, repository *backupv1alpha1.ResticRepository) error {
	log := log.FromContext(ctx)

	cronJob := r.buildCronJob(policy, repository)

	// Set owner reference
	if err := controllerutil.SetControllerReference(policy, cronJob, r.Scheme); err != nil {
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
		r.Recorder.Event(policy, corev1.EventTypeNormal, "CronJobCreated", fmt.Sprintf("Created CronJob %s", cronJob.Name))
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
	policy.Status.CronJobRef = &backupv1alpha1.ObjectReference{
		Name:      cronJob.Name,
		Namespace: cronJob.Namespace,
	}

	return nil
}

func (r *GlobalRetentionPolicyReconciler) buildCronJob(policy *backupv1alpha1.GlobalRetentionPolicy, repository *backupv1alpha1.ResticRepository) *batchv1.CronJob {
	cronJobName := fmt.Sprintf("globalretention-%s", policy.Name)

	// Build the retention script
	script := r.buildRetentionScript(policy)

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

	var successLimit, failLimit int32 = 3, 3
	var backoffLimit int32 = 0
	var activeDeadline int64 = 7200 // 2 hours for retention

	if policy.Spec.JobConfig != nil {
		if policy.Spec.JobConfig.SuccessfulJobsHistoryLimit != nil {
			successLimit = *policy.Spec.JobConfig.SuccessfulJobsHistoryLimit
		}
		if policy.Spec.JobConfig.FailedJobsHistoryLimit != nil {
			failLimit = *policy.Spec.JobConfig.FailedJobsHistoryLimit
		}
		if policy.Spec.JobConfig.BackoffLimit != nil {
			backoffLimit = *policy.Spec.JobConfig.BackoffLimit
		}
		if policy.Spec.JobConfig.ActiveDeadlineSeconds != nil {
			activeDeadline = *policy.Spec.JobConfig.ActiveDeadlineSeconds
		}
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: policy.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":                 "restic-backup-operator",
				"app.kubernetes.io/component":            "retention",
				"app.kubernetes.io/managed-by":           "restic-backup-operator",
				"backup.resticbackup.io/retentionpolicy": policy.Name,
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   policy.Spec.Schedule,
			Suspend:                    &policy.Spec.Suspend,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: &successLimit,
			FailedJobsHistoryLimit:     &failLimit,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":                 "restic-backup-operator",
						"app.kubernetes.io/component":            "retention",
						"backup.resticbackup.io/retentionpolicy": policy.Name,
					},
				},
				Spec: batchv1.JobSpec{
					BackoffLimit:          &backoffLimit,
					ActiveDeadlineSeconds: &activeDeadline,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name":                 "restic-backup-operator",
								"app.kubernetes.io/component":            "retention",
								"backup.resticbackup.io/retentionpolicy": policy.Name,
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
									Image:           "ghcr.io/restic/restic:0.18.0",
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command:         []string{"/bin/sh", "-c"},
									Args:            []string{script},
									Env:             envVars,
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: boolPtr(false),
										ReadOnlyRootFilesystem:   boolPtr(false),
										RunAsNonRoot:             boolPtr(true),
										Capabilities: &corev1.Capabilities{
											Drop: []corev1.Capability{"ALL"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return cronJob
}

func (r *GlobalRetentionPolicyReconciler) buildRetentionScript(policy *backupv1alpha1.GlobalRetentionPolicy) string {
	// Pre-allocate: 2 header + 2 per policy + 2 optional prune + 1 footer
	capacity := 3 + 2*len(policy.Spec.Policies)
	if policy.Spec.Prune {
		capacity += 2
	}
	commands := make([]string, 0, capacity)

	commands = append(commands, "set -e")
	commands = append(commands, "echo 'Starting retention policy execution'")

	for i, p := range policy.Spec.Policies {
		cmd := "restic forget"

		// Add tag filter
		for _, tag := range p.Selector.Tags {
			cmd += fmt.Sprintf(" --tag %s", tag)
		}

		// Add hostname filter
		if p.Selector.Hostname != "" {
			cmd += fmt.Sprintf(" --host %s", p.Selector.Hostname)
		}

		// Add retention rules
		if p.Retention.KeepLast != nil && *p.Retention.KeepLast > 0 {
			cmd += fmt.Sprintf(" --keep-last %d", *p.Retention.KeepLast)
		}
		if p.Retention.KeepHourly != nil && *p.Retention.KeepHourly > 0 {
			cmd += fmt.Sprintf(" --keep-hourly %d", *p.Retention.KeepHourly)
		}
		if p.Retention.KeepDaily != nil && *p.Retention.KeepDaily > 0 {
			cmd += fmt.Sprintf(" --keep-daily %d", *p.Retention.KeepDaily)
		}
		if p.Retention.KeepWeekly != nil && *p.Retention.KeepWeekly > 0 {
			cmd += fmt.Sprintf(" --keep-weekly %d", *p.Retention.KeepWeekly)
		}
		if p.Retention.KeepMonthly != nil && *p.Retention.KeepMonthly > 0 {
			cmd += fmt.Sprintf(" --keep-monthly %d", *p.Retention.KeepMonthly)
		}
		if p.Retention.KeepYearly != nil && *p.Retention.KeepYearly > 0 {
			cmd += fmt.Sprintf(" --keep-yearly %d", *p.Retention.KeepYearly)
		}

		commands = append(commands, fmt.Sprintf("echo 'Executing policy %d'", i+1))
		commands = append(commands, cmd)
	}

	// Add prune if enabled
	if policy.Spec.Prune {
		commands = append(commands, "echo 'Running prune'")
		commands = append(commands, "restic prune")
	}

	commands = append(commands, "echo 'Retention policy execution completed'")

	return strings.Join(commands, "\n")
}

func (r *GlobalRetentionPolicyReconciler) calculateNextRun(policy *backupv1alpha1.GlobalRetentionPolicy) *metav1.Time {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(policy.Spec.Schedule)
	if err != nil {
		return nil
	}

	next := schedule.Next(time.Now())
	return &metav1.Time{Time: next}
}

func (r *GlobalRetentionPolicyReconciler) setCondition(policy *backupv1alpha1.GlobalRetentionPolicy, condition metav1.Condition) {
	conditions.SetCondition(&policy.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GlobalRetentionPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.GlobalRetentionPolicy{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
