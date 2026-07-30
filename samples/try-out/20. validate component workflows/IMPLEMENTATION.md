# Implementation: Validate Component Workflows in WorkflowRun Controller

## Overview

Added validation in the WorkflowRun controller to ensure component-scoped WorkflowRuns reference workflows that are allowed by the component's ComponentType and match the component's configured workflow. This prevents bypasses when WorkflowRuns are created directly via `occ apply` or the API.

Additionally, changed the default workflow kind from `Workflow` to `ClusterWorkflow` across the codebase, introduced a separate `ComponentWorkflowConfig` struct for mutable component workflow references, and removed dead-code empty-kind fallbacks.

## Files Changed

### New Files

| File | Purpose |
|------|---------|
| `internal/controller/workflowrun/controller_validate.go` | Core validation logic (`validateComponentWorkflowRun`, `resolveComponentType`, `formatAllowedWorkflows`) |

### Modified Files

| File | Changes |
|------|---------|
| `internal/controller/workflowrun/controller.go` | Added RBAC markers for `componenttypes` and `clustercomponenttypes`; inserted validation call in `Reconcile()` after `isWorkflowInitiated` check, gated by `!isWorkflowRunning()` |
| `internal/controller/workflowrun/controller_conditions.go` | Added `ReasonComponentValidationFailed` constant, `isWorkflowRunning()` helper, and `setComponentValidationFailedCondition()` setter |
| `internal/controller/workflowrun/controller_validate.go` | Removed empty-kind fallback blocks (dead code since kubebuilder defaults guarantee Kind is populated); simplified `formatAllowedWorkflows` to use `ref.Kind` directly |
| `internal/controller/workflowrun/controller_unit_test.go` | 9 new unit tests for validation logic; added explicit `Kind: WorkflowRefKindClusterWorkflow` to all test objects; changed `WorkflowRunConfig` to `ComponentWorkflowConfig` for Component.Spec.Workflow |
| `internal/controller/workflowrun/controller_integration_test.go` | 5 new integration tests; changed `WorkflowRunConfig` to `ComponentWorkflowConfig`; converted tests from namespace-scoped `Workflow` to cluster-scoped `ClusterWorkflow`; replaced `forceDeleteWorkflow` with `forceDeleteClusterWorkflow` |
| `api/v1alpha1/types.go` | Changed `WorkflowRef.Kind` kubebuilder default from `Workflow` to `ClusterWorkflow` |
| `api/v1alpha1/workflowrun_types.go` | Changed `WorkflowRunConfig.Kind` kubebuilder default from `Workflow` to `ClusterWorkflow` |
| `api/v1alpha1/component_types.go` | Changed `Component.Spec.Workflow` type from `*WorkflowRunConfig` to `*ComponentWorkflowConfig`; added new `ComponentWorkflowConfig` struct (mutable kind/name, no immutability constraints) |
| `internal/controller/reference.go` | Changed `ResolveWorkflow` empty-kind fallback from `WorkflowRefKindWorkflow` to `WorkflowRefKindClusterWorkflow` |
| `internal/controller/reference_test.go` | Updated `TestResolveWorkflow_WithEmptyKind` to expect cluster-scoped resolution |
| `internal/controller/component/controller.go` | Removed 2 empty-kind fallbacks to `WorkflowRefKindWorkflow` |
| `internal/controller/component/controller_watch.go` | Removed empty-kind fallback in `setupWorkflowRefIndex` |
| `internal/controller/component/controller_integration_test.go` | Changed `WorkflowRunConfig` to `ComponentWorkflowConfig` |
| `internal/openchoreo-api/legacyservices/component_service.go` | Changed `WorkflowRunConfig` to `ComponentWorkflowConfig` (2 places) |
| `internal/openchoreo-api/legacyservices/componenttype_service.go` | Removed empty-kind fallback in allowedWorkflows response mapping |
| `internal/openchoreo-api/legacyservices/webhook_service_test.go` | Changed all 12 occurrences of `WorkflowRunConfig` to `ComponentWorkflowConfig` |
| `internal/openchoreo-api/mcphandlers/components.go` | Changed `WorkflowRunConfig` to `ComponentWorkflowConfig` |
| `openapi/openchoreo-api.yaml` | Added `ComponentWorkflowConfig` schema; updated `ComponentSpec.workflow` ref; changed kind defaults to `ClusterWorkflow`; updated allowedWorkflows enum and examples |

