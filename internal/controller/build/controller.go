// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openchoreov1alpha1 "github.com/openchoreo/openchoreo/api/v1alpha1"
	kubernetesClient "github.com/openchoreo/openchoreo/internal/clients/kubernetes"
	"github.com/openchoreo/openchoreo/internal/controller"
	"github.com/openchoreo/openchoreo/internal/controller/build/interfaces"
	buildservice "github.com/openchoreo/openchoreo/internal/controller/build/services/build"
)

// Reconciler reconciles a Build object
type Reconciler struct {
	k8sClientMgr *kubernetesClient.KubeMultiClientManager
	client.Client
	// IsGitOpsMode indicates whether the controller is running in GitOps mode
	IsGitOpsMode bool
	Scheme       *runtime.Scheme
	buildService *buildservice.Service
}

// +kubebuilder:rbac:groups=openchoreo.dev,resources=builds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openchoreo.dev,resources=builds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openchoreo.dev,resources=builds/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("build", req.NamespacedName)

	// Fetch the build resource
	build := &openchoreov1alpha1.Build{}
	if err := r.Get(ctx, req.NamespacedName, build); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Build resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Build")
		return ctrl.Result{}, err
	}

	oldBuild := build.DeepCopy()

	// Check if we should ignore reconciliation
	if shouldIgnoreReconcile(build) {
		return ctrl.Result{}, nil
	}

	// Set BuildInitiated condition if not already set
	if !isBuildInitiated(build) {
		setBuildInitiatedCondition(build)
		return r.updateStatusAndRequeue(ctx, oldBuild, build)
	}

	// Process the build using the service layer
	if !isBuildTriggered(build) {
		if err := r.buildService.ProcessBuild(ctx, build); err != nil {
			logger.Error(err, "Failed to process build")
			r.buildService.NotifyBuildFailed(ctx, build, err)
			return r.updateStatusAndReturn(ctx, oldBuild, build)
		}
		setBuildTriggeredCondition(build)
		return r.updateStatusAndRequeue(ctx, oldBuild, build)
	}

	// Check build status and update conditions
	if !isBuildWorkflowSucceeded(build) {
		return r.updateBuildStatusFromService(ctx, oldBuild, build, logger)
	}

	// Handle workload creation for successful builds
	if err := r.handleWorkloadUpdate(ctx, build, logger); err != nil {
		logger.Error(err, "Failed to handle workload update")
		meta.SetStatusCondition(&build.Status.Conditions, buildservice.NewWorkloadUpdateFailedCondition(build.Generation))
		return r.updateStatusAndRequeue(ctx, oldBuild, build)
	}

	meta.SetStatusCondition(&build.Status.Conditions, buildservice.NewWorkloadUpdatedCondition(build.Generation))
	return r.updateStatusAndReturn(ctx, oldBuild, build)
}

