# Task

References
- api/v1alpha1/componentworkflow_types.go
- api/v1alpha1/componentworkflowrun_types.go
- api/v1alpha1/workflow_types.go
- api/v1alpha1/workflowrun_types.go
- internal/controller/componentworkflowrun/controller.go
- internal/controller/workflowrun/controller.go


1. Introduce ttlAfterCompletion field for ComponentWorkflow and Workflow. When a workflowRun/ComponentWorkflowRun is created, that value should be passed to the workflow run. Once the time is up, it should get deleted.
2. Field should be like this:
    - ttlAfterCompletion: string (duration string, e.g. "10d 1h 10m 100s", "90d", "1h30m", "1000s")
    - Can include days, hours, minutes, seconds.
3. For Component Workflow Runs, it should check if the generate-workload-cr step is there, if it is there, create the Workload CR and then onwards wait for the given time and get deleted.
4. For Workflow Runs, it should just wait for the given time and get deleted.
