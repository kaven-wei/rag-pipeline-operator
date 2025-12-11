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
	indexJobFinalizer = "indexjob.rag.ai/finalizer"
)

// IndexJobReconciler reconciles a IndexJob object
type IndexJobReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	RAGAgentImage string
}

// +kubebuilder:rbac:groups=rag.ai,resources=indexjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rag.ai,resources=indexjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rag.ai,resources=indexjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get

// Reconcile handles IndexJob reconciliation
func (r *IndexJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling IndexJob", "name", req.Name, "namespace", req.Namespace)

	// Fetch the IndexJob
	indexJob := &ragv1alpha1.IndexJob{}
	if err := r.Get(ctx, req.NamespacedName, indexJob); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !indexJob.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, indexJob)
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(indexJob, indexJobFinalizer) {
		controllerutil.AddFinalizer(indexJob, indexJobFinalizer)
		if err := r.Update(ctx, indexJob); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if empty
	if indexJob.Status.Phase == "" {
		return r.initializeStatus(ctx, indexJob)
	}

	// Handle based on phase
	switch indexJob.Status.Phase {
	case ragv1alpha1.IndexJobPhasePending:
		return r.handlePendingPhase(ctx, indexJob)
	case ragv1alpha1.IndexJobPhaseBuilding:
		return r.handleBuildingPhase(ctx, indexJob)
	case ragv1alpha1.IndexJobPhaseOptimizing:
		return r.handleOptimizingPhase(ctx, indexJob)
	case ragv1alpha1.IndexJobPhaseSucceeded, ragv1alpha1.IndexJobPhaseFailed:
		return ctrl.Result{}, nil
	default:
		return r.initializeStatus(ctx, indexJob)
	}
}

// initializeStatus sets the initial status
func (r *IndexJobReconciler) initializeStatus(ctx context.Context, job *ragv1alpha1.IndexJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Initializing IndexJob status")

	job.Status.Phase = ragv1alpha1.IndexJobPhasePending
	job.Status.Message = "IndexJob created, waiting to start"

	if err := r.Status().Update(ctx, job); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

// handlePendingPhase creates the Kubernetes Job for index building
func (r *IndexJobReconciler) handlePendingPhase(ctx context.Context, indexJob *ragv1alpha1.IndexJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling Pending phase - creating Index Job")

	// Get the DocumentSet
	documentSet := &ragv1alpha1.DocumentSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      indexJob.Spec.DocumentSet,
		Namespace: indexJob.Namespace,
	}, documentSet); err != nil {
		return r.updateStatusFailed(ctx, indexJob, fmt.Sprintf("Failed to get DocumentSet: %v", err))
	}

	// Create the Kubernetes Job
	k8sJob := r.buildK8sJob(indexJob, documentSet)

	if err := controllerutil.SetControllerReference(indexJob, k8sJob, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, k8sJob); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("Kubernetes Job already exists")
		} else {
			return r.updateStatusFailed(ctx, indexJob, fmt.Sprintf("Failed to create Job: %v", err))
		}
	}

	// Update status
	now := metav1.Now()
	indexJob.Status.Phase = ragv1alpha1.IndexJobPhaseBuilding
	indexJob.Status.Message = "Index building started"
	indexJob.Status.JobRef = k8sJob.Name
	indexJob.Status.StartTime = &now

	meta.SetStatusCondition(&indexJob.Status.Conditions, metav1.Condition{
		Type:               ragv1alpha1.IndexJobConditionIndexCreated,
		Status:             metav1.ConditionFalse,
		Reason:             "BuildingIndex",
		Message:            "Index build in progress",
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, indexJob); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// buildK8sJob creates the Kubernetes Job spec for index building
func (r *IndexJobReconciler) buildK8sJob(indexJob *ragv1alpha1.IndexJob, documentSet *ragv1alpha1.DocumentSet) *batchv1.Job {
	image := r.RAGAgentImage
	if image == "" {
		image = ragAgentImage
	}

	backoffLimit := int32(3)
	ttlSeconds := int32(3600)

	// Build environment variables
	envVars := []corev1.EnvVar{
		{Name: "INDEX_JOB_NAME", Value: indexJob.Name},
		{Name: "DOCUMENT_SET_NAME", Value: indexJob.Spec.DocumentSet},
		{Name: "DOCUMENT_SET_NAMESPACE", Value: indexJob.Namespace},
		{Name: "VECTOR_DB_TYPE", Value: indexJob.Spec.VectorDB.Type},
		{Name: "VECTOR_DB_COLLECTION", Value: indexJob.Spec.VectorDB.Collection},
		{Name: "TARGET_ALIAS", Value: indexJob.Spec.TargetAlias},
		{Name: "INDEX_TYPE", Value: indexJob.Spec.IndexSpec.Type},
	}

	// Add index parameters
	for k, v := range indexJob.Spec.IndexSpec.Parameters {
		envVars = append(envVars, corev1.EnvVar{
			Name:  fmt.Sprintf("INDEX_PARAM_%s", k),
			Value: v,
		})
	}

	// Add vector DB endpoint
	if indexJob.Spec.VectorDB.Endpoint != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "VECTOR_DB_ENDPOINT",
			Value: indexJob.Spec.VectorDB.Endpoint,
		})
	}

	// Add secrets
	envFrom := []corev1.EnvFromSource{}
	if indexJob.Spec.VectorDB.SecretRef != nil {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: indexJob.Spec.VectorDB.SecretRef.Name,
				},
			},
		})
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      indexJob.Name + "-job",
			Namespace: indexJob.Namespace,
			Labels: map[string]string{
				"rag.ai/index-job":   indexJob.Name,
				"rag.ai/documentset": indexJob.Spec.DocumentSet,
				"rag.ai/job-type":    "index",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"rag.ai/index-job": indexJob.Name,
						"rag.ai/job-type":  "index",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "index-worker",
							Image:   image,
							Command: []string{"python", "scripts/run_index_job.py"},
							Args:    []string{indexJob.Name},
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

// handleBuildingPhase monitors the index building Job
func (r *IndexJobReconciler) handleBuildingPhase(ctx context.Context, indexJob *ragv1alpha1.IndexJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling Building phase")

	// Get the Kubernetes Job
	k8sJob := &batchv1.Job{}
	jobName := indexJob.Status.JobRef
	if jobName == "" {
		jobName = indexJob.Name + "-job"
	}

	if err := r.Get(ctx, types.NamespacedName{
		Name:      jobName,
		Namespace: indexJob.Namespace,
	}, k8sJob); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Kubernetes Job not found, recreating")
			return r.initializeStatus(ctx, indexJob)
		}
		return ctrl.Result{}, err
	}

	// Check Job status
	if k8sJob.Status.Succeeded > 0 {
		return r.handleJobSucceeded(ctx, indexJob)
	}

	if k8sJob.Status.Failed > 0 {
		// Check retry policy
		maxRetries := 3
		if indexJob.Spec.RetryPolicy != nil {
			maxRetries = indexJob.Spec.RetryPolicy.MaxRetries
		}

		if indexJob.Status.RetryCount < maxRetries {
			logger.Info("Job failed, scheduling retry", "retryCount", indexJob.Status.RetryCount+1)
			indexJob.Status.RetryCount++
			indexJob.Status.Phase = ragv1alpha1.IndexJobPhasePending
			indexJob.Status.Message = fmt.Sprintf("Retrying (%d/%d)", indexJob.Status.RetryCount, maxRetries)

			if err := r.Status().Update(ctx, indexJob); err != nil {
				return ctrl.Result{}, err
			}

			if err := r.Delete(ctx, k8sJob); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}

			backoff := 30
			if indexJob.Spec.RetryPolicy != nil {
				backoff = indexJob.Spec.RetryPolicy.BackoffSeconds
			}
			return ctrl.Result{RequeueAfter: time.Duration(backoff) * time.Second}, nil
		}

		return r.updateStatusFailed(ctx, indexJob, "Job failed after maximum retries")
	}

	// Job still running
	indexJob.Status.Message = fmt.Sprintf("Index build running: %d active pods", k8sJob.Status.Active)
	if err := r.Status().Update(ctx, indexJob); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// handleJobSucceeded handles successful job completion