// updateBuildStatusFromService updates build status using the service layer
func (r *Reconciler) updateBuildStatusFromService(ctx context.Context, oldBuild, build *openchoreov1alpha1.Build, logger logr.Logger) (ctrl.Result, error) {
	buildStatus, err := r.buildService.GetBuildStatus(ctx, build)
	if err != nil {
		logger.Error(err, "Failed to get build status")
		return r.updateStatusAndRequeue(ctx, oldBuild, build)
	}

	switch buildStatus.Phase {
	case interfaces.BuildPhaseRunning:
		setBuildInProgressCondition(build)
		// Requeue after 20 seconds to check build status
		return r.updateStatusAndRequeueAfter(ctx, oldBuild, build, 20*time.Second)
	case interfaces.BuildPhaseSucceeded:
		artifacts, err := r.buildService.ExtractBuildArtifacts(ctx, build)
		if err != nil {
			logger.Error(err, "Failed to extract build artifacts")
			return r.updateStatusAndRequeue(ctx, oldBuild, build)
		}

		setBuildCompletedCondition(build, "Build completed successfully")
		if artifacts != nil && artifacts.Image != "" {
			build.Status.ImageStatus.Image = artifacts.Image
		}

		// Notify build completion
		r.buildService.NotifyBuildCompleted(ctx, build, artifacts)

		if err := r.Status().Update(ctx, build); err != nil {
			logger.Error(err, "Failed to update build status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	case interfaces.BuildPhaseFailed, interfaces.BuildPhaseError:
		setBuildFailedCondition(build, buildservice.ReasonBuildFailed, buildStatus.Message)
		r.buildService.NotifyBuildFailed(ctx, build, fmt.Errorf("build failed: %s", buildStatus.Message))
		return r.updateStatusAndReturn(ctx, oldBuild, build)
	default:
		// Build is pending or in unknown state, requeue
		return r.updateStatusAndRequeue(ctx, oldBuild, build)
	}
}

// handleWorkloadUpdate handles updating existing workloads with build artifacts
// Following OpenChoreo architecture: workloads are created by developers, builds update them
func (r *Reconciler) handleWorkloadUpdate(ctx context.Context, build *openchoreov1alpha1.Build, logger logr.Logger) error {
	artifacts, err := r.buildService.ExtractBuildArtifacts(ctx, build)
	if err != nil {
		return fmt.Errorf("failed to extract build artifacts: %w", err)
	}

	if artifacts == nil {
		logger.Info("No artifacts found from build")
		return nil
	}

	return r.buildService.UpdateWorkloadFromArtifacts(ctx, build, artifacts)
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.k8sClientMgr == nil {
		r.k8sClientMgr = kubernetesClient.NewManager()
	}

	// Initialize the build service if not already set
	if r.buildService == nil {
		r.buildService = buildservice.NewService(r.Client, r.k8sClientMgr)
	}

	ctx := context.Background()

	// Field index: spec.owner.projectName
	if err := mgr.GetFieldIndexer().IndexField(ctx, &openchoreov1alpha1.Workload{}, workloadProjectIndexKey,
		func(obj client.Object) []string {
			if wl, ok := obj.(*openchoreov1alpha1.Workload); ok {
				return []string{wl.Spec.Owner.ProjectName}
			}
			return nil
		}); err != nil {
		return fmt.Errorf("index owner.projectName: %w", err)
	}

	// Field index: spec.owner.componentName
	if err := mgr.GetFieldIndexer().IndexField(ctx, &openchoreov1alpha1.Workload{}, workloadComponentIndexKey,
		func(obj client.Object) []string {
			if wl, ok := obj.(*openchoreov1alpha1.Workload); ok {
				return []string{wl.Spec.Owner.ComponentName}
			}
			return nil
		}); err != nil {
		return fmt.Errorf("index owner.componentName: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&openchoreov1alpha1.Build{}).
		Named("build").
		Complete(r)
}

// Helper functions for build status checking
func shouldIgnoreReconcile(build *openchoreoov1alpha1.Build) bool {
	// Skip reconciliation if build is already completed (success or failure)
	if isBuildCompleted(build) {
		return true
	}
	return false
}

func isBuildInitiated(build *openchoreoov1alpha1.Build) bool {
	return meta.IsStatusConditionTrue(build.Status.Conditions, string(buildservice.ConditionBuildInitiated))
}

func isBuildTriggered(build *openchoreoov1alpha1.Build) bool {
	return meta.IsStatusConditionTrue(build.Status.Conditions, string(buildservice.ConditionBuildTriggered))
}

func isBuildWorkflowSucceeded(build *openchoreoov1alpha1.Build) bool {
	cond := meta.FindStatusCondition(build.Status.Conditions, string(buildservice.ConditionBuildCompleted))
	if cond == nil {
		return false
	}

	if cond.Reason == string(buildservice.ReasonBuildCompleted) {
		return cond.Status == metav1.ConditionTrue
	}
	return false
}

func isBuildCompleted(build *openchoreoov1alpha1.Build) bool {
	cond := meta.FindStatusCondition(build.Status.Conditions, string(buildservice.ConditionWorkloadUpdated))
	if cond == nil {
		return false
	}

	if cond.Reason == string(buildservice.ReasonWorkloadUpdated) {
		return cond.Status == metav1.ConditionTrue
	}

	return false
}

// Helper functions for setting conditions
func setBuildInitiatedCondition(build *openchoreoov1alpha1.Build) {
	meta.SetStatusCondition(&build.Status.Conditions, newBuildInitiatedCondition(build.Generation))
}

func setBuildTriggeredCondition(build *openchoreoov1alpha1.Build) {
	meta.SetStatusCondition(&build.Status.Conditions, newBuildTriggeredCondition(build.Generation))
}

func setBuildCompletedCondition(build *openchoreoov1alpha1.Build, message string) {
	condition := buildservice.NewBuildCompletedCondition(build.Generation)
	if message != "" {
		condition.Message = message
	}
	meta.SetStatusCondition(&build.Status.Conditions, condition)
}

func setBuildFailedCondition(build *openchoreoov1alpha1.Build, reason controller.ConditionReason, message string) {
	condition := buildservice.NewBuildFailedCondition(build.Generation)
	if reason != "" {
		condition.Reason = string(reason)
	}
	if message != "" {
		condition.Message = message
	}
	meta.SetStatusCondition(&build.Status.Conditions, condition)
}

func setBuildInProgressCondition(build *openchoreoov1alpha1.Build) {
	meta.SetStatusCondition(&build.Status.Conditions, buildservice.NewBuildInProgressCondition(build.Generation))
}

// Local condition creators for controller-specific conditions
func newBuildInitiatedCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(buildservice.ConditionBuildInitiated),
		Status:             metav1.ConditionTrue,
		Reason:             string(buildservice.ReasonBuildInitiated),
		Message:            "Build initialization started",
		ObservedGeneration: generation,
	}
}

func newBuildTriggeredCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(buildservice.ConditionBuildTriggered),
		Status:             metav1.ConditionTrue,
		Reason:             string(buildservice.ReasonBuildTriggered),
		Message:            "Build has been triggered",
		ObservedGeneration: generation,
	}
}

const (
	workloadProjectIndexKey   = "spec.owner.projectName"
	workloadComponentIndexKey = "spec.owner.componentName"
)

// Status update methods
func (r *Reconciler) updateStatusAndRequeue(ctx context.Context, oldBuild, build *openchoreov1alpha1.Build) (ctrl.Result, error) {
	return controller.UpdateStatusConditionsAndRequeue(ctx, r.Client, oldBuild, build)
}

func (r *Reconciler) updateStatusAndReturn(ctx context.Context, oldBuild, build *openchoreov1alpha1.Build) (ctrl.Result, error) {
	return controller.UpdateStatusConditionsAndReturn(ctx, r.Client, oldBuild, build)
}

func (r *Reconciler) updateStatusAndRequeueAfter(ctx context.Context, oldBuild, build *openchoreoov1alpha1.Build, duration time.Duration) (ctrl.Result, error) {
	return controller.UpdateStatusConditionsAndRequeueAfter(ctx, r.Client, oldBuild, build, duration)
}
