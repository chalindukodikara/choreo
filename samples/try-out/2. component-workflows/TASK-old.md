We have decided to merge component workflows and workflows.
More details, read: samples/try-out/2. component-workflows/discussion.md

1. Remove ComponentWorkflow and ComponentWorkflowRun APIs from api/v1alpha1/componentworkflow_types.go. Remove all other rbac and other resources.
2. Remove SystemParameters from the Component API. api/v1alpha1/component_types.go
3. We need to update the API endpoints to reflect the merged concept.
internal/openchoreo-api/handlers/handlers.go

3.1 Remove all these endpoints related to component workflows. in internal/openchoreo-api/handlers/handlers.go.
```bash
	// ComponentWorkflow endpoints (component-specific workflows)
	api.HandleFunc("POST "+v1+"/namespaces/{namespaceName}/component-workflows/definition", h.CreateComponentWorkflowDefinition)
	api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/component-workflows", h.ListComponentWorkflows)
	api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/component-workflows/{cwName}/schema", h.GetComponentWorkflowSchema)
	api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/component-workflows/{cwName}/definition", h.GetComponentWorkflowDefinition)
	api.HandleFunc("PUT "+v1+"/namespaces/{namespaceName}/component-workflows/{cwName}/definition", h.UpdateComponentWorkflowDefinition)
	api.HandleFunc("DELETE "+v1+"/namespaces/{namespaceName}/component-workflows/{cwName}/definition", h.DeleteComponentWorkflowDefinition)
	api.HandleFunc("POST "+v1+"/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs", h.CreateComponentWorkflowRun)
	api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs", h.ListComponentWorkflowRuns)
	api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs/{runName}", h.GetComponentWorkflowRun)
	api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs/{runName}/status", h.GetComponentWorkflowRunStatus)
	api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs/{runName}/logs", h.GetComponentWorkflowRunLogs)
	api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs/{runName}/events", h.GetComponentWorkflowRunEvents)
```
3.2 Add following. Some implementations can be borrowed from component workflow handlers to workflow service or workflow run service as appropriate.
POST /namespaces/{namespaceName}/workflows/{workflowName}/definition like /namespaces/{namespaceName}/component-workflows/definition
GET /namespaces/{namespaceName}/workflow-runs/{runName}/logs like /namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs/{runName}/logs
3.3 Update following endpoint to avoid using systemParameters. To update the workflow-parameters. This is updating the component.
"PATCH "+v1+"/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-parameters

Remove systemParameters from:
UpdateComponentWorkflowRequest in internal/openchoreo-api/models/request.go

4. Fix auto build.
   routes.HandleFunc("POST "+v1+"/webhooks/github", h.HandleGitHubWebhook)
   routes.HandleFunc("POST "+v1+"/webhooks/gitlab", h.HandleGitLabWebhook)
   routes.HandleFunc("POST "+v1+"/webhooks/bitbucket", h.HandleBitbucketWebhook)

4.1 For affectedComponents, err := s.findAffectedComponents(ctx, event) in internal/openchoreo-api/legacyservices/webhook_service.go.
Retrieve workflow CR and get annotations and find repoUrl and appPath location. Then, you can get the actual values from the component parameters section.

4.2 This "func (s *ComponentWorkflowService) triggerWorkflowInternal(ctx context.Context, namespaceName, projectName, componentName, commit string) (*models.ComponentWorkflowResponse, error) {
also needs to remove systemParameters and follow the same as 4.1.

4.3 Check whether there are any issuesd in the apis after component workflow and componentworkflow run deleted.

5. Validate whether it adds labels project name and component name to the workflow run CR when creating a workflow run from a component. and make those query params for workflow run listing and get.

6. Remove componentworkflow related functions used in the openchoreo api server such as internal/openchoreo-api/legacyservices/component_workflow_service.go.

7. Remove internal/openchoreo-api/legacyservices/component_workflow_service.go file itself. If there are any functions that can be reused, move them to workflow service or workflow run service as appropriate.

