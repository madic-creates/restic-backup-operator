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

// PVCSource defines a PVC as backup source.
type PVCSource struct {
	// ClaimName is the name of the PVC to backup.
	// +kubebuilder:validation:Required
	ClaimName string `json:"claimName"`

	// Paths are the paths within the PVC to backup. Defaults to "/".
	// +optional
	Paths []string `json:"paths,omitempty"`

	// Excludes are paths to exclude from the backup.
	// +optional
	Excludes []string `json:"excludes,omitempty"`
}

// PodVolumeBackupSource defines backing up a volume from a running pod.
type PodVolumeBackupSource struct {
	// Selector selects the pod to backup from.
	// +kubebuilder:validation:Required
	Selector metav1.LabelSelector `json:"selector"`

	// VolumeName is the name of the volume in the pod to backup.
	// +kubebuilder:validation:Required
	VolumeName string `json:"volumeName"`

	// Container is the container name to use. If empty, uses the first container.
	// +optional
	Container string `json:"container,omitempty"`
}

// CustomSource defines a custom backup source using a pod template.
type CustomSource struct {
	// PodTemplate defines the pod specification for the custom backup.
	// +kubebuilder:validation:Required
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	PodTemplate corev1.PodTemplateSpec `json:"podTemplate"`

	// BackupPath is the path in the pod where backup data is written.
	// +kubebuilder:validation:Required
	BackupPath string `json:"backupPath"`
}

// BackupSource defines the source for backup data.
type BackupSource struct {
	// PVC defines a PersistentVolumeClaim as the backup source.
	// +optional
	PVC *PVCSource `json:"pvc,omitempty"`

	// PodVolumeBackup defines backing up a volume from a running pod.
	// +optional
	PodVolumeBackup *PodVolumeBackupSource `json:"podVolumeBackup,omitempty"`

	// CustomSource defines a custom backup source.
	// +optional
	CustomSource *CustomSource `json:"customSource,omitempty"`
}

// ResticConfig defines restic-specific configuration.
type ResticConfig struct {
	// Hostname is the hostname for snapshots. Defaults to the CR name.
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Tags are tags for this backup.
	// +optional
	Tags []string `json:"tags,omitempty"`

	// ExtraArgs are additional restic backup arguments.
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`

	// Image is the container image for restic.
	// +kubebuilder:default="ghcr.io/restic/restic:0.18.0"
	// +optional
	Image string `json:"image,omitempty"`
}

// RetentionConfig configures snapshot retention.
type RetentionConfig struct {
	// Enabled enables retention after each backup.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Policy defines the retention policy.
	// +optional
	Policy *RetentionPolicy `json:"policy,omitempty"`

	// Prune runs prune after forget (can be expensive).
	// +optional
	Prune bool `json:"prune,omitempty"`

	// GroupBy specifies the grouping for retention. Defaults to "host,tags".
	// +optional
	GroupBy []string `json:"groupBy,omitempty"`
}

// BackupRunStatus contains information about a backup run.
type BackupRunStatus struct {
	// StartTime is when the backup started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the backup completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Duration is the backup duration.
	// +optional
	Duration string `json:"duration,omitempty"`

	// SnapshotID is the ID of the created snapshot.
	// +optional
	SnapshotID string `json:"snapshotID,omitempty"`

	// Result is the backup result: Succeeded, Failed, PartiallyFailed.
	// +optional
	Result string `json:"result,omitempty"`
}

// BackupStatistics contains backup statistics.
type BackupStatistics struct {
	// TotalBackups is the total number of backups.
	// +optional
	TotalBackups int32 `json:"totalBackups,omitempty"`

	// SuccessfulBackups is the number of successful backups.
	// +optional
	SuccessfulBackups int32 `json:"successfulBackups,omitempty"`

	// FailedBackups is the number of failed backups.
	// +optional
	FailedBackups int32 `json:"failedBackups,omitempty"`

	// LastBackupSize is the size of the last backup.
	// +optional
	LastBackupSize string `json:"lastBackupSize,omitempty"`

	// LastBackupFiles is the number of files in the last backup.
	// +optional
	LastBackupFiles int64 `json:"lastBackupFiles,omitempty"`
}

// ResticBackupSpec defines the desired state of ResticBackup.
type ResticBackupSpec struct {
	// RepositoryRef references the ResticRepository to use.
	// +kubebuilder:validation:Required
	RepositoryRef CrossNamespaceObjectReference `json:"repositoryRef"`

	// Schedule is the backup schedule in cron format.
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`

	// Timezone is the timezone for schedule interpretation.
	// +kubebuilder:default="UTC"
	// +optional
	Timezone string `json:"timezone,omitempty"`

	// Source defines what to backup.
	// +kubebuilder:validation:Required
	Source BackupSource `json:"source"`

	// Restic contains restic-specific configuration.
	// +optional
	Restic *ResticConfig `json:"restic,omitempty"`

	// Hooks defines pre/post backup hooks.
	// +optional
	Hooks *BackupHooks `json:"hooks,omitempty"`

	// Retention configures snapshot retention.
	// +optional
	Retention *RetentionConfig `json:"retention,omitempty"`

	// Notifications configures backup notifications.
	// +optional
	Notifications *NotificationConfig `json:"notifications,omitempty"`

	// JobConfig configures the backup job/cronjob.
	// +optional
	JobConfig *JobConfiguration `json:"jobConfig,omitempty"`

	// Suspend suspends backup scheduling.
	// +kubebuilder:default=false
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ResticBackupStatus defines the observed state of ResticBackup.
type ResticBackupStatus struct {
	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastBackup contains information about the last backup.
	// +optional
	LastBackup *BackupRunStatus `json:"lastBackup,omitempty"`

	// LastSuccessfulBackup is the timestamp of the last successful backup.
	// +optional
	LastSuccessfulBackup *metav1.Time `json:"lastSuccessfulBackup,omitempty"`

	// NextBackup is the timestamp of the next scheduled backup.
	// +optional
	NextBackup *metav1.Time `json:"nextBackup,omitempty"`

	// Statistics contains backup statistics.
	// +optional
	Statistics *BackupStatistics `json:"statistics,omitempty"`

	// LastRetentionRun is the timestamp of the last retention run.
	// +optional
	LastRetentionRun *metav1.Time `json:"lastRetentionRun,omitempty"`

	// SnapshotsAfterRetention is the number of snapshots after retention.
	// +optional
	SnapshotsAfterRetention int32 `json:"snapshotsAfterRetention,omitempty"`

	// CronJobRef references the managed CronJob.
	// +optional
	CronJobRef *ObjectReference `json:"cronJobRef,omitempty"`

	// ObservedGeneration reflects the generation of the spec observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rb
// +kubebuilder:printcolumn:name="Schedule",type="string",JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Last Backup",type="date",JSONPath=".status.lastSuccessfulBackup"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResticBackup is the Schema for the resticbackups API.
type ResticBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResticBackupSpec   `json:"spec,omitempty"`
	Status ResticBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResticBackupList contains a list of ResticBackup.
type ResticBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResticBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResticBackup{}, &ResticBackupList{})
}
