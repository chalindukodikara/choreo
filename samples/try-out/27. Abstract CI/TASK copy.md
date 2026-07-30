## PROBLEM
Currently, there is no abstraction layer for openchoreo workflows. different modules cannot be plugged in. Argo Workflow should be hard coded into the run template section in the workflow spec. It is hard to abstract out a single abstraction since there are lot of concepts in the CI and different vendors have different ones.

## Requirement
Different moduels should be plugged in as openchoreo modules. Should support tekton pipelines, azure pipelines, jenkins, circle ci, github actions, etc.

## Proposal
If we want to expand the capability to any workflow beyond k8s native ones.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: dockerfile-builder
  labels:
    openchoreo.dev/workflow-type: "component"
  annotations:
    openchoreo.dev/description: "Build with a provided Dockerfile/Containerfile/Podmanfile"
spec:
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
  ttlAfterCompletion: "1d"
  parameters:
    ...
  runTemplate:
    apiVersion: argoproj.io/v1alpha1
    kind: WorkflowAbstraction
    metadata:
      name: ${metadata.workflowRunName}
      namespace: ${metadata.namespace}
    spec:
      template:
        # Anything
        # For argo, this will be an argo workflow CR. 
        # For tektone, this will be a tekton pipeline CR.
        # For azure pipelines, this will be set of params required to trigger, retrieve, view azure pipelines. Relevant controller will do API calls, etc.
  resources:
    ...
```

- There will be a controller running in the openchoreo-workflow-plane namespace in the workflow plane and listening to this new CR that is applied. It will get it and it will have the logic (written by module owners). 
    - If it is an argo workflow, it will create service account, role, role binding and apply the CR into relevant namespace. WorkflowAbstraction and Argo Workflow will be in the same namespace.
    - If it is a tekton pipeline, relevant controller will apply it and write the status to the WorkflowAbstraction CR. 
    - If it is an azure pipeline, its controller will call AZURE APIs and trigger the workflow, retrieve the status and write to our CR.
    - etc. any ci can be plugged in.
- We will still support argo workflow in the runTemplate to preserve the backward compatibility.
- WorkflowRun controller will simply looking into this WorkflowAbstraction CR and its status will be set into workflowRun status.

## Decisions
1. Are there any other alternative suggestions to achieve this?
2. What should be this CRs name? Good suggestions?
3. Any issues/flaws in this design?
