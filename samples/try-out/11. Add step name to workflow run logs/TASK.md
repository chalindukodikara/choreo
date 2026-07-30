INFO
# When we update anything in the openapi spec, it should generate the client codes. Using a make command.
internal/openchoreo-api/api/gen/*

# Service implementation layer is in the following folder.
- handlers: internal/openchoreo-api/api/handlers/*
- services: internal/openchoreo-api/services/*

# Legacy code (dont need to update)
- internal/openchoreo-api/handlers/handlers.go
- internal/openchoreo-api/legacyservices

Task
1. We need to add taskName query param to the workflow run logs new API.
- internal/openchoreo-api/handlers/workflowruns.go
- 	logs, err := h.services.WorkflowRunService.GetWorkflowRunLogs(ctx, namespaceName, runName, stepName, h.config.ClusterGateway.URL, sinceSeconds)

1.1 In old API, we used stepName, but for new one, lets use taskName.

1.2 Update the OpenAPI spec first and do the implementation after that.
- spec: openapi/openchoreo-api.yaml