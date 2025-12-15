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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types for all CRDs
const (
	// ConditionReady indicates the resource is ready and operational.
	ConditionReady = "Ready"
	// ConditionRepositoryReady indicates the referenced repository is accessible.
	ConditionRepositoryReady = "RepositoryReady"
	// ConditionProgressing indicates an operation is in progress.
	ConditionProgressing = "Progressing"
	// ConditionDegraded indicates the resource is operational but experiencing issues.
	ConditionDegraded = "Degraded"
)

// SecretKeySelector selects a key from a Secret.
type SecretKeySelector struct {
	// Name of the secret in the same namespace.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key within the secret to select.
	// +optional
	Key string `json:"key,omitempty"`
}

// CrossNamespaceObjectReference references a resource in a potentially different namespace.
type CrossNamespaceObjectReference struct {
	// Name of the resource.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the resource. If empty, uses the same namespace as the referencing resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ObjectReference references a resource in the same namespace.
type ObjectReference struct {
	// Name of the resource.
	Name string `json:"name"`

	// Namespace of the resource.
	Namespace string `json:"namespace"`
}

// RetentionPolicy defines snapshot retention rules.
type RetentionPolicy struct {
	// KeepLast specifies the number of last snapshots to keep.
	// +kubebuilder:validation:Minimum=0
	// +optional
	KeepLast *int32 `json:"keepLast,omitempty"`

	// KeepHourly specifies the number of hourly snapshots to keep.
	// +kubebuilder:validation:Minimum=0
	// +optional
	KeepHourly *int32 `json:"keepHourly,omitempty"`

	// KeepDaily specifies the number of daily snapshots to keep.
	// +kubebuilder:validation:Minimum=0
	// +optional
	KeepDaily *int32 `json:"keepDaily,omitempty"`

	// KeepWeekly specifies the number of weekly snapshots to keep.
	// +kubebuilder:validation:Minimum=0
	// +optional
	KeepWeekly *int32 `json:"keepWeekly,omitempty"`

	// KeepMonthly specifies the number of monthly snapshots to keep.
	// +kubebuilder:validation:Minimum=0
	// +optional
	KeepMonthly *int32 `json:"keepMonthly,omitempty"`

	// KeepYearly specifies the number of yearly snapshots to keep.
	// +kubebuilder:validation:Minimum=0
	// +optional
	KeepYearly *int32 `json:"keepYearly,omitempty"`
}

// PushgatewayConfig configures Prometheus Pushgateway notifications.
type PushgatewayConfig struct {
	// Enabled enables Pushgateway notifications.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// URL of the Pushgateway.
	// +kubebuilder:validation:Pattern=`^https?://.*`
	URL string `json:"url"`

	// JobName is the job name in Pushgateway. Defaults to "backup".
	// +optional
	JobName string `json:"jobName,omitempty"`
}

// NtfyConfig configures ntfy push notifications.
type NtfyConfig struct {
	// Enabled enables ntfy notifications.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// ServerURL is the ntfy server URL.
	// +kubebuilder:validation:Pattern=`^https?://.*`
	ServerURL string `json:"serverURL"`

	// Topic is the ntfy topic.
	// +kubebuilder:validation:Required
	Topic string `json:"topic"`

	// CredentialsSecretRef references a secret containing ntfy credentials.
	// The secret should contain a key 'auth-header' with the Authorization header value.
	// +optional
	CredentialsSecretRef *SecretKeySelector `json:"credentialsSecretRef,omitempty"`

	// OnlyOnFailure sends notifications only on failure.
	// +optional
	OnlyOnFailure bool `json:"onlyOnFailure,omitempty"`

	// Priority is the notification priority (1-5).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=4
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// Tags are ntfy notification tags.
	// +optional
	Tags []string `json:"tags,omitempty"`
}

// NotificationConfig configures backup notifications.
type NotificationConfig struct {
	// Pushgateway configures Prometheus Pushgateway notifications.
	// +optional
	Pushgateway *PushgatewayConfig `json:"pushgateway,omitempty"`

	// Ntfy configures ntfy push notifications.
	// +optional
	Ntfy *NtfyConfig `json:"ntfy,omitempty"`
}

// ExecHook defines an exec hook to run in an existing pod.
type ExecHook struct {
	// PodSelector selects the pod to execute the command in.
	// +kubebuilder:validation:Required
	PodSelector metav1.LabelSelector `json:"podSelector"`

	// Container is the container name to execute the command in.
	// +optional
	Container string `json:"container,omitempty"`

	// Command is the command to execute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Command []string `json:"command"`

	// Timeout is the maximum duration for the hook to run.
	// +kubebuilder:default="60s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

// JobHook defines a job-based hook.
type JobHook struct {
	// PodTemplate defines the pod specification for the hook job.
	// +kubebuilder:validation:Required
	PodTemplate corev1.PodTemplateSpec `json:"podTemplate"`
}

// Hook defines a hook that can be executed.
type Hook struct {
	// Exec defines an exec hook to run in an existing pod.
	// +optional
	Exec *ExecHook `json:"exec,omitempty"`

	// Job defines a job-based hook.
	// +optional
	Job *JobHook `json:"job,omitempty"`
}

// BackupHooks defines hooks for backup operations.
type BackupHooks struct {
	// PreBackup runs before the backup starts.
	// +optional
	PreBackup *Hook `json:"preBackup,omitempty"`

	// PostBackup runs after a successful backup.
	// +optional
	PostBackup *Hook `json:"postBackup,omitempty"`

	// OnFailure runs when a backup fails.
	// +optional
	OnFailure *Hook `json:"onFailure,omitempty"`
}

// RestoreHooks defines hooks for restore operations.
type RestoreHooks struct {
	// PreRestore runs before the restore starts.
	// +optional
	PreRestore *Hook `json:"preRestore,omitempty"`

	// PostRestore runs after a successful restore.
	// +optional
	PostRestore *Hook `json:"postRestore,omitempty"`
}

// JobConfiguration configures the backup/restore job.
type JobConfiguration struct {
	// ConcurrencyPolicy specifies how to treat concurrent executions.
	// +kubebuilder:validation:Enum=Allow;Forbid;Replace
	// +kubebuilder:default=Forbid
	// +optional
	ConcurrencyPolicy string `json:"concurrencyPolicy,omitempty"`

	// SuccessfulJobsHistoryLimit specifies how many successful jobs to keep.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	// +optional
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`

	// FailedJobsHistoryLimit specifies how many failed jobs to keep.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	// +optional
	FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty"`

	// ActiveDeadlineSeconds specifies the job timeout.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3600
	// +optional
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`

	// BackoffLimit specifies the number of retries before considering a job as failed.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// SecurityContext defines the security context for the backup pod.
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// Resources defines resource requirements for the backup container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// NodeSelector defines node selection constraints.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations defines pod tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity defines pod affinity rules.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// ServiceAccountName specifies the service account for the backup pod.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}
