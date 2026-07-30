## Map outputs to status

Current workflow:
samples/try-out/13. Mapping Outputs/workflow.yaml

Suggestion:
samples/try-out/13. Mapping Outputs/mapping.yaml

Currently, there is no way to map such outputs from different steps/tasks to the workflowrun status. We want to add a new field in the workflowrun status to store those outputs, and we want to update the workflowrun controller to map those outputs to the workflowrun status.

We need to consider both argo workflows/tektone/etc to generalize workflow and workflow run. Currently we only support argo workflows, but we want to make it more generic in the future. So we need to design the workflow and workflow run spec in a way that it can support different workflow engines in the future.

1. Suggest a good structure that works for both argo workflows and tektone. We can have a field in the workflow spec to define the outputs that we want to map, and we can have a field in the workflowrun status to store those outputs. The workflow controller will be responsible for mapping the outputs from the workflow to the workflowrun status based on the definition in the workflow spec.