## Change Categories

### 1. WorkflowRun Validation (Original Feature)

Validates that component-scoped WorkflowRuns reference workflows allowed by the ComponentType and matching the Component's configured workflow.

### 2. Default Workflow Kind Change: `Workflow` -> `ClusterWorkflow`

Changed the default workflow kind across all CRD types and controllers. `ClusterWorkflow` is the standard pattern since workflows are typically cluster-scoped resources shared across namespaces.

**Files affected:**
- `api/v1alpha1/types.go` — `WorkflowRef.Kind` default
- `api/v1alpha1/workflowrun_types.go` — `WorkflowRunConfig.Kind` default
- `api/v1alpha1/component_types.go` — `ComponentWorkflowConfig.Kind` default
- `internal/controller/reference.go` — `ResolveWorkflow` empty-kind case
- `openapi/openchoreo-api.yaml` — All kind defaults and examples

### 3. Dead-Code Removal: Empty-Kind Fallbacks

Kubebuilder `+kubebuilder:default=` markers guarantee that Kind is always populated by the API server. All `if kind == "" { kind = ... }` fallbacks were dead code and have been removed.

**Files affected:**
- `internal/controller/workflowrun/controller_validate.go` — 4 fallback blocks removed
- `internal/controller/component/controller.go` — 2 fallback blocks removed
- `internal/controller/component/controller_watch.go` — 1 fallback block removed
- `internal/openchoreo-api/legacyservices/componenttype_service.go` — 1 fallback block removed

### 4. New `ComponentWorkflowConfig` Struct

`Component.Spec.Workflow` was previously typed as `*WorkflowRunConfig`, which has immutability constraints (`XValidation:rule="self == oldSelf"`). Components need mutable workflow references (developers should be able to change which workflow a component uses). A new `ComponentWorkflowConfig` struct was created without immutability constraints.

```go
// ComponentWorkflowConfig defines a mutable workflow configuration for a Component.
type ComponentWorkflowConfig struct {
    // +optional
    // +kubebuilder:default=ClusterWorkflow
    Kind WorkflowRefKind `json:"kind,omitempty"`
    // +required
    // +kubebuilder:validation:MinLength=1
    Name string `json:"name"`
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    // +kubebuilder:validation:Type=object
    Parameters *runtime.RawExtension `json:"parameters,omitempty"`
}
```

**Distinction from `WorkflowRunConfig`:** `WorkflowRunConfig` retains immutability markers because once a WorkflowRun is created, its workflow reference must not change.

## Validation Flow

```
Reconcile()
  -> isWorkflowCompleted? -> return (existing)
  -> isWorkflowInitiated? -> set pending, requeue (existing)
  -> isWorkflowRunning? -> skip validation (already executing)
  -> validateComponentWorkflowRun()
      1. Read labels: openchoreo.dev/project, openchoreo.dev/component
      2. Neither label -> standalone, skip validation
      3. Only one label -> permanent failure (ComponentValidationFailed)
      4. Both labels -> fetch Component -> resolve ComponentType -> validate
  -> ResolveWorkflow (existing)
  -> ... rest of reconciliation
```

## Label-Based Scope Detection

| Project Label | Component Label | Scope | Action |
|:---:|:---:|---|---|
| absent | absent | Standalone | Skip validation |
| present | absent | Invalid | Fail permanently |
| absent | present | Invalid | Fail permanently |
| present | present | Component-scoped | Run validation |

## Validation Checks (when both labels present)

### 1. Component Existence
- Fetch Component by `openchoreo.dev/component` label value in the WorkflowRun's namespace
- Not found -> permanent failure (`ComponentValidationFailed`)
- API error -> requeue

### 2. ComponentType Resolution
- Parse `component.spec.componentType.name` (format: `{workloadType}/{name}`)
- Supports both `ComponentType` (namespace-scoped) and `ClusterComponentType` (cluster-scoped)
- For `ClusterComponentType`, converts `ClusterWorkflowRef` to `WorkflowRef` for uniform handling
- Not found -> permanent failure
- API error -> requeue

