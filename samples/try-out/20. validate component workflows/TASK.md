# Validate Component Workflows in WorkflowRun Controller

## Context

WorkflowRuns can be either standalone (namespace-level) or component-scoped. Component-scoped WorkflowRuns are identified by the presence of labels `openchoreo.dev/project` and `openchoreo.dev/component`. Currently the WorkflowRun controller (`internal/controller/workflowrun/controller.go`) does not validate whether the referenced workflow is actually allowed for the component's type, or matches the component's configured workflow. This validation already exists in the Component controller (`internal/controller/component/`), but must also be enforced at the WorkflowRun level to prevent bypasses (e.g., WorkflowRuns created directly via `occ apply` or the API).

## Key Types and Files

| Concept | File | Key Fields |
|---------|------|------------|
| WorkflowRun reconciler | `internal/controller/workflowrun/controller.go` | `Reconcile()` — main loop |
| WorkflowRun conditions | `internal/controller/workflowrun/controller_conditions.go` | `isWorkflowCompleted()`, `isWorkflowInitiated()`, condition setters |
| WorkflowRun CRD | `api/v1alpha1/workflowrun_types.go` | `spec.workflow.kind`, `spec.workflow.name` |
| Component CRD | `api/v1alpha1/component_types.go` | `spec.workflow` (WorkflowRunConfig), `spec.componentType` |
| ComponentType CRD | `api/v1alpha1/componenttype_types.go` | `spec.allowedWorkflows` ([]WorkflowRef) |
| Label constants | `internal/labels/labels.go` | `LabelKeyProjectName`, `LabelKeyComponentName` |
| WorkflowRef types | `api/v1alpha1/types.go` | `WorkflowRef{Kind, Name}`, `WorkflowRefKind` |
| Reference resolution | `internal/controller/reference.go` | `ResolveWorkflow()`, `ResolveComponentType()` |

## Label Convention

- `openchoreo.dev/project` (`LabelKeyProjectName`) — project name
- `openchoreo.dev/component` (`LabelKeyComponentName`) — component name

These labels are set when the component controller (or API/CLI) creates WorkflowRuns for components. See `internal/controller/component/controller_integration_test.go:1059-1061` for the pattern.

## Requirements

### 0. Determine WorkflowRun Scope from Labels

Read labels from the WorkflowRun:
- **Both** `project` AND `component` labels present → **component workflow run** → proceed to validation (step 3)
- **Neither** label present → **standalone workflow run** → skip validation, follow existing reconciliation logic
- **Only one** label present → **invalid** → fail the WorkflowRun immediately (step 1)

### 1. Fail WorkflowRun When Only One Label Is Present

If the WorkflowRun has `openchoreo.dev/project` but NOT `openchoreo.dev/component` (or vice versa), mark it as failed:
- Set `WorkflowCompleted=True` with reason `WorkflowFailed`
- Use a descriptive message, e.g.: `"component workflow run must have both openchoreo.dev/project and openchoreo.dev/component labels"`
- Return `ctrl.Result{}, nil` (no requeue — permanent failure)

**Follow the existing condition-setting pattern** in `controller_conditions.go`. Consider adding a new reason constant (e.g., `ReasonInvalidLabels` or `ReasonValidationFailed`) or reuse `WorkflowFailed` with a descriptive message.

### 2. Skip Validation for Standalone WorkflowRuns

If neither label is present, continue with the current reconciliation logic unchanged. No new code needed for this path.

### 3. Validate Component Workflow Runs (Both Labels Present)

When both labels are present, perform the following validations **before** resolving the workflow plane or rendering. The ideal insertion point is after the `isWorkflowInitiated` check (line ~113) and before `ResolveWorkflow` (line ~116) in `controller.go`.

#### 3.1 Validate Workflow Against ComponentType's AllowedWorkflows

1. Fetch the Component by name (from `openchoreo.dev/component` label) in the WorkflowRun's namespace
2. If Component not found → fail with reason and message (e.g., `"component %q not found"`)
3. Resolve the Component's ComponentType via `spec.componentType` (kind + name)
4. If ComponentType not found → fail with reason and message
5. Check that the WorkflowRun's `spec.workflow` (kind + name) is in the ComponentType's `spec.allowedWorkflows[]`
   - Compare both `kind` and `name` fields. Default kind is `"Workflow"` for WorkflowRef (namespace-scoped)
   - Note: ComponentType.allowedWorkflows uses `WorkflowRef` (kind defaults to `Workflow`), while WorkflowRun uses `WorkflowRunConfig` (kind is `WorkflowRefKind`). The kind values are compatible
