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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
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
	embeddingJobFinalizer = "embeddingjob.rag.ai/finalizer"
	ragAgentImage         = "rag-agent:latest"
)

// EmbeddingJobReconciler reconciles a EmbeddingJob object
type EmbeddingJobReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	RAGAgentImage string // Configurable image for the RAG agent
}

// +kubebuilder:rbac:groups=rag.ai,resources=embeddingjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rag.ai,resources=embeddingjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rag.ai,resources=embeddingjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop
func (r *EmbeddingJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling EmbeddingJob", "name", req.Name, "namespace", req.Namespace)

	// Fetch the EmbeddingJob
	embeddingJob := &ragv1alpha1.EmbeddingJob{}
	if err := r.Get(ctx, req.NamespacedName, embeddingJob); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !embeddingJob.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, embeddingJob)
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(embeddingJob, embeddingJobFinalizer) {
		controllerutil.AddFinalizer(embeddingJob, embeddingJobFinalizer)
		if err := r.Update(ctx, embeddingJob); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if empty
	if embeddingJob.Status.Phase == "" {
		return r.initializeStatus(ctx, embeddingJob)
	}

	// Handle based on phase
	switch embeddingJob.Status.Phase {
	case ragv1alpha1.EmbeddingJobPhasePending:
		return r.handlePendingPhase(ctx, embeddingJob)
	case ragv1alpha1.EmbeddingJobPhaseRunning:
		return r.handleRunningPhase(ctx, embeddingJob)
	case ragv1alpha1.EmbeddingJobPhaseSucceeded, ragv1alpha1.EmbeddingJobPhaseFailed:
		// Terminal states, nothing to do
		return ctrl.Result{}, nil
	default:
		return r.initializeStatus(ctx, embeddingJob)
	}
}

// initializeStatus sets the initial status
func (r *EmbeddingJobReconciler) initializeStatus(ctx context.Context, job *ragv1alpha1.EmbeddingJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Initializing EmbeddingJob status")

	job.Status.Phase = ragv1alpha1.EmbeddingJobPhasePending
	job.Status.Message = "EmbeddingJob created, waiting to start"

	if err := r.Status().Update(ctx, job); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

// handlePendingPhase creates the Kubernetes Job
func (r *EmbeddingJobReconciler) handlePendingPhase(ctx context.Context, embeddingJob *ragv1alpha1.EmbeddingJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling Pending phase - creating Kubernetes Job")

	// Get the DocumentSet to retrieve source configuration
	documentSet := &ragv1alpha1.DocumentSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      embeddingJob.Spec.DocumentSet,
		Namespace: embeddingJob.Namespace,
	}, documentSet); err != nil {
		return r.updateStatusFailed(ctx, embeddingJob, fmt.Sprintf("Failed to get DocumentSet: %v", err))
	}

	// Create the Kubernetes Job
	k8sJob := r.buildK8sJob(embeddingJob, documentSet)

	if err := controllerutil.SetControllerReference(embeddingJob, k8sJob, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, k8sJob); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("Kubernetes Job already exists")
		} else {
			return r.updateStatusFailed(ctx, embeddingJob, fmt.Sprintf("Failed to create Job: %v", err))
		}
	}

	// Update status to Running
	now := metav1.Now()
	embeddingJob.Status.Phase = ragv1alpha1.EmbeddingJobPhaseRunning
	embeddingJob.Status.Message = "Kubernetes Job created, processing embeddings"
	embeddingJob.Status.JobRef = k8sJob.Name
	embeddingJob.Status.StartTime = &now

	meta.SetStatusCondition(&embeddingJob.Status.Conditions, metav1.Condition{
		Type:               ragv1alpha1.EmbeddingJobConditionStarted,
		Status:             metav1.ConditionTrue,
		Reason:             "JobCreated",
		Message:            "Kubernetes Job created successfully",
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, embeddingJob); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// buildK8sJob creates the Kubernetes Job spec for embedding generation
func (r *EmbeddingJobReconciler) buildK8sJob(embeddingJob *ragv1alpha1.EmbeddingJob, documentSet *ragv1alpha1.DocumentSet) *batchv1.Job {
	image := r.RAGAgentImage
	if image == "" {
		image = ragAgentImage
	}

	backoffLimit := int32(3)
	ttlSeconds := int32(3600) // 1 hour TTL after completion

	// Build environment variables
	envVars := []corev1.EnvVar{
		{Name: "DOCUMENT_SET_NAME", Value: embeddingJob.Spec.DocumentSet},
		{Name: "DOCUMENT_SET_NAMESPACE", Value: embeddingJob.Namespace},
		{Name: "EMBEDDING_MODEL", Value: embeddingJob.Spec.EmbeddingModel},
		{Name: "VECTOR_DB_TYPE", Value: embeddingJob.Spec.VectorDB.Type},
		{Name: "VECTOR_DB_COLLECTION", Value: embeddingJob.Spec.VectorDB.Collection},
		{Name: "SOURCE_TYPE", Value: documentSet.Spec.Source.Type},
		{Name: "SOURCE_URI", Value: documentSet.Spec.Source.URI},
		{Name: "CHUNK_SIZE", Value: fmt.Sprintf("%d", documentSet.Spec.Chunking.Size)},
		{Name: "CHUNK_OVERLAP", Value: fmt.Sprintf("%d", documentSet.Spec.Chunking.Overlap)},
		{Name: "BATCH_SIZE", Value: fmt.Sprintf("%d", documentSet.Spec.Embedding.BatchSize)},
	}

	// Add vector DB endpoint if specified
	if embeddingJob.Spec.VectorDB.Endpoint != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "VECTOR_DB_ENDPOINT",
			Value: embeddingJob.Spec.VectorDB.Endpoint,
		})
	}

	// Add secrets as environment variables
	envFrom := []corev1.EnvFromSource{}
	if documentSet.Spec.Source.SecretRef != nil {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: documentSet.Spec.Source.SecretRef.Name,
				},
			},
		})
	}

	if embeddingJob.Spec.VectorDB.SecretRef != nil {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: embeddingJob.Spec.VectorDB.SecretRef.Name,
				},
			},
		})
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      embeddingJob.Name + "-job",
			Namespace: embeddingJob.Namespace,
			Labels: map[string]string{
				"rag.ai/embedding-job": embeddingJob.Name,
				"rag.ai/documentset":   embeddingJob.Spec.DocumentSet,
				"rag.ai/job-type":      "embedding",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"rag.ai/embedding-job": embeddingJob.Name,
						"rag.ai/job-type":      "embedding",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "embedding-worker",
							Image:   image,
							Command: []string{"python", "scripts/run_embedding_job.py"},
							Args:    []string{embeddingJob.Spec.DocumentSet},
							Env:     envVars,
							EnvFrom: envFrom,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("4Gi"),
								},
							},
						},
					},
				},
			},
		},
	}
}

