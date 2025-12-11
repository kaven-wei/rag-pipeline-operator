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

package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ragv1alpha1 "rag.ai/rag-operator/api/v1alpha1"
)

const (
	documentSetFinalizer = "documentset.rag.ai/finalizer"
	requeueAfter         = 30 * time.Second
)

// DocumentSetReconciler reconciles a DocumentSet object
type DocumentSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=rag.ai,resources=documentsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rag.ai,resources=documentsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rag.ai,resources=documentsets/finalizers,verbs=update
// +kubebuilder:rbac:groups=rag.ai,resources=embeddingjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rag.ai,resources=indexjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DocumentSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling DocumentSet", "name", req.Name, "namespace", req.Namespace)

	// Fetch the DocumentSet instance
	documentSet := &ragv1alpha1.DocumentSet{}
	if err := r.Get(ctx, req.NamespacedName, documentSet); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("DocumentSet not found, might be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get DocumentSet")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !documentSet.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, documentSet)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(documentSet, documentSetFinalizer) {
		controllerutil.AddFinalizer(documentSet, documentSetFinalizer)
		if err := r.Update(ctx, documentSet); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if empty
	if documentSet.Status.Phase == "" {
		return r.initializeStatus(ctx, documentSet)
	}

	// Handle based on current phase
	switch documentSet.Status.Phase {
	case ragv1alpha1.DocumentSetPhasePending:
		return r.handlePendingPhase(ctx, documentSet)
	case ragv1alpha1.DocumentSetPhaseEmbedding:
		return r.handleEmbeddingPhase(ctx, documentSet)
	case ragv1alpha1.DocumentSetPhaseIndexing:
		return r.handleIndexingPhase(ctx, documentSet)
	case ragv1alpha1.DocumentSetPhaseReady:
		return r.handleReadyPhase(ctx, documentSet)
	case ragv1alpha1.DocumentSetPhaseFailed:
		return r.handleFailedPhase(ctx, documentSet)
	default:
		logger.Info("Unknown phase, resetting to Pending", "phase", documentSet.Status.Phase)
		return r.initializeStatus(ctx, documentSet)
	}
}

// initializeStatus sets the initial status of a DocumentSet
func (r *DocumentSetReconciler) initializeStatus(ctx context.Context, ds *ragv1alpha1.DocumentSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Initializing DocumentSet status")

	now := metav1.Now()
	ds.Status.Phase = ragv1alpha1.DocumentSetPhasePending
	ds.Status.Message = "DocumentSet created, waiting for processing"
	ds.Status.ObservedGeneration = ds.Generation
	ds.Status.LastUpdateTime = &now

	if err := r.Status().Update(ctx, ds); err != nil {
		logger.Error(err, "Failed to update DocumentSet status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

// handlePendingPhase creates an EmbeddingJob for the DocumentSet
func (r *DocumentSetReconciler) handlePendingPhase(ctx context.Context, ds *ragv1alpha1.DocumentSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling Pending phase - creating EmbeddingJob")

	// Generate collection name with timestamp
	timestamp := time.Now().Format("20060102150405")
	collectionName := fmt.Sprintf("%s_%s", ds.Spec.Index.Collection, timestamp)

	// Create EmbeddingJob
	embeddingJob := &ragv1alpha1.EmbeddingJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-embedding-%s", ds.Name, timestamp),
			Namespace: ds.Namespace,
			Labels: map[string]string{
				"rag.ai/documentset": ds.Name,
				"rag.ai/job-type":    "embedding",
			},
		},
		Spec: ragv1alpha1.EmbeddingJobSpec{
			DocumentSet:    ds.Name,
			EmbeddingModel: ds.Spec.Embedding.Model,
			VectorDB: ragv1alpha1.VectorDBSpec{
				Type:       ds.Spec.Index.VectorDB,
				Collection: collectionName,
			},
			RetryPolicy: &ragv1alpha1.RetryPolicy{
				MaxRetries:     3,
				BackoffSeconds: 30,
			},
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(ds, embeddingJob, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference")
		return ctrl.Result{}, err
	}

	// Create the EmbeddingJob
	if err := r.Create(ctx, embeddingJob); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("EmbeddingJob already exists")
		} else {
			logger.Error(err, "Failed to create EmbeddingJob")
			return r.updateStatusFailed(ctx, ds, fmt.Sprintf("Failed to create EmbeddingJob: %v", err))
		}
	}

	// Update DocumentSet status
	now := metav1.Now()
	ds.Status.Phase = ragv1alpha1.DocumentSetPhaseEmbedding
	ds.Status.Message = "EmbeddingJob created, processing documents"
	ds.Status.LastEmbeddingJobRef = embeddingJob.Name
	ds.Status.CurrentCollection = collectionName
	ds.Status.LastUpdateTime = &now

	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:               ragv1alpha1.ConditionTypeChunkingCompleted,
		Status:             metav1.ConditionFalse,
		Reason:             "EmbeddingJobCreated",
		Message:            "Waiting for embedding job to complete",
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// handleEmbeddingPhase monitors the EmbeddingJob and transitions to Indexing when complete
func (r *DocumentSetReconciler) handleEmbeddingPhase(ctx context.Context, ds *ragv1alpha1.DocumentSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling Embedding phase")

	// Get the EmbeddingJob
	embeddingJob := &ragv1alpha1.EmbeddingJob{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      ds.Status.LastEmbeddingJobRef,
		Namespace: ds.Namespace,
	}, embeddingJob); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("EmbeddingJob not found, transitioning back to Pending")
			return r.initializeStatus(ctx, ds)
		}
		return ctrl.Result{}, err
	}

	// Check EmbeddingJob status
	switch embeddingJob.Status.Phase {
	case ragv1alpha1.EmbeddingJobPhaseSucceeded:
		logger.Info("EmbeddingJob succeeded, creating IndexJob")
		return r.createIndexJob(ctx, ds)

	case ragv1alpha1.EmbeddingJobPhaseFailed:
		return r.updateStatusFailed(ctx, ds, fmt.Sprintf("EmbeddingJob failed: %s", embeddingJob.Status.Message))

	case ragv1alpha1.EmbeddingJobPhaseRunning:
		// Update progress in DocumentSet status
		now := metav1.Now()
		ds.Status.TotalChunks = embeddingJob.Status.Progress.TotalChunks
		ds.Status.Message = fmt.Sprintf("Embedding in progress: %d/%d chunks",
			embeddingJob.Status.Progress.ProcessedChunks,
			embeddingJob.Status.Progress.TotalChunks)
		ds.Status.LastUpdateTime = &now

		if err := r.Status().Update(ctx, ds); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueAfter}, nil

	default:
		// Still pending or unknown state
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
}

