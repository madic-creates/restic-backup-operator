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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RetentionSelector defines how to select snapshots for a retention policy.
type RetentionSelector struct {
	// Tags filters snapshots by tags.
	// +optional
	Tags []string `json:"tags,omitempty"`

	// Hostname filters snapshots by hostname.
	// +optional
	Hostname string `json:"hostname,omitempty"`
}

// RetentionPolicyEntry defines a retention policy for a set of snapshots.
type RetentionPolicyEntry struct {
	// Selector selects snapshots for this policy.
	// +kubebuilder:validation:Required
	Selector RetentionSelector `json:"selector"`

	// Retention defines the retention rules.
	// +kubebuilder:validation:Required
	Retention RetentionPolicy `json:"retention"`
}

// EmailNotificationConfig configures email notifications.
type EmailNotificationConfig struct {
	// Enabled enables email notifications.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// SMTPServer is the SMTP server address.
	// +optional
	SMTPServer string `json:"smtpServer,omitempty"`

	// From is the sender email address.
	// +optional
	From string `json:"from,omitempty"`

	// To is the recipient email address.
	// +optional
	To string `json:"to,omitempty"`

	// Subject is the email subject.
	// +optional
	Subject string `json:"subject,omitempty"`
}

// GlobalRetentionNotificationConfig configures notifications for retention operations.
type GlobalRetentionNotificationConfig struct {
	// Email configures email notifications.
	// +optional
	Email *EmailNotificationConfig `json:"email,omitempty"`

	// Ntfy configures ntfy notifications.
	// +optional
	Ntfy *NtfyConfig `json:"ntfy,omitempty"`
}

// GlobalRetentionPolicySpec defines the desired state of GlobalRetentionPolicy.
type GlobalRetentionPolicySpec struct {
	// RepositoryRef references the ResticRepository.
	// +kubebuilder:validation:Required
	RepositoryRef CrossNamespaceObjectReference `json:"repositoryRef"`

	// Schedule is the cron schedule for retention runs.
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`

	// Policies defines retention policies per tag/hostname.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Policies []RetentionPolicyEntry `json:"policies"`

	// Prune runs prune after all forget operations.
	// +optional
	Prune bool `json:"prune,omitempty"`

	// Notifications configures retention notifications.
	// +optional
	Notifications *GlobalRetentionNotificationConfig `json:"notifications,omitempty"`

	// JobConfig configures the retention job.
	// +optional
	JobConfig *JobConfiguration `json:"jobConfig,omitempty"`

	// Suspend suspends retention scheduling.
	// +kubebuilder:default=false
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// GlobalRetentionPolicyStatus defines the observed state of GlobalRetentionPolicy.
type GlobalRetentionPolicyStatus struct {
	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastRun is the timestamp of the last retention run.
	// +optional
	LastRun *metav1.Time `json:"lastRun,omitempty"`

	// LastRunResult is the result of the last retention run.
	// +optional
	LastRunResult string `json:"lastRunResult,omitempty"`

	// LastRunDuration is the duration of the last retention run.
	// +optional
	LastRunDuration string `json:"lastRunDuration,omitempty"`

	// RepositorySizeBefore is the repository size before the last run.
	// +optional
	RepositorySizeBefore string `json:"repositorySizeBefore,omitempty"`

	// RepositorySizeAfter is the repository size after the last run.
	// +optional
	RepositorySizeAfter string `json:"repositorySizeAfter,omitempty"`

	// SnapshotsRemoved is the number of snapshots removed in the last run.
	// +optional
	SnapshotsRemoved int32 `json:"snapshotsRemoved,omitempty"`

	// NextRun is the timestamp of the next scheduled run.
	// +optional
	NextRun *metav1.Time `json:"nextRun,omitempty"`

	// CronJobRef references the managed CronJob.
	// +optional
	CronJobRef *ObjectReference `json:"cronJobRef,omitempty"`

	// ObservedGeneration reflects the generation of the spec observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=grp
// +kubebuilder:printcolumn:name="Schedule",type="string",JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Last Run",type="date",JSONPath=".status.lastRun"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// GlobalRetentionPolicy is the Schema for the globalretentionpolicies API.
type GlobalRetentionPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GlobalRetentionPolicySpec   `json:"spec,omitempty"`
	Status GlobalRetentionPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GlobalRetentionPolicyList contains a list of GlobalRetentionPolicy.
type GlobalRetentionPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalRetentionPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GlobalRetentionPolicy{}, &GlobalRetentionPolicyList{})
}
