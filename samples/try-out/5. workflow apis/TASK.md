
INFO
# When we update anything in the openapi spec, it should generate the client codes. Using a make command.
internal/openchoreo-api/api/gen/*

# Service implementation layer is in the following folder.
- handlers: internal/openchoreo-api/api/handlers/*
- services: internal/openchoreo-api/services/*

# Legacy code (dont need to update)
- internal/openchoreo-api/handlers/handlers.go
- internal/openchoreo-api/legacyservices

1. /api/v1/namespaces/{namespaceName}/workflowruns needs a query param called workflow which is the name of the workflow. With that, we can filter workflowruns of a particular workflow.
Do this.