// createIndexJob creates an IndexJob after embedding completes
func (r *DocumentSetReconciler) createIndexJob(ctx context.Context, ds *ragv1alpha1.DocumentSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	timestamp := time.Now().Format("20060102150405")

	indexJob := &ragv1alpha1.IndexJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-index-%s", ds.Name, timestamp),
			Namespace: ds.Namespace,
			Labels: map[string]string{
				"rag.ai/documentset": ds.Name,
				"rag.ai/job-type":    "index",
			},
		},
		Spec: ragv1alpha1.IndexJobSpec{
			DocumentSet: ds.Name,
			VectorDB: ragv1alpha1.VectorDBSpec{
				Type:       ds.Spec.Index.VectorDB,
				Collection: ds.Status.CurrentCollection,
			},
			TargetAlias: ds.Spec.Index.Alias,
			IndexSpec: ragv1alpha1.IndexConfig{
				Type: ragv1alpha1.IndexTypeHNSW,
				Parameters: map[string]string{
					"efConstruction": "200",
					"M":              "16",
				},
			},
			RetryPolicy: &ragv1alpha1.RetryPolicy{
				MaxRetries:     3,
				BackoffSeconds: 30,
			},
		},
	}

	if err := controllerutil.SetControllerReference(ds, indexJob, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, indexJob); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("IndexJob already exists")
		} else {
			logger.Error(err, "Failed to create IndexJob")
			return r.updateStatusFailed(ctx, ds, fmt.Sprintf("Failed to create IndexJob: %v", err))
		}
	}

	// Update status
	now := metav1.Now()
	ds.Status.Phase = ragv1alpha1.DocumentSetPhaseIndexing
	ds.Status.Message = "IndexJob created, building vector index"
	ds.Status.LastIndexJobRef = indexJob.Name
	ds.Status.LastUpdateTime = &now

	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:               ragv1alpha1.ConditionTypeEmbeddingCompleted,
		Status:             metav1.ConditionTrue,
		Reason:             "EmbeddingJobSucceeded",
		Message:            "Embedding generation completed successfully",
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// handleIndexingPhase monitors the IndexJob and transitions to Ready when complete
func (r *DocumentSetReconciler) handleIndexingPhase(ctx context.Context, ds *ragv1alpha1.DocumentSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling Indexing phase")

	indexJob := &ragv1alpha1.IndexJob{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      ds.Status.LastIndexJobRef,
		Namespace: ds.Namespace,
	}, indexJob); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("IndexJob not found, creating new one")
			return r.createIndexJob(ctx, ds)
		}
		return ctrl.Result{}, err
	}

	switch indexJob.Status.Phase {
	case ragv1alpha1.IndexJobPhaseSucceeded:
		logger.Info("IndexJob succeeded, DocumentSet is Ready")
		return r.updateStatusReady(ctx, ds, indexJob)

	case ragv1alpha1.IndexJobPhaseFailed:
		return r.updateStatusFailed(ctx, ds, fmt.Sprintf("IndexJob failed: %s", indexJob.Status.Message))

	case ragv1alpha1.IndexJobPhaseBuilding, ragv1alpha1.IndexJobPhaseOptimizing:
		now := metav1.Now()
		ds.Status.TotalVectors = indexJob.Status.Progress.TotalVectors
		ds.Status.Message = fmt.Sprintf("Index building in progress: %d/%d vectors",
			indexJob.Status.Progress.IndexedVectors,
			indexJob.Status.Progress.TotalVectors)
		ds.Status.LastUpdateTime = &now

		if err := r.Status().Update(ctx, ds); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueAfter}, nil

	default:
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
}

