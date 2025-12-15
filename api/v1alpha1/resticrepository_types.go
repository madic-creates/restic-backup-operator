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

// IntegrityCheckConfig configures repository integrity checks.
type IntegrityCheckConfig struct {
	// Enabled enables periodic integrity checks.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Schedule is the cron schedule for integrity checks.
	// +kubebuilder:validation:Pattern=`^(@(annually|yearly|monthly|weekly|daily|hourly))|(((\d+,)*\d+|(\d+(\/|-)\d+)|\d+|\*)\s?){5}$`
	// +optional
	Schedule string `json:"schedule,omitempty"`
}

// CacheConfig configures the restic cache.
type CacheConfig struct {
	// Enabled enables the cache.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Size is the size limit for the cache PVC.
	// +kubebuilder:default="5Gi"
	// +optional
	Size string `json:"size,omitempty"`

	// StorageClassName is the storage class for the cache PVC.
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`
}

// RepositoryStatistics contains repository statistics.
type RepositoryStatistics struct {
	// TotalSize is the total size of the repository.
	// +optional
	TotalSize string `json:"totalSize,omitempty"`

	// TotalFileCount is the total number of files in the repository.
	// +optional
	TotalFileCount int64 `json:"totalFileCount,omitempty"`

	// SnapshotCount is the number of snapshots in the repository.
	// +optional
	SnapshotCount int32 `json:"snapshotCount,omitempty"`
}

// ResticRepositorySpec defines the desired state of ResticRepository.
type ResticRepositorySpec struct {
	// RepositoryURL is the restic repository URL (s3:, sftp:, rest:, azure:, gs:, b2:, swift:).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^(s3|sftp|rest|azure|gs|b2|swift|rclone|local):.*`
	RepositoryURL string `json:"repositoryURL"`

	// CredentialsSecretRef references the secret containing repository credentials.
	// Expected keys: RESTIC_PASSWORD (required), AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY (for S3).
	// +kubebuilder:validation:Required
	CredentialsSecretRef SecretKeySelector `json:"credentialsSecretRef"`

	// IntegrityCheck configures periodic repository integrity verification.
	// +optional
	IntegrityCheck *IntegrityCheckConfig `json:"integrityCheck,omitempty"`

	// Cache configures the restic cache.
	// +optional
	Cache *CacheConfig `json:"cache,omitempty"`
}

// ResticRepositoryStatus defines the observed state of ResticRepository.
type ResticRepositoryStatus struct {
	// Conditions represent the latest available observations of the repository's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastIntegrityCheck is the timestamp of the last integrity check.
	// +optional
	LastIntegrityCheck *metav1.Time `json:"lastIntegrityCheck,omitempty"`

	// LastIntegrityCheckResult stores the result of the last check.
	// +optional
	LastIntegrityCheckResult string `json:"lastIntegrityCheckResult,omitempty"`

	// Statistics contains repository statistics.
	// +optional
	Statistics *RepositoryStatistics `json:"statistics,omitempty"`

	// ObservedGeneration reflects the generation of the spec observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rr
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.repositoryURL"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Snapshots",type="integer",JSONPath=".status.statistics.snapshotCount"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResticRepository is the Schema for the resticrepositories API.
type ResticRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResticRepositorySpec   `json:"spec,omitempty"`
	Status ResticRepositoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResticRepositoryList contains a list of ResticRepository.
type ResticRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResticRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResticRepository{}, &ResticRepositoryList{})
}