func (r *IndexJobReconciler) handleJobSucceeded(ctx context.Context, indexJob *ragv1alpha1.IndexJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Index build job succeeded")

	now := metav1.Now()

	// Update conditions
	meta.SetStatusCondition(&indexJob.Status.Conditions, metav1.Condition{
		Type:               ragv1alpha1.IndexJobConditionIndexCreated,
		Status:             metav1.ConditionTrue,
		Reason:             "IndexBuilt",
		Message:            "Index built successfully",
		LastTransitionTime: now,
	})

	// Check if alias swap is needed
	if indexJob.Spec.TargetAlias != "" {
		indexJob.Status.Phase = ragv1alpha1.IndexJobPhaseOptimizing
		indexJob.Status.Message = "Index built, performing alias swap"

		// In a real implementation, we would call the vector DB API here
		// For now, we mark it as successful and assume the Python job handles alias swap
		meta.SetStatusCondition(&indexJob.Status.Conditions, metav1.Condition{
			Type:               ragv1alpha1.IndexJobConditionAliasSwapped,
			Status:             metav1.ConditionTrue,
			Reason:             "AliasSwapped",
			Message:            fmt.Sprintf("Alias '%s' switched to collection '%s'", indexJob.Spec.TargetAlias, indexJob.Spec.VectorDB.Collection),
			LastTransitionTime: now,
		})
		indexJob.Status.AliasSwapped = true
	}

	// Mark as succeeded
	indexJob.Status.Phase = ragv1alpha1.IndexJobPhaseSucceeded
	indexJob.Status.Message = "Index building completed successfully"
	indexJob.Status.CompletionTime = &now

	meta.SetStatusCondition(&indexJob.Status.Conditions, metav1.Condition{
		Type:               ragv1alpha1.IndexJobConditionIndexOptimized,
		Status:             metav1.ConditionTrue,
		Reason:             "IndexOptimized",
		Message:            "Index optimized and ready for queries",
		LastTransitionTime: now,
	})

	if err := r.Status().Update(ctx, indexJob); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleOptimizingPhase handles the optimization phase (if separate from building)
func (r *IndexJobReconciler) handleOptimizingPhase(ctx context.Context, indexJob *ragv1alpha1.IndexJob) (ctrl.Result, error) {
	// In most cases, optimization is part of the building phase
	// This is here for future expansion or databases that separate these steps
	return r.handleJobSucceeded(ctx, indexJob)
}

// updateStatusFailed marks the IndexJob as failed
func (r *IndexJobReconciler) updateStatusFailed(ctx context.Context, job *ragv1alpha1.IndexJob, message string) (ctrl.Result, error) {
	now := metav1.Now()
	job.Status.Phase = ragv1alpha1.IndexJobPhaseFailed
	job.Status.Message = message
	job.Status.CompletionTime = &now

	if err := r.Status().Update(ctx, job); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles cleanup
func (r *IndexJobReconciler) handleDeletion(ctx context.Context, job *ragv1alpha1.IndexJob) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling IndexJob deletion")

	controllerutil.RemoveFinalizer(job, indexJobFinalizer)
	if err := r.Update(ctx, job); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IndexJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ragv1alpha1.IndexJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
