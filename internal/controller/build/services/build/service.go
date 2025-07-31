// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package build

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openchoreov1alpha1 "github.com/openchoreo/openchoreo/api/v1alpha1"
	kubernetesClient "github.com/openchoreo/openchoreo/internal/clients/kubernetes"
	"github.com/openchoreo/openchoreo/internal/controller"
	"github.com/openchoreo/openchoreo/internal/controller/build/engines/argo"
	"github.com/openchoreo/openchoreo/internal/controller/build/interfaces"
	"github.com/openchoreo/openchoreo/internal/controller/build/services/artifact"
	"github.com/openchoreo/openchoreo/internal/controller/build/services/notification"
	"github.com/openchoreo/openchoreo/internal/controller/build/services/workload"
)

// Service handles the core business logic for build operations
type Service struct {
	client              client.Client
	k8sClientMgr        *kubernetesClient.KubeMultiClientManager
	logger              logr.Logger
	buildEngines        map[string]interfaces.BuildEngine
	artifactService     *artifact.Service
	workloadService     *workload.Service
	notificationService *notification.Service
}

// NewService creates a new build service with all dependent services
func NewService(client client.Client, k8sClientMgr *kubernetesClient.KubeMultiClientManager) *Service {
	service := &Service{
		client:              client,
		k8sClientMgr:        k8sClientMgr,
		logger:              log.Log.WithName("build-service"),
		buildEngines:        make(map[string]interfaces.BuildEngine),
		artifactService:     artifact.NewService(client),
		workloadService:     workload.NewService(client),
		notificationService: notification.NewService(client),
	}

	// Register available build engines
	service.registerBuildEngines()

	return service
}

// registerBuildEngines registers all available build engines
func (s *Service) registerBuildEngines() {
	// Register Argo engine
	argoEngine := argo.NewEngine()
	s.buildEngines[argoEngine.GetName()] = argoEngine

	// Future engines can be registered here:
	// tektonEngine := tekton.NewEngine()
	// s.buildEngines[tektonEngine.GetName()] = tektonEngine
}

// ProcessBuild handles the main build processing logic
func (s *Service) ProcessBuild(ctx context.Context, build *openchoreov1alpha1.Build) error {
	logger := s.logger.WithValues("build", build.Name)

	// Send build started notification
	if err := s.notificationService.NotifyBuildStarted(ctx, build); err != nil {
		logger.Error(err, "Failed to send build started notification")
		// Don't fail the build for notification errors
	}

	// Get build plane and client
	buildPlane, bpClient, err := s.getBuildPlaneClient(ctx, build)
	if err != nil {
		return fmt.Errorf("failed to get build plane client: %w", err)
	}

	// Determine build engine (default to argo for now)
	engineName := s.determineBuildEngine(build)
	buildEngine, exists := s.buildEngines[engineName]
	if !exists {
		return fmt.Errorf("unsupported build engine: %s", engineName)
	}

	logger.Info("Using build engine", "engine", engineName)

	// Ensure prerequisites
	if err := buildEngine.EnsurePrerequisites(ctx, bpClient, build); err != nil {
		s.notificationService.NotifyBuildFailed(ctx, build, err)
		return fmt.Errorf("failed to ensure prerequisites: %w", err)
	}

	// Create or get existing build
	buildInfo, err := buildEngine.CreateBuild(ctx, bpClient, build)
	if err != nil {
		s.notificationService.NotifyBuildFailed(ctx, build, err)
		return fmt.Errorf("failed to create build: %w", err)
	}

	logger.Info("Build processed", "buildID", buildInfo.ID, "created", buildInfo.Created)
	return nil
}

// GetBuildStatus returns the current status of a build
func (s *Service) GetBuildStatus(ctx context.Context, build *openchoreov1alpha1.Build) (interfaces.BuildStatus, error) {
	// Get build plane client
	_, bpClient, err := s.getBuildPlaneClient(ctx, build)
	if err != nil {
		return interfaces.BuildStatus{}, fmt.Errorf("failed to get build plane client: %w", err)
	}

	// Determine build engine
	engineName := s.determineBuildEngine(build)
	buildEngine, exists := s.buildEngines[engineName]
	if !exists {
		return interfaces.BuildStatus{}, fmt.Errorf("unsupported build engine: %s", engineName)
	}

	return buildEngine.GetBuildStatus(ctx, bpClient, build)
}

