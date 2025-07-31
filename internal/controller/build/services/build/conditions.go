// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package build

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openchoreo/openchoreo/internal/controller"
)

// Build condition types
const (
	ConditionBuildInitiated  controller.ConditionType = "BuildInitiated"
	ConditionBuildTriggered  controller.ConditionType = "BuildTriggered"
	ConditionBuildCompleted  controller.ConditionType = "BuildCompleted"
	ConditionWorkloadUpdated controller.ConditionType = "WorkloadUpdated"
)

// Build condition reasons
const (
	ReasonBuildInitiated       controller.ConditionReason = "BuildInitiated"
	ReasonBuildTriggered       controller.ConditionReason = "BuildTriggered"
	ReasonBuildCompleted       controller.ConditionReason = "BuildCompleted"
	ReasonBuildFailed          controller.ConditionReason = "BuildFailed"
	ReasonBuildInProgress      controller.ConditionReason = "BuildInProgress"
	ReasonWorkloadUpdated      controller.ConditionReason = "WorkloadUpdated"
	ReasonWorkloadUpdateFailed controller.ConditionReason = "WorkloadUpdateFailed"
)

// NewBuildInProgressCondition creates a new BuildInProgress condition
func NewBuildInProgressCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionBuildCompleted),
		Status:             metav1.ConditionFalse,
		Reason:             string(ReasonBuildInProgress),
		Message:            "Build is in progress",
		ObservedGeneration: generation,
	}
}

// NewBuildCompletedCondition creates a new BuildCompleted condition
func NewBuildCompletedCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionBuildCompleted),
		Status:             metav1.ConditionTrue,
		Reason:             string(ReasonBuildCompleted),
		Message:            "Build completed successfully",
		ObservedGeneration: generation,
	}
}

// NewBuildFailedCondition creates a new BuildFailed condition
func NewBuildFailedCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionBuildCompleted),
		Status:             metav1.ConditionFalse,
		Reason:             string(ReasonBuildFailed),
		Message:            "Build failed",
		ObservedGeneration: generation,
	}
}

// NewWorkloadUpdatedCondition creates a new WorkloadUpdated condition
func NewWorkloadUpdatedCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionWorkloadUpdated),
		Status:             metav1.ConditionTrue,
		Reason:             string(ReasonWorkloadUpdated),
		Message:            "Workload updated successfully",
		ObservedGeneration: generation,
	}
}

// NewWorkloadUpdateFailedCondition creates a new WorkloadUpdateFailed condition
func NewWorkloadUpdateFailedCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(ConditionWorkloadUpdated),
		Status:             metav1.ConditionFalse,
		Reason:             string(ReasonWorkloadUpdateFailed),
		Message:            "Failed to update workload with the new built image",
		ObservedGeneration: generation,
	}
}
