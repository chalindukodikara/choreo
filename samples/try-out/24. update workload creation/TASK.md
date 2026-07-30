# Task: Update Workload Generation Workflow to Distinguish Source-Defined vs Auto-Generated Workloads

## Context

The workflow template `samples/getting-started/workflow-templates/generate-workload.yaml` generates a workload CR using `occ` and then creates/updates it via the OpenChoreo API. Currently, it treats all workloads the same regardless of whether the developer provided a `workload.yaml` descriptor in their source repository.

We need the workflow to behave differently based on whether a `workload.yaml` file exists in the source:
- **Source-defined workload** (`workload.yaml` exists): The developer owns the full workload spec. Replace the entire workload via PUT.
- **Auto-generated workload** (`workload.yaml` does not exist): The platform generated a default workload. Only update the container image on the existing workload via GET + merge + PUT, preserving any prior configuration.

## Key Files

- `samples/getting-started/workflow-templates/generate-workload.yaml` — The Argo ClusterWorkflowTemplate to modify
- `api/v1alpha1/workload_types.go` — Workload CRD type definitions (for understanding the spec structure)
- `internal/openchoreo-api/api/handlers/workloads.go` — Workload API handlers (GET/PUT/POST)
- `internal/openchoreo-api/services/workload/service.go` — Workload service layer

## Requirements

### 1. Detect whether `workload.yaml` exists in the source repository

Add a step (or extend an existing step) in the workflow to check whether a `workload.yaml` file is present at the expected path in the cloned repository. The path may be:
- The repository root
- A subdirectory specified by the `applicationPath` parameter

Store the result (true/false) as an output parameter for use in subsequent steps.

### 2. Add an annotation to the WorkflowRun indicating the source of the workload

The workflow already annotates the WorkflowRun with `openchoreo.dev/workload` (the full workload JSON). Add an additional annotation to indicate whether the workload was defined in source or auto-generated.

**Annotation:** `openchoreo.dev/workload-from-source: "true"`
- Set this annotation on the WorkflowRun when `workload.yaml` is found in the source repository
- Do NOT set the annotation when `workload.yaml` is absent (presence-based semantics)

### 3. Update the create/update logic based on workload source

Currently the workflow does:
1. POST workload (create)
2. If 409 Conflict → PUT workload (full replace)

Change this to:

#### 3a. If workload IS source-defined (`workload.yaml` found):
- POST workload (create)
- If 409 Conflict → PUT workload with the full generated CR (full replace, as today)

#### 3b. If workload is NOT source-defined (auto-generated):
- POST workload (create)
- If 409 Conflict:
  1. GET the existing workload from the API: `GET /api/v1/namespaces/{namespaceName}/workloads/{workloadName}`
  2. Update only the container image field (`spec.container.image`) in the fetched workload CR
  3. PUT the merged workload back: `PUT /api/v1/namespaces/{namespaceName}/workloads/{workloadName}`

This ensures that for auto-generated workloads, any manual configuration changes (endpoints, env vars, connections) made outside the workflow are preserved — only the newly built image is updated.

## API Reference

The relevant API endpoints (see `internal/openchoreo-api/api/handlers/workloads.go`):

| Method | Path | Response |
|--------|------|----------|
| POST   | `/api/v1/namespaces/{ns}/workloads` | 201 Created / 409 Conflict |
| GET    | `/api/v1/namespaces/{ns}/workloads/{name}` | 200 OK / 404 Not Found |
| PUT    | `/api/v1/namespaces/{ns}/workloads/{name}` | 200 OK / 404 Not Found |

## Constraints

- The workflow runs as an Argo ClusterWorkflowTemplate with shell-based steps
- Authentication uses OAuth client credentials (token retrieval already exists in the workflow)
- The `occ` CLI is used to generate the initial workload CR from source
- Use `jq` for JSON manipulation (already used in existing steps)
- Workload name format: `{componentName}-workload`
