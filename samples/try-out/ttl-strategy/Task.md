# Task

References
- api/v1alpha1/componentworkflow_types.go
- api/v1alpha1/componentworkflowrun_types.go
- internal/controller/componentworkflowrun/controller.go

- Introduce TTL Strategy for ComponentWorkflow and Workflow. When a workflowRun/ComponentWorkflowRun is created, we should check if the referenced Workflow/ComponentWorkflow has a TTL strategy defined. If it does, we should set the TTL for the workflow run accordingly. Once the TTL expires, the workflow run should be automatically deleted by Kubernetes. This will help to clean up old workflow runs and save resources in the cluster. 
- How we can do that?

Fields
TTLStrategy is the strategy for the time to live depending on if the workflow succeeded or failed

Fields
Field Name	Field Type	Description
secondsAfterCompletion	integer	SecondsAfterCompletion is the number of seconds to live after completion
secondsAfterFailure	integer	SecondsAfterFailure is the number of seconds to live after failure
secondsAfterSuccess	integer	SecondsAfterSuccess is the number of seconds to live after success

Should have fields,
 - finishedAt
 - startedAt 

Component Workflow CR/Workflow CR
spec:
    ttlStrategy:
        secondsAfterCompletion:
        secondsAfterSuccess:
        secondsAfterFailure:
    successfulRunsHistoryLimit: 10
    failedRunsHistoryLimit: 10

ComponentWorkflowRun CR/WorkflowRun CR
spec:
    ttlStrategy:
        secondsAfterCompletion:
        secondsAfterSuccess:
        secondsAfterFailure:
    successfulRunsHistoryLimit: 10
    failedRunsHistoryLimit: 10


1. Can component override this? is this limit per component? if so component should be able to override this. Workflow Runs, there is no component in betwen.
2. Which controller should responsible for doing failedRunsHistoryLimit and successfulRunsHistoryLimit. TTL Strategy should be done by the run controller.
