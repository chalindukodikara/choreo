# Task

- Validate Component Workflows exist which are defined in the ComponentType CRD. When the Component controller reconciles a Component, it should check that the referenced ComponentWorkflow is from the allowedWorkflows list in the ComponentType.
- We should check the existance of the ComponentWorkflow referenced in the component and it is in the allowedWorkflows list in the ComponentType. If not, we should update the component status to indicate the error and not proceed with component creation.
- For performance, lets first check the allowedWorkflows list in the ComponentType. If the referenced ComponentWorkflow is not in that list, we can directly update the component status with the error without checking for the existence of the ComponentWorkflow. We only need to check for the existence of the ComponentWorkflow if it is in the allowedWorkflows list.
- Component Controller should listen to new ComponentWorkflows and ComponentType allowedWorkflow field updates and reconcile the components accordingly. If there is a new ComponentWorkflow or an update to the allowedWorkflows field in the ComponentType, we should check if the referenced ComponentWorkflow exists and is in the allowedWorkflows list. If not, we should update the component status to indicate the error and not proceed with component creation.
- We should also validate in the component workflow run controller that the referenced ComponentWorkflow exists and is in the allowedWorkflows list in the ComponentType. If not, we should update the ComponentWorkflowRun status to indicate the error and not proceed with workflow run creation. WorkflowRun should get failed and not proceed again.

References
- api/v1alpha1/componentworkflow_types.go
- api/v1alpha1/component_types.go
- api/v1alpha1/componenttype_types.go
- api/v1alpha1/componentworkflowrun_types.go
- internal/controller/component/controller.go
- internal/controller/componentworkflowrun/controller.go

# Next Steps
- Don't need to evaluate whether component type has allowed workflows or not.
- It is enough to check if the component is ready before proceeding with the workflow run creation. Component will make sure component workflow exists and in the allowed List.