// ExtractBuildArtifacts extracts artifacts from a completed build
func (s *Service) ExtractBuildArtifacts(ctx context.Context, build *openchoreov1alpha1.Build) (*interfaces.BuildArtifacts, error) {
	// Get build plane client
	_, bpClient, err := s.getBuildPlaneClient(ctx, build)
	if err != nil {
		return nil, fmt.Errorf("failed to get build plane client: %w", err)
	}

	// Determine build engine
	engineName := s.determineBuildEngine(build)
	buildEngine, exists := s.buildEngines[engineName]
	if !exists {
		return nil, fmt.Errorf("unsupported build engine: %s", engineName)
	}

	artifacts, err := buildEngine.ExtractBuildArtifacts(ctx, bpClient, build)
	if err != nil {
		return nil, err
	}

	// Validate artifacts using artifact service
	if err := s.artifactService.ValidateArtifacts(ctx, artifacts); err != nil {
		return nil, fmt.Errorf("artifact validation failed: %w", err)
	}

	// Store artifacts for future reference
	if err := s.artifactService.StoreArtifacts(ctx, build, artifacts); err != nil {
		s.logger.Error(err, "Failed to store artifacts", "build", build.Name)
		// Don't fail the build for storage errors
	}

	return artifacts, nil
}

// UpdateWorkloadFromArtifacts updates an existing workload with new build artifacts
// This follows OpenChoreo architecture where workloads are created by developers, not builds
func (s *Service) UpdateWorkloadFromArtifacts(ctx context.Context, build *openchoreov1alpha1.Build, artifacts *interfaces.BuildArtifacts) error {
	err := s.workloadService.UpdateWorkloadWithBuildArtifacts(ctx, build, artifacts)
	if err != nil {
		return err
	}

	// Send workload updated notification (not created)
	if artifacts != nil && artifacts.Image != "" {
		workloadName := fmt.Sprintf("%s-workload", build.Spec.Owner.ComponentName)
		if err := s.notificationService.NotifyWorkloadUpdated(ctx, build, workloadName); err != nil {
			s.logger.Error(err, "Failed to send workload updated notification")
		}
	}

	return nil
}

// NotifyBuildCompleted sends build completion notification
func (s *Service) NotifyBuildCompleted(ctx context.Context, build *openchoreov1alpha1.Build, artifacts *interfaces.BuildArtifacts) error {
	return s.notificationService.NotifyBuildCompleted(ctx, build, artifacts)
}

// NotifyBuildFailed sends build failure notification
func (s *Service) NotifyBuildFailed(ctx context.Context, build *openchoreov1alpha1.Build, err error) error {
	return s.notificationService.NotifyBuildFailed(ctx, build, err)
}

// UpdateBuildStatusConditions updates build status based on current build status
func (s *Service) UpdateBuildStatusConditions(build *openchoreov1alpha1.Build, status interfaces.BuildStatus, artifacts *interfaces.BuildArtifacts) {
	switch status.Phase {
	case interfaces.BuildPhaseRunning:
		s.setBuildInProgressCondition(build)
	case interfaces.BuildPhaseSucceeded:
		s.setBuildCompletedCondition(build, "Build completed successfully")
		if artifacts != nil && artifacts.Image != "" {
			build.Status.ImageStatus.Image = artifacts.Image
		}
	case interfaces.BuildPhaseFailed, interfaces.BuildPhaseError:
		s.setBuildFailedCondition(build, "BuildFailed", status.Message)
	}
}

// determineBuildEngine determines which build engine to use based on build spec
func (s *Service) determineBuildEngine(build *openchoreov1alpha1.Build) string {
	// For now, always use argo. In the future, this could be determined by:
	// - build.Spec.TemplateRef.Engine
	// - build annotations
	// - organization/project defaults
	// - build plane configuration
	if build.Spec.TemplateRef.Engine != "" {
		return build.Spec.TemplateRef.Engine
	}
	return "argo" // default
}

// getBuildPlaneClient gets the build plane and its client
func (s *Service) getBuildPlaneClient(ctx context.Context, build *openchoreov1alpha1.Build) (*openchoreov1alpha1.BuildPlane, client.Client, error) {
	buildPlane, err := controller.GetBuildPlane(ctx, s.client, build)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot retrieve the build plane: %w", err)
	}

	bpClient, err := kubernetesClient.GetK8sClient(s.k8sClientMgr, buildPlane.Namespace, buildPlane.Name, buildPlane.Spec.KubernetesCluster)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get build plane client: %w", err)
	}

	return buildPlane, bpClient, nil
}

// Helper methods for setting conditions
func (s *Service) setBuildInProgressCondition(build *openchoreov1alpha1.Build) {
	condition := NewBuildInProgressCondition(build.Generation)
	meta.SetStatusCondition(&build.Status.Conditions, condition)
}

func (s *Service) setBuildCompletedCondition(build *openchoreov1alpha1.Build, message string) {
	condition := NewBuildCompletedCondition(build.Generation)
	if message != "" {
		condition.Message = message
	}
	meta.SetStatusCondition(&build.Status.Conditions, condition)
}

func (s *Service) setBuildFailedCondition(build *openchoreov1alpha1.Build, reason, message string) {
	condition := NewBuildFailedCondition(build.Generation)
	if reason != "" {
		condition.Reason = reason
	}
	if message != "" {
		condition.Message = message
	}
	meta.SetStatusCondition(&build.Status.Conditions, condition)
}