// updateStatusReady marks the DocumentSet as Ready
func (r *DocumentSetReconciler) updateStatusReady(ctx context.Context, ds *ragv1alpha1.DocumentSet, indexJob *ragv1alpha1.IndexJob) (ctrl.Result, error) {
	now := metav1.Now()
	ds.Status.Phase = ragv1alpha1.DocumentSetPhaseReady
	ds.Status.Message = "DocumentSet is ready for queries"
	ds.Status.TotalVectors = indexJob.Status.Progress.TotalVectors
	ds.Status.LastUpdateTime = &now
	ds.Status.ObservedGeneration = ds.Generation

	meta.SetStatusCondition(&ds.Status.Conditions, metav1.Condition{
		Type:               ragv1alpha1.ConditionTypeIndexingCompleted,
		Status:             metav1.ConditionTrue,
		Reason:             "IndexJobSucceeded",
		Message:            "Index building completed successfully",
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, ds); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleReadyPhase checks if spec has changed and needs reprocessing
func (r *DocumentSetReconciler) handleReadyPhase(ctx context.Context, ds *ragv1alpha1.DocumentSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if generation changed (spec was updated)
	if ds.Generation != ds.Status.ObservedGeneration {
		logger.Info("DocumentSet spec changed, reprocessing", "generation", ds.Generation, "observedGeneration", ds.Status.ObservedGeneration)
		return r.initializeStatus(ctx, ds)
	}

	// No changes, stay in Ready state
	return ctrl.Result{}, nil
}

// handleFailedPhase handles retry logic for failed DocumentSets
func (r *DocumentSetReconciler) handleFailedPhase(ctx context.Context, ds *ragv1alpha1.DocumentSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if spec changed, which might fix the issue
	if ds.Generation != ds.Status.ObservedGeneration {
		logger.Info("DocumentSet spec changed after failure, retrying")
		return r.initializeStatus(ctx, ds)
	}

	// Check if enough time has passed for automatic retry (if autoRetry is enabled)
	if ds.Spec.Embedding.AutoRetry && ds.Status.LastUpdateTime != nil {
		elapsed := time.Since(ds.Status.LastUpdateTime.Time)
		if elapsed > 5*time.Minute {
			logger.Info("Auto-retrying failed DocumentSet after timeout")
			return r.initializeStatus(ctx, ds)
		}
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// updateStatusFailed updates the DocumentSet status to Failed
func (r *DocumentSetReconciler) updateStatusFailed(ctx context.Context, ds *ragv1alpha1.DocumentSet, message string) (ctrl.Result, error) {
	now := metav1.Now()
	ds.Status.Phase = ragv1alpha1.DocumentSetPhaseFailed
	ds.Status.Message = message
	ds.Status.LastUpdateTime = &now

	if err := r.Status().Update(ctx, ds); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// handleDeletion performs cleanup when a DocumentSet is deleted
func (r *DocumentSetReconciler) handleDeletion(ctx context.Context, ds *ragv1alpha1.DocumentSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling DocumentSet deletion")

	// Cleanup logic here (e.g., delete collections from vector DB)
	// Note: Child EmbeddingJobs and IndexJobs will be garbage collected via OwnerReferences

	// Remove finalizer
	controllerutil.RemoveFinalizer(ds, documentSetFinalizer)
	if err := r.Update(ctx, ds); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DocumentSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ragv1alpha1.DocumentSet{}).
		Owns(&ragv1alpha1.EmbeddingJob{}).
		Owns(&ragv1alpha1.IndexJob{}).
		Complete(r)
}