6. If not in allowed list → fail with message like `"workflow %s/%s is not allowed for component type %q; allowed: %v"`

**Existing pattern reference:** The Component controller already does this validation — see `internal/controller/component/controller_integration_test.go:400-435` for the test pattern and `ReasonWorkflowNotAllowed` in `controller_conditions.go`.

#### 3.2 Validate Workflow Matches Component's Configured Workflow

1. Read the Component's `spec.workflow` field
2. If the Component has `spec.workflow` set, verify the WorkflowRun's `spec.workflow.kind` and `spec.workflow.name` match
   - `Component.spec.workflow` is a `*WorkflowRunConfig` with fields `Kind` and `Name`
   - `WorkflowRun.spec.workflow` is a `WorkflowRunConfig` with the same structure
3. If they don't match → fail with message like `"workflow run references workflow %s/%s but component %q is configured with workflow %s/%s"`
4. If the Component's `spec.workflow` is nil/empty, skip this sub-check (the 3.1 allowedWorkflows check is sufficient)

#### 3.3 Skip Validation Once Running

This validation should only run while the WorkflowRun is in a pre-running state. Once the Argo Workflow reaches `Running` phase (i.e., `WorkflowRunning` condition is `True`), skip validation on subsequent reconciliations.

**Check:** Use `meta.IsStatusConditionTrue(workflowRun.Status.Conditions, string(ConditionWorkflowRunning))` to determine if already running.

**Why:** Once the workflow is executing, failing it due to a concurrent ComponentType change would be disruptive. The validation is a gate before execution begins.

## Implementation Notes

### Where to Add Validation

Add a new function (e.g., `validateComponentWorkflowRun`) called early in the `Reconcile` method, after the `isWorkflowInitiated` check but before `ResolveWorkflow`. Suggested location: between lines 113 and 115 in `controller.go`.

```
// Existing flow:
// 1. Fetch WorkflowRun
// 2. Handle deletion
// 3. Ensure finalizer
// 4. Check TTL
// 5. Check completion
// 6. Check initiation
// → NEW: Validate component workflow (if component labels present)
// 7. Resolve Workflow
// 8. Resolve WorkflowPlane
// ... rest of reconciliation
```

### RBAC

The controller already has RBAC for reading Components (`+kubebuilder:rbac:groups=openchoreo.dev,resources=components,verbs=get;list;watch` at line 49). You may need to add RBAC for ComponentType/ClusterComponentType if not already present. Check `controller.go` RBAC markers.

### New Condition Reason

Add a new reason constant in `controller_conditions.go`, e.g.:
```go
ReasonComponentValidationFailed controller.ConditionReason = "ComponentValidationFailed"
```

And a corresponding condition setter function that marks the workflow as permanently failed (sets `WorkflowCompleted=True` with `WorkflowFailed` reason).

### Error Handling Pattern

Follow the existing pattern: validation failures are **permanent** (return `ctrl.Result{}, nil`), not transient. Only API/network errors should trigger requeue. See how `setWorkflowNotFoundCondition` handles permanent failure vs `setWorkflowResolutionFailedCondition` handles transient failure.

However, consider: if the Component or ComponentType is not found, is that permanent or transient? The Component may not have been created yet. Consider:
- Component not found → could be transient (requeue with backoff) if the WorkflowRun was created before the Component
- ComponentType not found → likely permanent if the Component exists but references a missing type
- Workflow not in allowedWorkflows → permanent failure
- Workflow doesn't match Component's workflow → permanent failure

### Tests

Add tests in `internal/controller/workflowrun/`:
1. **Unit test** (`controller_unit_test.go`): Test the validation function in isolation
2. **Integration test** (`controller_integration_test.go`): Test full reconciliation with envtest:
   - WorkflowRun with only project label → fails
   - WorkflowRun with only component label → fails
   - WorkflowRun with both labels, workflow not in allowedWorkflows → fails
   - WorkflowRun with both labels, workflow doesn't match component's workflow → fails
   - WorkflowRun with both labels, all valid → proceeds normally
   - WorkflowRun with neither label → proceeds normally (existing behavior)
   - WorkflowRun already running → skips validation

### ClusterComponentType Consideration

If the Component references a `ClusterComponentType` (via `spec.componentType.kind`), you need to resolve it differently (cluster-scoped). Use the existing `controller.ResolveComponentType()` helper or look at how the Component controller resolves it. The `allowedWorkflows` field on `ClusterComponentType` uses `ClusterWorkflowRef` (only `ClusterWorkflow` kind), so the comparison logic must account for this.
