# Task: Add WorkflowPlane Reference to Git Secret API

## Status: COMPLETED

## Problem

The git secret creation API (`POST /api/v1alpha1/namespaces/{namespaceName}/gitsecrets`) previously auto-discovered the workflow plane by listing all `WorkflowPlane` resources in the namespace and picking the first one. This had three problems:

1. **No ClusterWorkflowPlane support**: `getWorkflowPlane()` only listed namespace-scoped `WorkflowPlane` resources. It ignored cluster-scoped `ClusterWorkflowPlane` resources entirely.
2. **No explicit selection**: If multiple workflow planes existed, the user could not choose which one to use. The system silently picked the first one.
3. **No traceability**: The created `SecretReference` CR had no record of which workflow plane was used, making it impossible to know where the secret was pushed to.

## Requirements

### 1. Accept WorkflowPlane reference in the Create request

Add `workflowPlaneKind` and `workflowPlaneName` fields to the `CreateGitSecretRequest`:

- `workflowPlaneKind` (required, string, enum: `"WorkflowPlane"` | `"ClusterWorkflowPlane"`)
- `workflowPlaneName` (required, string) — name of the workflow plane resource

These should be used to fetch the correct workflow plane instead of the current auto-discovery logic.

### 2. Store WorkflowPlane reference on the SecretReference CR

Add **labels** to the `SecretReference` created in `buildSecretReference()`:

- `openchoreo.dev/workflow-plane-kind`: `"WorkflowPlane"` or `"ClusterWorkflowPlane"`
- `openchoreo.dev/workflow-plane-name`: name of the workflow plane

Additionally, move existing metadata (`openchoreo.dev/secret-type` and `kubernetes.io/secret-type`) from annotations to labels for consistent filtering support.

### 3. Return WorkflowPlane info in responses

Update `GitSecretResponse` and `GitSecretInfo` to include:

- `workflowPlaneName` (string)
- `workflowPlaneKind` (string)

These should be populated:
- **On create**: from the request parameters
- **On list**: by reading the labels from the `SecretReference` CR

### 4. Support both WorkflowPlane and ClusterWorkflowPlane in service layer

Replace the current `getWorkflowPlane()` method with logic that:
- If `kind == "WorkflowPlane"`: fetch the namespace-scoped `WorkflowPlane` by name from the namespace
- If `kind == "ClusterWorkflowPlane"`: fetch the cluster-scoped `ClusterWorkflowPlane` by name

The `DeleteGitSecret` flow must also be updated to read the workflow plane kind/name from the SecretReference labels instead of using auto-discovery.

## Validation Rules

- `workflowPlaneKind` must be exactly `"WorkflowPlane"` or `"ClusterWorkflowPlane"`
- `workflowPlaneName` must be non-empty
- The referenced workflow plane must exist (return appropriate error if not found)
- The workflow plane must have `secretStoreRef` configured (existing check, kept as-is)

## Testing

After making changes, run:
```bash
make openapi-codegen   # Regenerate models from OpenAPI spec
make test              # Unit tests
make code.gen-check    # Verify generated code is up to date
make lint-fix          # Fix lint issues
make go.build          # Verify build
```
