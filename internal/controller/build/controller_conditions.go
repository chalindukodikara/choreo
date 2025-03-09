package build

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	choreov1 "github.com/choreo-idp/choreo/api/v1"
	"github.com/choreo-idp/choreo/internal/controller"
	argoproj "github.com/choreo-idp/choreo/internal/dataplane/kubernetes/types/argoproj.io/workflow/v1alpha1"
)

// Constants for condition types

const (
	// ConditionInitialized represents whether the workflow has been created
	ConditionInitialized controller.ConditionType = "Initialized"
	// ConditionCloneSucceeded represents whether the source code clone step is succeeded
	ConditionCloneSucceeded controller.ConditionType = "CloneSucceeded"
	// ConditionBuildSucceeded represents whether the build step is succeeded
	ConditionBuildSucceeded controller.ConditionType = "BuildSucceeded"
	// ConditionPushSucceeded represents whether the push step is succeeded
	ConditionPushSucceeded controller.ConditionType = "PushSucceeded"
	// ConditionCompleted represents whether the ci workflow is completed
	ConditionCompleted controller.ConditionType = "Completed"
	// ConditionDeployableArtifactCreated represents whether the deployable artifact is created after a successful build
	ConditionDeployableArtifactCreated controller.ConditionType = "DeployableArtifactCreated"
	// ConditionDeploymentApplied represents whether the deployment is created/updated when auto deploy is enabled
	ConditionDeploymentApplied controller.ConditionType = "DeploymentApplied"
)

// Constants for condition reasons

const (
	// Reason for Initialized condition type

	// ReasonWorkflowCreatedSuccessfully represents the workflow has been created successfully
	ReasonWorkflowCreatedSuccessfully controller.ConditionReason = "WorkflowCreated"

	// Reasons for ci workflow related conditions

	ReasonCloneSucceeded    controller.ConditionReason = "CloneSourceCodeSucceeded"
	ReasonCloneFailed       controller.ConditionReason = "CloneSourceCodeFailed"
	ReasonBuildSucceeded    controller.ConditionReason = "BuildImageSucceeded"
	ReasonBuildFailed       controller.ConditionReason = "BuildImageFailed"
	ReasonPushSucceeded     controller.ConditionReason = "PushImageSucceeded"
	ReasonPushFailed        controller.ConditionReason = "PushImageFailed"
	ReasonWorkflowCompleted controller.ConditionReason = "BuildCompleted"
	ReasonWorkflowFailed    controller.ConditionReason = "BuildFailed"

	// ReasonArtifactCreatedSuccessfully represents the reason for DeployableArtifactCreated condition type
	ReasonArtifactCreatedSuccessfully controller.ConditionReason = "ArtifactCreationSuccessful"

	// Reasons for auto deployment related conditions

	ReasonAutoDeploymentFailed  controller.ConditionReason = "DeploymentFailed"
	ReasonAutoDeploymentApplied controller.ConditionReason = "DeploymentAppliedSuccessfully"
)

func NewWorkflowInitializedCondition(generation int64) metav1.Condition {
	return controller.NewCondition(
		ConditionInitialized,
		metav1.ConditionTrue,
		ReasonWorkflowCreatedSuccessfully,
		"Workflow was created in the cluster.",
		generation,
	)
}

func NewDeployableArtifactCreatedCondition(generation int64) metav1.Condition {
	return controller.NewCondition(
		ConditionDeployableArtifactCreated,
		metav1.ConditionTrue,
		ReasonArtifactCreatedSuccessfully,
		"Successfully created a deployable artifact for the build.",
		generation,
	)
}

func NewAutoDeploymentFailedCondition(generation int64) metav1.Condition {
	return controller.NewCondition(
		ConditionDeploymentApplied,
		metav1.ConditionFalse,
		ReasonAutoDeploymentFailed,
		"Deployment configuration failed.",
		generation,
	)
}

func NewAutoDeploymentSuccessfulCondition(generation int64) metav1.Condition {
	return controller.NewCondition(
		ConditionDeploymentApplied,
		metav1.ConditionTrue,
		ReasonAutoDeploymentApplied,
		"Successfully configured the deployment.",
		generation,
	)
}

func (r *Reconciler) markStepAsSucceeded(build *choreov1.Build, conditionType controller.ConditionType) {
	successDescriptors := map[controller.ConditionType]struct {
		Reason  controller.ConditionReason
		Message string
	}{
		ConditionCloneSucceeded: {
			Reason:  ReasonCloneSucceeded,
			Message: "Source code cloning was successful.",
		},
		ConditionBuildSucceeded: {
			Reason:  ReasonBuildSucceeded,
			Message: "Building the source code was successful.",
		},
		ConditionPushSucceeded: {
			Reason:  ReasonPushSucceeded,
			Message: "Pushing the built image to the registry was successful.",
		},
	}

	meta.SetStatusCondition(&build.Status.Conditions, controller.NewCondition(
		conditionType,
		metav1.ConditionTrue,
		successDescriptors[conditionType].Reason,
		successDescriptors[conditionType].Message,
		build.Generation,
	))
}

func (r *Reconciler) markStepAsFailed(build *choreov1.Build, conditionType controller.ConditionType) {
	failureDescriptors := map[controller.ConditionType]struct {
		Reason  controller.ConditionReason
		Message string
	}{
		ConditionCloneSucceeded: {
			Reason:  ReasonCloneFailed,
			Message: "Source code cloning failed.",
		},
		ConditionBuildSucceeded: {
			Reason:  ReasonBuildFailed,
			Message: "Building the source code failed.",
		},
		ConditionPushSucceeded: {
			Reason:  ReasonPushFailed,
			Message: "Pushing the built image to the registry failed.",
		},
	}

	meta.SetStatusCondition(&build.Status.Conditions, controller.NewCondition(
		conditionType,
		metav1.ConditionFalse,
		failureDescriptors[conditionType].Reason,
		failureDescriptors[conditionType].Message,
		build.Generation,
	))

	meta.SetStatusCondition(&build.Status.Conditions, controller.NewCondition(
		ConditionCompleted,
		metav1.ConditionFalse,
		ReasonWorkflowFailed,
		"Build completed with a failure status",
		build.Generation,
	))
}

func (r *Reconciler) markWorkflowCompleted(build *choreov1.Build, argoPushStepOutput *argoproj.Outputs) {
	newCondition := metav1.Condition{
		Type:               string(ConditionCompleted),
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             string(ReasonWorkflowCompleted),
		Message:            "Build completed successfully.",
	}
	image := getImageNameFromWorkflow(*argoPushStepOutput)
	if image == "" {
		newCondition.Status = metav1.ConditionFalse
		newCondition.Reason = string(ReasonWorkflowFailed)
		newCondition.Message = "Image name is not found in the workflow"
	} else {
		build.Status.ImageStatus.Image = image
	}
	meta.SetStatusCondition(&build.Status.Conditions, newCondition)
}

func getImageNameFromWorkflow(output argoproj.Outputs) string {
	for _, param := range output.Parameters {
		if param.Name == "image" {
			return *param.Value
		}
	}
	return ""
}