8. Fix func (s *ComponentService) createComponentResources(ctx context.Context, namespaceName, projectName string, req *models.CreateComponentRequest) (*openchoreov1alpha1.Component, error) {
in component service to remove system parameters and use parameters.

9. Fix mcp related code related to component workflow if there are any. internal/mcp/* and pkg/mcp/tools/. Remove functions like ListComponentWorkflowRuns from internal/openchoreo-api/mcphandlers/components.go and use workflow related functions instead.

10. Remove  component workflow related files from "config" folder in the root generated from kubebuilder.

11. Check whether there are any other places where component workflow or component workflow run is used and fix those.

12. SecretRef in the annotation ("openchoreo.dev/component-workflow-parameters") should be used for retrieving the secret reference CR and adding it to the cel context for rendering. Get the annotation from the workflow if exists, and find secretRef field. Find the corresponding secretRef from the workflowRun CR. When the name is found (only if annotations are there),
read the secretReference CR and add it to the context. Add that to the rendering logic of workflow to use that secretRef like in componentworkflow rendering. "internal/pipeline/workflow/*"

13. Rename the following APIs.
From:
    api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflow-runs", h.ListWorkflowRuns)
    api.HandleFunc("POST "+v1+"/namespaces/{namespaceName}/workflow-runs", h.CreateWorkflowRun)
    api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflow-runs/{runName}", h.GetWorkflowRun)
    api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflow-runs/{runName}/logs", h.GetWorkflowRunLogs)
    api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflow-runs/{runName}/events", h.GetWorkflowRunEvents)
To: I have already added these. Just delete older ones. 
    api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflowruns", h.ListWorkflowRuns)
    api.HandleFunc("POST "+v1+"/namespaces/{namespaceName}/workflowruns", h.CreateWorkflowRun)
    api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflowruns/{runName}", h.GetWorkflowRun)
    api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflowruns/{runName}/logs", h.GetWorkflowRunLogs)
    api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflowruns/{runName}/events", h.GetWorkflowRunEvents)

13.1 CreateWorkflowRun function should not depend on annotations. It should expect that the whole yaml will be passed in the request body. Should not add any labels, that should be a responsibility of the caller.
But we should get labels from the request body and use that to pass to the authorization.

14. Add workload creation logic to the workflow run controller similar to component workflow run controller. Now component workflow run controller is deleted, if you want get a git diff. Just copy paste the functions to workflow run controller.

15. Update samples with Workflows. Add annotations and labels to the workflow CR and workflow run CR.
Add annotation to workflows, openchoreo.dev/component-workflow-parameters: "repoUrl: parameters.repository.url, branch: parameters.repository.revision.branch, appPath: parameters.repository.appPath, secretRef: parameters.repository.secretRef, projectName: parameters.scope.projectName, componentName: parameters.scope.componentName"
samples/getting-started/component-workflows rename to - samples/getting-started/workflows
- samples/getting-started/component-workflows/*
- samples/component-workflows/docker.yaml
- samples/component-workflows/react.yaml
- samples/component-workflows/google-cloud-buildpacks.yaml

Update all other samples with workflowrun to include labels for project name and component name.

Current Implementation
samples/try-out/2. component-workflows/IMPLEMENTATION.md
samples/try-out/2. component-workflows/TASK.md

New Tasks
1. Update handlers of workflow run apis.
internal/openchoreo-api/handlers/handlers.go

```bash
api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflowruns", h.ListWorkflowRuns)
api.HandleFunc("POST "+v1+"/namespaces/{namespaceName}/workflowruns", h.CreateWorkflowRun)
api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflowruns/{runName}", h.GetWorkflowRun)
api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflowruns/{runName}/logs", h.GetWorkflowRunLogs)
api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflowruns/{runName}/status", h.GetWorkflowRunStatus)
api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflowruns/{runName}/events", h.GetWorkflowRunEvents)
```

Example: Use pattern used in "internal/openchoreo-api/services/component/*"

2. Update openapi spec with the changes of workflow run apis. Check whether component workflow run related APIs are there and remove those.
openapi/openchoreo-api.yaml

 Use
annotations:
# The controller can parse this string as YAML into a map[string]string.
openchoreo.dev/component-workflow-parameters: |
    repoUrl: parameters.repository.url
    branch: parameters.repository.revision.branch
    commit: parameters.repository.revision.commit
    appPath: parameters.repository.appPath
    secretRef: parameters.repository.secretRef
    projectName: parameters.scope.projectName
    componentName: parameters.scope.componentName
3. Fix auto build.
routes.HandleFunc("POST "+v1+"/webhooks/github", h.HandleGitHubWebhook)
routes.HandleFunc("POST "+v1+"/webhooks/gitlab", h.HandleGitLabWebhook)
routes.HandleFunc("POST "+v1+"/webhooks/bitbucket", h.HandleBitbucketWebhook)

3.1 For affectedComponents, err := s.findAffectedComponents(ctx, event) in internal/openchoreo-api/legacyservices/webhook_service.go.
Retrieve workflow CR and get annotations and find repoUrl and appPath location. Then, you can get the actual values from the component parameters section.

4. WorkflowRun controller needs to parse the correct annotation format.

5. update samples
- samples/getting-started/workflows/*
- samples/component-workflows/docker.yaml
- samples/component-workflows/react.yaml
- samples/component-workflows/google-cloud-buildpacks.yaml