// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package workload

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openchoreov1alpha1 "github.com/openchoreo/openchoreo/api/v1alpha1"
	"github.com/openchoreo/openchoreo/internal/controller/build/interfaces"
)

// Service handles workload updates from build artifacts (NOT creation)
type Service struct {
	client client.Client
	logger logr.Logger
}

// NewService creates a new workload service
func NewService(client client.Client) *Service {
	return &Service{
		client: client,
		logger: log.Log.WithName("workload-update-service"),
	}
}

// UpdateWorkloadWithBuildArtifacts updates an existing workload with new build artifacts
// This does NOT create workloads - workloads should be created by developers
func (s *Service) UpdateWorkloadWithBuildArtifacts(ctx context.Context, build *openchoreov1alpha1.Build, artifacts *interfaces.BuildArtifacts) error {
	logger := s.logger.WithValues("build", build.Name)

	// Find the workload associated with this build
	workload, err := s.findWorkloadForBuild(ctx, build)
	if err != nil {
		return fmt.Errorf("failed to find workload for build: %w", err)
	}

	if workload == nil {
		logger.Info("No workload found for this build, skipping workload update")
		return nil
	}

	// Update workload with build information
	if err := s.updateWorkloadWithBuildInfo(workload, build, artifacts); err != nil {
		return fmt.Errorf("failed to update workload with build info: %w", err)
	}

	// Update the workload
	if err := s.client.Update(ctx, workload); err != nil {
		return fmt.Errorf("failed to update workload: %w", err)
	}

	logger.Info("Successfully updated workload with build artifacts",
		"workload", workload.Name,
		"image", artifacts.Image)
	return nil
}

// findWorkloadForBuild finds the workload that should be updated based on the build
func (s *Service) findWorkloadForBuild(ctx context.Context, build *openchoreov1alpha1.Build) (*openchoreov1alpha1.Workload, error) {
	// Strategy 1: Look for workload with matching component name
	workloadList := &openchoreov1alpha1.WorkloadList{}
	if err := s.client.List(ctx, workloadList,
		client.InNamespace(build.Namespace),
		client.MatchingLabels{
			"openchoreo.dev/component-name": build.Spec.Owner.ComponentName,
			"openchoreo.dev/project-name":   build.Spec.Owner.ProjectName,
		}); err != nil {
		return nil, err
	}

	if len(workloadList.Items) > 0 {
		return &workloadList.Items[0], nil
	}

	// Strategy 2: Look for workload by name convention
	workloadName := fmt.Sprintf("%s-workload", build.Spec.Owner.ComponentName)
	workload := &openchoreov1alpha1.Workload{}
	err := s.client.Get(ctx, client.ObjectKey{
		Name:      workloadName,
		Namespace: build.Namespace,
	}, workload)

	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil // Not found, but that's okay
		}
		return nil, err
	}

	return workload, nil
}

// updateWorkloadWithBuildInfo updates workload with build-specific information
func (s *Service) updateWorkloadWithBuildInfo(workload *openchoreov1alpha1.Workload, build *openchoreov1alpha1.Build, artifacts *interfaces.BuildArtifacts) error {
	// Add build annotations
	if workload.Annotations == nil {
		workload.Annotations = make(map[string]string)
	}

	workload.Annotations["openchoreo.dev/last-build"] = build.Name
	workload.Annotations["openchoreo.dev/source-repo"] = build.Spec.Repository.URL

	if build.Spec.Repository.Revision.Commit != "" {
		workload.Annotations["openchoreo.dev/source-commit"] = build.Spec.Repository.Revision.Commit
	}
	if build.Spec.Repository.Revision.Branch != "" {
		workload.Annotations["openchoreo.dev/source-branch"] = build.Spec.Repository.Revision.Branch
	}

	// Update container images in workload spec if we have a new image
	if artifacts.Image != "" {
		s.updateContainerImages(workload, artifacts.Image)
	}

	return nil
}

// updateContainerImages updates container image references in the workload
func (s *Service) updateContainerImages(workload *openchoreov1alpha1.Workload, image string) {
	// Update the container image in workload template spec
	if workload.Spec.WorkloadTemplateSpec.Containers != nil {
		for i := range workload.Spec.WorkloadTemplateSpec.Containers {
			// Update the first container or container matching component name
			if i == 0 || workload.Spec.WorkloadTemplateSpec.Containers[i].Name == workload.Spec.Owner.ComponentName {
				workload.Spec.WorkloadTemplateSpec.Containers[i].Image = image
				s.logger.V(1).Info("Updated container image",
					"container", workload.Spec.WorkloadTemplateSpec.Containers[i].Name,
					"image", image)
			}
		}
	}
}

// ValidateWorkload validates a workload CR for correctness
func (s *Service) ValidateWorkload(ctx context.Context, workload *openchoreov1alpha1.Workload) error {
	// TODO: Implement workload validation
	// This could include:
	// - Schema validation
	// - Resource limits validation
	// - Security policy validation
	// - Dependencies validation

	return nil
}