### 3. AllowedWorkflows Check
- Compare `workflowRun.spec.workflow` (kind + name) against `componentType.spec.allowedWorkflows[]`
- Empty `allowedWorkflows` list -> no workflows allowed, permanent failure
- Workflow not in list -> permanent failure
- Comparison uses composite key: `{kind}:{name}` (kind always populated via kubebuilder defaults)

### 4. Component Workflow Match
- If `component.spec.workflow` is set, verify the WorkflowRun's workflow matches (both kind and name)
- If `component.spec.workflow` is nil -> skip this check (allowedWorkflows check is sufficient)
- Mismatch -> permanent failure

## Skip-When-Running Guard

Validation is skipped once the workflow reaches `Running` phase (`WorkflowRunning` condition is `True`). This prevents failing an in-progress workflow due to a concurrent ComponentType change.

Since validation only runs when `WorkflowRunning` is not `True`, the `setComponentValidationFailedCondition` setter only sets `WorkflowCompleted=True` without explicitly setting `WorkflowRunning=False` (it can never be `True` at that point).

## Error Handling

| Scenario | Behavior | Reason |
|----------|----------|--------|
| Component not found | Permanent failure (`ctrl.Result{}`) | Component should exist if labels are set |
| ComponentType not found | Permanent failure | Component references a missing type |
| API/network error fetching Component | Requeue (`ctrl.Result{Requeue: true}`) | Transient |
| API/network error fetching ComponentType | Requeue | Transient |
| Workflow not in allowedWorkflows | Permanent failure | Configuration mismatch |
| Workflow doesn't match component's workflow | Permanent failure | Configuration mismatch |
| Invalid componentType name format | Requeue | Unexpected state |

## RBAC

Added kubebuilder RBAC markers for read access to ComponentType and ClusterComponentType:
```go
// +kubebuilder:rbac:groups=openchoreo.dev,resources=componenttypes,verbs=get;list;watch
// +kubebuilder:rbac:groups=openchoreo.dev,resources=clustercomponenttypes,verbs=get;list;watch
```

The controller already had RBAC for Components.

## Test Coverage

### Unit Tests (`controller_unit_test.go`)

| Test | Validates |
|------|-----------|
| standalone workflow run (no labels) skips validation | Neither label -> no validation |
| only project label present fails | Single label -> permanent failure |
| only component label present fails | Single label -> permanent failure |
| component not found fails permanently | Missing component -> permanent failure |
| workflow not in allowedWorkflows fails permanently | Disallowed workflow -> permanent failure |
| workflow does not match component configured workflow | Workflow mismatch -> permanent failure |
| valid component workflow run passes | All checks pass -> no shouldReturn |
| empty allowedWorkflows rejects any workflow | Empty list -> permanent failure |
| component with nil workflow skips workflow match check | Nil workflow on component -> skip match, pass |

### Integration Tests (`controller_integration_test.go`)

| Test | Validates |
|------|-----------|
| WorkflowRun with only project label | Full reconcile -> `ComponentValidationFailed` condition |
| WorkflowRun with only component label | Full reconcile -> `ComponentValidationFailed` condition |
| WorkflowRun with workflow not in allowedWorkflows | Full reconcile with Component + ComponentType -> `ComponentValidationFailed` |
| WorkflowRun with valid component workflow | Passes validation, proceeds to workflow resolution |
| Standalone WorkflowRun (no labels) | Skips validation, proceeds normally |

### Test Adjustments for Default Kind Change

- All unit tests explicitly set `Kind: WorkflowRefKindClusterWorkflow` since fake clients don't apply kubebuilder defaults
- Integration tests use `ClusterWorkflow` objects (envtest applies kubebuilder defaults, so Kind defaults to `ClusterWorkflow`)
- Reference tests updated to expect cluster-scoped resolution for empty kind

## Pending Generation Steps

After these changes, the following must be run before the code is fully ready:
```bash
make generate manifests   # Regenerate deepcopy methods and CRD manifests
make openapi-codegen      # Regenerate Go models from updated OpenAPI spec
```
