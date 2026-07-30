# Task: Make `workflowPlaneRef` Defaulted on Workflow and ClusterWorkflow CRDs

## Goal

Ensure `workflowPlaneRef` is always populated on Workflow and ClusterWorkflow resources. Add defaulting webhooks so that if a user omits `workflowPlaneRef`, it is automatically set. Then remove the fallback/resolution logic from controllers since the ref will always be present.

## Default Values (when omitted)

- **Workflow** -> `{ kind: "ClusterWorkflowPlane", name: "default" }`
- **ClusterWorkflow** -> `{ kind: "ClusterWorkflowPlane", name: "default" }`

## Constraint

- ClusterWorkflow can **only** reference `ClusterWorkflowPlane`.
- Workflow can reference both `WorkflowPlane` and `ClusterWorkflowPlane`.

---

## Changes Required

### 1. CRD Type Definitions

- **`api/v1alpha1/workflow_types.go`** (line 23-24): Remove `// +optional` and `omitempty` from the `WorkflowPlaneRef` field. Change to value type or keep pointer but add `+required` marker. Update godoc to explain the default.
- **`api/v1alpha1/clusterworkflow_types.go`** (line 17-18): Same treatment — remove `// +optional` and `omitempty` from `WorkflowPlaneRef`. Update godoc.

### 2. Defaulting Webhooks (NEW)

Create defaulting webhooks following existing patterns (e.g., `internal/webhook/component/webhook.go`):

- **`internal/webhook/workflow/webhook.go`** — Implement `CustomDefaulter` that sets `workflowPlaneRef` to `{ kind: ClusterWorkflowPlane, name: default }` when nil.
- **`internal/webhook/clusterworkflow/webhook.go`** — Implement `CustomDefaulter` that sets `workflowPlaneRef` to `{ kind: ClusterWorkflowPlane, name: default }` when nil.
- Register these webhooks in the manager setup (wherever existing webhooks are registered).

### 3. Remove Fallback Logic in `internal/controller/reference.go`

- **`ResolveWorkflowPlane()`** (lines 277-301): Remove the `if ref == nil` fallback branch entirely. The ref will always be non-nil after webhook defaulting. Add a defensive error return if ref is unexpectedly nil.
- **`GetWorkflowSpec()`** (lines 560-587): Remove the `else` branch (lines 577-583) that injects a default for ClusterWorkflow when `WorkflowPlaneRef` is nil — the CRD will already have the ref set by the webhook.

### 4. API Service — `internal/openchoreo-api/services/workflowrun/service.go`

- **`getWorkflowPlane()`** (lines 361-375): Currently grabs the FIRST WorkflowPlane found in the namespace, ignoring `workflowPlaneRef`. Update to respect the workflow's `workflowPlaneRef` — fetch the specific plane by kind and name.

### 5. OCC CLI Logs — `internal/occ/cmd/workflowrun/logs.go`

- **`resolveWorkflowPlaneObsRef()`**: `workflowName` is validated non-empty before this function is called (line 185-187), and `workflowPlaneRef` is always set by CRD defaulting. Remove the `if workflowName != ""` guard — it's always true. Remove the fallback logic (lines 282-295) that tries namespaced WorkflowPlane "default" then ClusterWorkflowPlane "default". The function should resolve directly from the workflow's `workflowPlaneRef` and return an error if resolution fails instead of silently falling back.
- **`resolveObserverURL()`**: Update comment to reflect that no fallback is needed.

### 6. OpenAPI Spec — `openapi/openchoreo-api.yaml`

- Update the Workflow and ClusterWorkflow schemas to make `workflowPlaneRef` required (add to `required` list) and update descriptions.

### 7. Tests — `internal/controller/reference_test.go`

- Remove or update tests that validate the `ref == nil` fallback behavior (e.g., `TestResolveWorkflowPlane_WithNoRef_*`).
- Add a test: `ResolveWorkflowPlane` returns an error when ref is nil.
- Update `TestWorkflowResult_GetWorkflowSpec_FromClusterWorkflow_NilWorkflowPlaneRef` — this scenario should no longer occur after defaulting.

### 8. Code Generation

After type changes, run:
```bash
make generate manifests openapi-codegen
```

### 9. Update READMEs

### 10. Update/Add test cases

### 11. Critically evaluate whether we have any existing code paths that rely on `workflowPlaneRef` being nil or `workflowName` being empty and ensure they are updated to reflect the new invariants:
- `workflowPlaneRef` will always be set (CRD defaulting).
- `workflowName` will always be non-empty when workflow operations are invoked (validated at call sites).
This includes any logic in controllers, API services, CLI commands, or other components that may have previously handled the nil/empty case with fallback logic.
