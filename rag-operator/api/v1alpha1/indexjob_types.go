/*
Copyright 2024.

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

// IndexJob Phase constants
const (
	IndexJobPhasePending    = "Pending"
	IndexJobPhaseBuilding   = "Building"
	IndexJobPhaseOptimizing = "Optimizing"
	IndexJobPhaseSucceeded  = "Succeeded"
	IndexJobPhaseFailed     = "Failed"
)

// IndexJob Condition types
const (
	IndexJobConditionIndexCreated   = "IndexCreated"
	IndexJobConditionIndexOptimized = "IndexOptimized"
	IndexJobConditionAliasSwapped   = "AliasSwapped"
)

// Index types
const (
	IndexTypeHNSW    = "HNSW"
	IndexTypeIVFFlat = "IVF_FLAT"
	IndexTypeIVFPQ   = "IVF_PQ"
)

// IndexJobSpec defines the desired state of IndexJob
type IndexJobSpec struct {
	// DocumentSet is the name of the source DocumentSet
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	DocumentSet string `json:"documentSet"`

	// VectorDB configuration
	// +kubebuilder:validation:Required
	VectorDB VectorDBSpec `json:"vectorDB"`

	// TargetAlias is the alias to swap to after indexing completes
	// +optional
	TargetAlias string `json:"targetAlias,omitempty"`

	// IndexSpec defines the index type and parameters
	// +kubebuilder:validation:Required
	IndexSpec IndexConfig `json:"indexSpec"`

	// RetryPolicy for the job
	// +optional
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

type IndexConfig struct {
	// Type of index: HNSW, IVF_FLAT, IVF_PQ
	// +kubebuilder:validation:Enum=HNSW;IVF_FLAT;IVF_PQ
	// +kubebuilder:default=HNSW
	Type string `json:"type"`

	// Parameters for the index (e.g., efConstruction, M for HNSW)
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// IndexJobStatus defines the observed state of IndexJob
type IndexJobStatus struct {
	// Phase represents the current state: Pending, Building, Optimizing, Succeeded, Failed
	// +kubebuilder:validation:Enum=Pending;Building;Optimizing;Succeeded;Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// Progress tracks the index building progress
	// +optional
	Progress IndexProgress `json:"progress,omitempty"`

	// Message provides additional status information
	// +optional
	Message string `json:"message,omitempty"`

	// StartTime is when the job started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the job completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Conditions track detailed status
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// JobRef is the name of the underlying Kubernetes Job
	// +optional
	JobRef string `json:"jobRef,omitempty"`

	// AliasSwapped indicates if the alias has been swapped successfully
	// +optional
	AliasSwapped bool `json:"aliasSwapped,omitempty"`

	// RetryCount is the number of retries attempted
	// +optional
	RetryCount int `json:"retryCount,omitempty"`
}

type IndexProgress struct {
	// IndexedVectors is the number of vectors indexed
	// +optional
	IndexedVectors int `json:"indexedVectors,omitempty"`

	// TotalVectors is the total number of vectors to index
	// +optional
	TotalVectors int `json:"totalVectors,omitempty"`

	// Percentage is the completion percentage (0-100)
	// +optional
	Percentage int `json:"percentage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

// IndexJob is the Schema for the indexjobs API
type IndexJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IndexJobSpec   `json:"spec,omitempty"`
	Status IndexJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IndexJobList contains a list of IndexJob
type IndexJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IndexJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IndexJob{}, &IndexJobList{})
}