// handleRunningPhase monitors the Kubernetes Job
func (r *EmbeddingJobReconciler) handleRunningPhase(ctx context.Context, embeddingJob *ragv1alpha1.EmbeddingJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling Running phase")

	// Get the Kubernetes Job
	k8sJob := &batchv1.Job{}
	jobName := embeddingJob.Status.JobRef
	if jobName == "" {
		jobName = embeddingJob.Name + "-job"
	}

	if err := r.Get(ctx, types.NamespacedName{
		Name:      jobName,
		Namespace: embeddingJob.Namespace,
	}, k8sJob); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Kubernetes Job not found, recreating")
			return r.initializeStatus(ctx, embeddingJob)
		}
		return ctrl.Result{}, err
	}

	// Check Job status
	if k8sJob.Status.Succeeded > 0 {
		return r.updateStatusSucceeded(ctx, embeddingJob)
	}

	if k8sJob.Status.Failed > 0 {
		// Check if we should retry
		maxRetries := 3
		if embeddingJob.Spec.RetryPolicy != nil {
			maxRetries = embeddingJob.Spec.RetryPolicy.MaxRetries
		}

		if embeddingJob.Status.RetryCount < maxRetries {
			logger.Info("Job failed, scheduling retry", "retryCount", embeddingJob.Status.RetryCount+1)
			embeddingJob.Status.RetryCount++
			embeddingJob.Status.Phase = ragv1alpha1.EmbeddingJobPhasePending
			embeddingJob.Status.Message = fmt.Sprintf("Retrying (%d/%d)", embeddingJob.Status.RetryCount, maxRetries)

			if err := r.Status().Update(ctx, embeddingJob); err != nil {
				return ctrl.Result{}, err
			}

			// Delete the failed job
			if err := r.Delete(ctx, k8sJob); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: time.Duration(embeddingJob.Spec.RetryPolicy.BackoffSeconds) * time.Second}, nil
		}

		return r.updateStatusFailed(ctx, embeddingJob, "Job failed after maximum retries")
	}

	// Job is still running
	embeddingJob.Status.Message = fmt.Sprintf("Job running: %d active pods", k8sJob.Status.Active)
	if err := r.Status().Update(ctx, embeddingJob); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// updateStatusSucceeded marks the EmbeddingJob as succeeded
func (r *EmbeddingJobReconciler) updateStatusSucceeded(ctx context.Context, job *ragv1alpha1.EmbeddingJob) (ctrl.Result, error) {
	now := metav1.Now()
	job.Status.Phase = ragv1alpha1.EmbeddingJobPhaseSucceeded
	job.Status.Message = "Embedding generation completed successfully"
	job.Status.CompletionTime = &now

	meta.SetStatusCondition(&job.Status.Conditions, metav1.Condition{
		Type:               ragv1alpha1.EmbeddingJobConditionVectorUpserted,
		Status:             metav1.ConditionTrue,
		Reason:             "JobSucceeded",
		Message:            "All vectors upserted to vector database",
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, job); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// updateStatusFailed marks the EmbeddingJob as failed
func (r *EmbeddingJobReconciler) updateStatusFailed(ctx context.Context, job *ragv1alpha1.EmbeddingJob, message string) (ctrl.Result, error) {
	now := metav1.Now()
	job.Status.Phase = ragv1alpha1.EmbeddingJobPhaseFailed
	job.Status.Message = message
	job.Status.CompletionTime = &now

	if err := r.Status().Update(ctx, job); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles cleanup
func (r *EmbeddingJobReconciler) handleDeletion(ctx context.Context, job *ragv1alpha1.EmbeddingJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling EmbeddingJob deletion")

	// Kubernetes Jobs are cleaned up via OwnerReferences
	controllerutil.RemoveFinalizer(job, embeddingJobFinalizer)
	if err := r.Update(ctx, job); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EmbeddingJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ragv1alpha1.EmbeddingJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
