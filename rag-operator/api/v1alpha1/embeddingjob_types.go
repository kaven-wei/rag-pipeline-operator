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

// EmbeddingJob Phase constants
const (
	EmbeddingJobPhasePending   = "Pending"
	EmbeddingJobPhaseRunning   = "Running"
	EmbeddingJobPhaseSucceeded = "Succeeded"
	EmbeddingJobPhaseFailed    = "Failed"
)

// EmbeddingJob Condition types
const (
	EmbeddingJobConditionStarted        = "JobStarted"
	EmbeddingJobConditionVectorUpserted = "VectorUpserted"
)

// EmbeddingJobSpec defines the desired state of EmbeddingJob
type EmbeddingJobSpec struct {
	// DocumentSet is the name of the source DocumentSet
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	DocumentSet string `json:"documentSet"`

	// EmbeddingModel to use for generating embeddings
	// +kubebuilder:validation:Required
	EmbeddingModel string `json:"embeddingModel"`

	// VectorDB configuration for storing vectors
	// +kubebuilder:validation:Required
	VectorDB VectorDBSpec `json:"vectorDB"`

	// RetryPolicy for the job
	// +optional
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

type VectorDBSpec struct {
	// Type of vector database: milvus, qdrant, weaviate
	// +kubebuilder:validation:Enum=milvus;qdrant;weaviate
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// Collection name to store vectors
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Collection string `json:"collection"`

	// Endpoint of the vector database
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// SecretRef for vector database credentials
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	// +optional
	MaxRetries int `json:"maxRetries,omitempty"`

	// BackoffSeconds is the initial backoff duration in seconds
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=30
	// +optional
	BackoffSeconds int `json:"backoffSeconds,omitempty"`
}

// EmbeddingJobStatus defines the observed state of EmbeddingJob
type EmbeddingJobStatus struct {
	// Phase represents the current state: Pending, Running, Succeeded, Failed
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// Progress tracks the job progress
	// +optional
	Progress JobProgress `json:"progress,omitempty"`

	// StartTime is when the job started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the job completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Message provides additional status information
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions track detailed status
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// JobRef is the name of the underlying Kubernetes Job
	// +optional
	JobRef string `json:"jobRef,omitempty"`

	// RetryCount is the number of retries attempted
	// +optional
	RetryCount int `json:"retryCount,omitempty"`
}

type JobProgress struct {
	// TotalChunks is the total number of chunks to process
	// +optional
	TotalChunks int `json:"totalChunks,omitempty"`

	// ProcessedChunks is the number of chunks processed
	// +optional
	ProcessedChunks int `json:"processedChunks,omitempty"`

	// Percentage is the completion percentage (0-100)
	// +optional
	Percentage int `json:"percentage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

// EmbeddingJob is the Schema for the embeddingjobs API
type EmbeddingJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EmbeddingJobSpec   `json:"spec,omitempty"`
	Status EmbeddingJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EmbeddingJobList contains a list of EmbeddingJob
type EmbeddingJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EmbeddingJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EmbeddingJob{}, &EmbeddingJobList{})
}
