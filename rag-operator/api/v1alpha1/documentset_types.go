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

// DocumentSetSpec defines the desired state of DocumentSet
type DocumentSetSpec struct {
	// Source defines where the documents are located
	Source SourceSpec `json:"source,omitempty"`

	// Chunking defines how to split the documents
	Chunking ChunkingSpec `json:"chunking,omitempty"`

	// Embedding defines the model and parameters for embedding
	Embedding EmbeddingSpec `json:"embedding,omitempty"`

	// Index defines the vector database and collection settings
	Index IndexSpec `json:"index,omitempty"`
}

type SourceSpec struct {
	// Type of source: s3, http, git, pvc
	Type string `json:"type"`
	// URI to the source
	URI string `json:"uri"`
	// SecretRef for authentication
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

type SecretReference struct {
	Name string `json:"name"`
}

type ChunkingSpec struct {
	Size    int    `json:"size"`
	Overlap int    `json:"overlap"`
	Format  string `json:"format,omitempty"` // text, markdown, html
}

type EmbeddingSpec struct {
	Model     string `json:"model"`
	Device    string `json:"device,omitempty"` // cpu, gpu
	BatchSize int    `json:"batchSize,omitempty"`
	AutoRetry bool   `json:"autoRetry,omitempty"`
}

type IndexSpec struct {
	VectorDB   string `json:"vectorDB"`
	Collection string `json:"collection"`
	Alias      string `json:"alias,omitempty"`
	Recreate   bool   `json:"recreate,omitempty"`
}

// DocumentSetStatus defines the observed state of DocumentSet
type DocumentSetStatus struct {
	// Phase represents the current stage of the pipeline
	Phase string `json:"phase,omitempty"`
	// Message provides details about the current status
	Message string `json:"message,omitempty"`
	// Conditions tracks detailed status
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// References to child jobs
	LastEmbeddingJobRef string `json:"lastEmbeddingJobRef,omitempty"`
	LastIndexJobRef     string `json:"lastIndexJobRef,omitempty"`
	// ObservedGeneration is the last generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// DocumentSet is the Schema for the documentsets API
type DocumentSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DocumentSetSpec   `json:"spec,omitempty"`
	Status DocumentSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DocumentSetList contains a list of DocumentSet
type DocumentSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DocumentSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DocumentSet{}, &DocumentSetList{})
}
