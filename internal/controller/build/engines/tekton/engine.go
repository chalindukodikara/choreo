// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package tekton

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openchoreov1alpha1 "github.com/openchoreo/openchoreo/api/v1alpha1"
	"github.com/openchoreo/openchoreo/internal/controller/build/interfaces"
)

// Engine implements BuildEngine interface for Tekton Pipelines
type Engine struct {
	logger logr.Logger
}

// NewEngine creates a new Tekton build engine
func NewEngine() *Engine {
	return &Engine{
		logger: log.Log.WithName("tekton-build-engine"),
	}
}

// GetName returns the name of the build engine
func (e *Engine) GetName() string {
	return "tekton"
}

// EnsurePrerequisites creates necessary prerequisites for Tekton builds
func (e *Engine) EnsurePrerequisites(ctx context.Context, client client.Client, build *openchoreov1alpha1.Build) error {
	// TODO: Implement Tekton-specific prerequisites
	// - ServiceAccount
	// - Secrets for registry access
	// - PVC for workspace
	return fmt.Errorf("tekton engine not yet implemented")
}

// CreateBuild creates a Tekton PipelineRun for the build
func (e *Engine) CreateBuild(ctx context.Context, client client.Client, build *openchoreov1alpha1.Build) (interfaces.BuildInfo, error) {
	// TODO: Implement Tekton PipelineRun creation
	return interfaces.BuildInfo{}, fmt.Errorf("tekton engine not yet implemented")
}

// GetBuildStatus retrieves the current status of the Tekton PipelineRun
func (e *Engine) GetBuildStatus(ctx context.Context, client client.Client, build *openchoreov1alpha1.Build) (interfaces.BuildStatus, error) {
	// TODO: Implement Tekton PipelineRun status checking
	return interfaces.BuildStatus{}, fmt.Errorf("tekton engine not yet implemented")
}

// ExtractBuildArtifacts extracts artifacts from completed Tekton build
func (e *Engine) ExtractBuildArtifacts(ctx context.Context, client client.Client, build *openchoreov1alpha1.Build) (*interfaces.BuildArtifacts, error) {
	// TODO: Implement artifact extraction from Tekton PipelineRun results
	return nil, fmt.Errorf("tekton engine not yet implemented")
}
