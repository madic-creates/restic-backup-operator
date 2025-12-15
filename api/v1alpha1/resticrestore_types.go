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

// SnapshotSelector defines how to select a snapshot.
type SnapshotSelector struct {
	// Latest selects the latest snapshot.
	// +optional
	Latest bool `json:"latest,omitempty"`

	// Tags filters snapshots by tags.
	// +optional
	Tags []string `json:"tags,omitempty"`

	// Hostname filters snapshots by hostname.
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Before selects the latest snapshot before this time.
	// +optional
	Before *metav1.Time `json:"before,omitempty"`
}

// PVCTarget defines a PVC as restore target.
type PVCTarget struct {
	// ClaimName is the name of the PVC to restore to.
	// +kubebuilder:validation:Required
	ClaimName string `json:"claimName"`

	// Path is the path within the PVC to restore to. Defaults to "/".
	// +optional
	Path string `json:"path,omitempty"`
}

// NewPVCTarget defines creating a new PVC for restore.
type NewPVCTarget struct {
	// Name is the name of the new PVC.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// StorageClassName is the storage class for the new PVC.
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// AccessModes are the access modes for the new PVC.
	// +optional
	AccessModes []string `json:"accessModes,omitempty"`

	// Size is the size of the new PVC.
	// +kubebuilder:validation:Required
	Size string `json:"size"`
}

// RestoreTarget defines where to restore data.
type RestoreTarget struct {
	// PVC defines restoring to an existing PVC.
	// +optional
	PVC *PVCTarget `json:"pvc,omitempty"`

	// NewPVC defines creating a new PVC for restore.
	// +optional
	NewPVC *NewPVCTarget `json:"newPVC,omitempty"`
}

// RestoreOptions configures restore behavior.
type RestoreOptions struct {
	// Overwrite enables overwriting existing files.
	// +kubebuilder:default=true
	// +optional
	Overwrite bool `json:"overwrite,omitempty"`

	// Verify enables verification of restored data.
	// +optional
	Verify bool `json:"verify,omitempty"`
}

// RestorePhase represents the current phase of a restore operation.
// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed
type RestorePhase string

const (
	// RestorePhasePending indicates the restore has not started.
	RestorePhasePending RestorePhase = "Pending"
	// RestorePhaseInProgress indicates the restore is running.
	RestorePhaseInProgress RestorePhase = "InProgress"
	// RestorePhaseCompleted indicates the restore completed successfully.
	RestorePhaseCompleted RestorePhase = "Completed"
	// RestorePhaseFailed indicates the restore failed.
	RestorePhaseFailed RestorePhase = "Failed"
)

// ResticRestoreSpec defines the desired state of ResticRestore.
type ResticRestoreSpec struct {
	// BackupRef references the ResticBackup CR for repository info.
	// +kubebuilder:validation:Required
	BackupRef CrossNamespaceObjectReference `json:"backupRef"`

	// SnapshotID specifies the exact snapshot to restore.
	// +optional
	SnapshotID string `json:"snapshotID,omitempty"`

	// SnapshotSelector selects a snapshot if SnapshotID is not specified.
	// +optional
	SnapshotSelector *SnapshotSelector `json:"snapshotSelector,omitempty"`

	// Target defines where to restore data.
	// +kubebuilder:validation:Required
	Target RestoreTarget `json:"target"`

	// IncludePaths specifies paths to restore. Defaults to all.
	// +optional
	IncludePaths []string `json:"includePaths,omitempty"`

	// ExcludePaths specifies paths to exclude from restore.
	// +optional
	ExcludePaths []string `json:"excludePaths,omitempty"`

	// Options configures restore behavior.
	// +optional
	Options *RestoreOptions `json:"options,omitempty"`

	// Hooks defines pre/post restore hooks.
	// +optional
	Hooks *RestoreHooks `json:"hooks,omitempty"`

	// JobConfig configures the restore job.
	// +optional
	JobConfig *JobConfiguration `json:"jobConfig,omitempty"`
}

// ResticRestoreStatus defines the observed state of ResticRestore.
type ResticRestoreStatus struct {
	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase is the current phase of the restore operation.
	// +optional
	Phase RestorePhase `json:"phase,omitempty"`

	// StartTime is when the restore started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the restore completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// RestoredSnapshot is the ID of the restored snapshot.
	// +optional
	RestoredSnapshot string `json:"restoredSnapshot,omitempty"`

	// RestoredFiles is the number of restored files.
	// +optional
	RestoredFiles int64 `json:"restoredFiles,omitempty"`

	// RestoredSize is the size of restored data.
	// +optional
	RestoredSize string `json:"restoredSize,omitempty"`

	// JobRef references the restore job.
	// +optional
	JobRef *ObjectReference `json:"jobRef,omitempty"`

	// ObservedGeneration reflects the generation of the spec observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rres
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Snapshot",type="string",JSONPath=".status.restoredSnapshot"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResticRestore is the Schema for the resticrestores API.
type ResticRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResticRestoreSpec   `json:"spec,omitempty"`
	Status ResticRestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResticRestoreList contains a list of ResticRestore.
type ResticRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResticRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResticRestore{}, &ResticRestoreList{})
}
