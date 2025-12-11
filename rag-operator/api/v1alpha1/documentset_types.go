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

// DocumentSet Phase constants
const (
	DocumentSetPhasePending   = "Pending"
	DocumentSetPhaseEmbedding = "Embedding"
	DocumentSetPhaseIndexing  = "Indexing"
	DocumentSetPhaseReady     = "Ready"
	DocumentSetPhaseFailed    = "Failed"
)

// DocumentSet Condition types
const (
	ConditionTypeChunkingCompleted  = "ChunkingCompleted"
	ConditionTypeEmbeddingCompleted = "EmbeddingCompleted"
	ConditionTypeIndexingCompleted  = "IndexingCompleted"
)

// Source types
const (
	SourceTypeS3   = "s3"
	SourceTypeHTTP = "http"
	SourceTypeGit  = "git"
	SourceTypePVC  = "pvc"
)

// Vector database types
const (
	VectorDBMilvus   = "milvus"
	VectorDBQdrant   = "qdrant"
	VectorDBWeaviate = "weaviate"
)

// DocumentSetSpec defines the desired state of DocumentSet
type DocumentSetSpec struct {
	// Source defines where the documents are located
	// +kubebuilder:validation:Required
	Source SourceSpec `json:"source"`

	// Chunking defines how to split the documents
	// +kubebuilder:validation:Required
	Chunking ChunkingSpec `json:"chunking"`

	// Embedding defines the model and parameters for embedding
	// +kubebuilder:validation:Required
	Embedding EmbeddingSpec `json:"embedding"`

	// Index defines the vector database and collection settings
	// +kubebuilder:validation:Required
	Index IndexSpec `json:"index"`
}

type SourceSpec struct {
	// Type of source: s3, http, git, pvc
	// +kubebuilder:validation:Enum=s3;http;git;pvc
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// URI to the source
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	URI string `json:"uri"`

	// SecretRef for authentication
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

type SecretReference struct {
	// Name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type ChunkingSpec struct {
	// Size of each chunk in characters
	// +kubebuilder:validation:Minimum=100
	// +kubebuilder:default=512
	Size int `json:"size"`

	// Overlap between chunks in characters
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=100
	Overlap int `json:"overlap"`

	// Format of the documents: text, markdown, html
	// +kubebuilder:validation:Enum=text;markdown;html
	// +kubebuilder:default=text
	// +optional
	Format string `json:"format,omitempty"`
}

type EmbeddingSpec struct {
	// Model name for embedding generation
	// +kubebuilder:validation:Required
	Model string `json:"model"`

	// Device to run embedding on: cpu, gpu
	// +kubebuilder:validation:Enum=cpu;gpu
	// +kubebuilder:default=cpu
	// +optional
	Device string `json:"device,omitempty"`

	// BatchSize for embedding generation
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=16
	// +optional
	BatchSize int `json:"batchSize,omitempty"`

	// AutoRetry enables automatic retry on failure
	// +kubebuilder:default=true
	// +optional
	AutoRetry bool `json:"autoRetry,omitempty"`
}

type IndexSpec struct {
	// VectorDB type: milvus, qdrant, weaviate
	// +kubebuilder:validation:Enum=milvus;qdrant;weaviate
	// +kubebuilder:validation:Required
	VectorDB string `json:"vectorDB"`

	// Collection name in the vector database
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Collection string `json:"collection"`

	// Alias for the collection (used for zero-downtime updates)
	// +optional
	Alias string `json:"alias,omitempty"`

	// Recreate forces recreation of the collection
	// +kubebuilder:default=false
	// +optional
	Recreate bool `json:"recreate,omitempty"`
}

// DocumentSetStatus defines the observed state of DocumentSet
type DocumentSetStatus struct {
	// Phase represents the current stage of the pipeline: Pending, Embedding, Indexing, Ready, Failed
	// +kubebuilder:validation:Enum=Pending;Embedding;Indexing;Ready;Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides details about the current status
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions tracks detailed status
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastEmbeddingJobRef is the name of the last created EmbeddingJob
	// +optional
	LastEmbeddingJobRef string `json:"lastEmbeddingJobRef,omitempty"`

	// LastIndexJobRef is the name of the last created IndexJob
	// +optional
	LastIndexJobRef string `json:"lastIndexJobRef,omitempty"`

	// CurrentCollection is the active collection name with timestamp
	// +optional
	CurrentCollection string `json:"currentCollection,omitempty"`

	// TotalChunks is the number of chunks processed
	// +optional
	TotalChunks int `json:"totalChunks,omitempty"`

	// TotalVectors is the number of vectors indexed
	// +optional
	TotalVectors int `json:"totalVectors,omitempty"`

	// LastUpdateTime is when the status was last updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ObservedGeneration is the last generation observed by the controller
	// +optional
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
