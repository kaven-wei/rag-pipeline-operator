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

// IndexJobSpec defines the desired state of IndexJob
type IndexJobSpec struct {
	DocumentSet string `json:"documentSet"`

	VectorDB VectorDBSpec `json:"vectorDB"`

	// TargetAlias is the alias to swap to after indexing
	TargetAlias string `json:"targetAlias,omitempty"`

	IndexSpec IndexConfig `json:"indexSpec"`

	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

type IndexConfig struct {
	Type       string            `json:"type"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

// IndexJobStatus defines the observed state of IndexJob
type IndexJobStatus struct {
	Phase          string             `json:"phase,omitempty"`
	Progress       IndexProgress      `json:"progress,omitempty"`
	Message        string             `json:"message,omitempty"`
	StartTime      *metav1.Time       `json:"startTime,omitempty"`
	CompletionTime *metav1.Time       `json:"completionTime,omitempty"`
	Conditions     []metav1.Condition `json:"conditions,omitempty"`
}

type IndexProgress struct {
	IndexedVectors int `json:"indexedVectors,omitempty"`
	TotalVectors   int `json:"totalVectors,omitempty"`
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
