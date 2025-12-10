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

// EmbeddingJobSpec defines the desired state of EmbeddingJob
type EmbeddingJobSpec struct {
	// DocumentSet is the name of the source DocumentSet
	DocumentSet string `json:"documentSet"`

	// EmbeddingModel to use
	EmbeddingModel string `json:"embeddingModel"`

	// VectorDB configuration
	VectorDB VectorDBSpec `json:"vectorDB"`

	// RetryPolicy for the job
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

type VectorDBSpec struct {
	Type       string `json:"type"`
	Collection string `json:"collection"`
}

type RetryPolicy struct {
	MaxRetries     int `json:"maxRetries,omitempty"`
	BackoffSeconds int `json:"backoffSeconds,omitempty"`
}

// EmbeddingJobStatus defines the observed state of EmbeddingJob
type EmbeddingJobStatus struct {
	Phase          string             `json:"phase,omitempty"`
	Progress       JobProgress        `json:"progress,omitempty"`
	StartTime      *metav1.Time       `json:"startTime,omitempty"`
	CompletionTime *metav1.Time       `json:"completionTime,omitempty"`
	Message        string             `json:"message,omitempty"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
}

type JobProgress struct {
	TotalChunks     int `json:"totalChunks,omitempty"`
	ProcessedChunks int `json:"processedChunks,omitempty"`
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
