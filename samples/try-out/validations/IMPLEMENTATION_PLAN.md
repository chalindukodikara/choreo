# Implementation Plan: Validate ComponentWorkflow References Against allowedWorkflows

## Context

This implementation adds validation to ensure that ComponentWorkflow references in Components and ComponentWorkflowRuns are restricted to the `allowedWorkflows` list defined in the ComponentType. This provides Platform Engineers with governance control over which build workflows developers can use for different component types.

**Why this is needed:**
- Platform Engineers define ComponentType templates with specific allowed workflows via `spec.allowedWorkflows`
- Currently, there's no validation to enforce this restriction
- Developers could reference any ComponentWorkflow, even if not intended for that ComponentType
- This could lead to incorrect builds, security issues, or misconfigured components

**Intended outcome:**
- Component controller validates workflow references during reconciliation
- ComponentWorkflowRun controller validates before executing workflows
- Clear error messages guide developers to use allowed workflows
- Automatic re-validation when ComponentType allowedWorkflows or ComponentWorkflow resources change

## Architecture Overview

### Current Structure

**Component:**
- `Component.spec.workflow` (type: `*ComponentWorkflowRunConfig`) - optional field for build configuration
- `Component.spec.workflow.name` - references the ComponentWorkflow CR name
- `Component.spec.componentType` - references ComponentType (format: `{workloadType}/{ctName}`)

**ComponentType:**
- `ComponentType.spec.allowedWorkflows` - `[]string` of allowed ComponentWorkflow names
- If empty, NO workflows are allowed (explicit deny-by-default)
- Similar pattern to existing `spec.allowedTraits`

**ComponentWorkflowRun:**
- `ComponentWorkflowRun.spec.workflow.name` - references the ComponentWorkflow CR name
- `ComponentWorkflowRun.spec.owner.componentName` - references owning Component

### Validation Flow

```
Component Reconciliation:
├── Fetch & Validate ComponentType
├── Validate Workload
├── Validate Traits (existing)
├── ✨ Validate ComponentWorkflow (NEW)
│   ├── Check if workflow in allowedWorkflows (fast path)
│   └── Check if ComponentWorkflow exists (only if allowed)
├── Fetch DeploymentPipeline
└── Handle AutoDeploy

ComponentWorkflowRun Reconciliation:
├── Fetch ComponentWorkflowRun
├── Handle Deletion
├── Check if already completed
├── Get BuildPlane
├── ✨ Validate ComponentWorkflow is allowed (NEW)
│   ├── Fetch owning Component
│   ├── Fetch ComponentType
│   └── Check if workflow in allowedWorkflows
├── Fetch ComponentWorkflow
├── Render workflow template
└── Execute workflow in BuildPlane
```

## Implementation Details

### Part 1: Component Controller - Validation Logic

**File:** `internal/controller/component/controller.go`

#### 1.1 New Validation Function

Add after `areValidTraits()` function (around line 332):

```go
// validateComponentWorkflow validates that the referenced ComponentWorkflow exists
// and is in the allowedWorkflows list of the ComponentType.
// Returns the ComponentWorkflow on success, or nil with no error if validation failed
// (condition already set).
func (r *Reconciler) validateComponentWorkflow(
    ctx context.Context,
    comp *openchoreov1alpha1.Component,
    ct *openchoreov1alpha1.ComponentType,
) (*openchoreov1alpha1.ComponentWorkflow, error) {
    logger := log.FromContext(ctx)

    // If no workflow is specified, validation passes (workflows are optional)
    if comp.Spec.Workflow == nil {
        return nil, nil
    }

    workflowName := comp.Spec.Workflow.Name

    // Performance optimization: Check allowedWorkflows list first
    // This avoids fetching the ComponentWorkflow if it's not allowed
    if len(ct.Spec.AllowedWorkflows) > 0 {
        allowedSet := make(map[string]bool, len(ct.Spec.AllowedWorkflows))
        for _, name := range ct.Spec.AllowedWorkflows {
            allowedSet[name] = true
        }

        if !allowedSet[workflowName] {
            msg := fmt.Sprintf("ComponentWorkflow %q is not allowed by ComponentType %q; allowed workflows: %v",
                workflowName, ct.Name, ct.Spec.AllowedWorkflows)
            controller.MarkFalseCondition(comp, ConditionReady, ReasonComponentWorkflowNotAllowed, msg)
            logger.Info(msg, "component", comp.Name)
            return nil, nil
        }
    } else {
        // If allowedWorkflows is empty, no workflows are allowed
        msg := fmt.Sprintf("No ComponentWorkflows are allowed by ComponentType %q, but component specifies workflow %q",
            ct.Name, workflowName)
        controller.MarkFalseCondition(comp, ConditionReady, ReasonComponentWorkflowNotAllowed, msg)
        logger.Info(msg, "component", comp.Name)
        return nil, nil
    }

    // Now check if the ComponentWorkflow actually exists
    workflow := &openchoreov1alpha1.ComponentWorkflow{}
    if err := r.Get(ctx, types.NamespacedName{
        Name:      workflowName,
        Namespace: comp.Namespace,
    }, workflow); err != nil {
        if apierrors.IsNotFound(err) {
            msg := fmt.Sprintf("ComponentWorkflow %q not found", workflowName)
            controller.MarkFalseCondition(comp, ConditionReady, ReasonComponentWorkflowNotFound, msg)
            logger.Info(msg, "component", comp.Name)
            return nil, nil
        }
        logger.Error(err, "Failed to fetch ComponentWorkflow", "name", workflowName)
        return nil, err
    }

    return workflow, nil
}
```

**Key Design Decisions:**
- Returns `(nil, nil)` for validation failures (non-retryable configuration errors)
- Returns `(nil, err)` for transient system errors (will retry)
- Checks allowedWorkflows list BEFORE fetching ComponentWorkflow (performance optimization)
- Follows existing pattern from `areValidTraits()`

#### 1.2 Integration into Reconciliation Flow

In `reconcileWithComponentType()`, add validation call after trait validation (around line 140):

```go
// Validate traits
if !r.areValidTraits(ctx, comp, ct) {
    // Validation failed, condition already set
    return ctrl.Result{}, nil
}

// Validate ComponentWorkflow (if specified)
componentWorkflow, err := r.validateComponentWorkflow(ctx, comp, ct)
if err != nil {
    return ctrl.Result{}, err
}
if componentWorkflow == nil && comp.Spec.Workflow != nil {
    // Validation failed, condition already set
    return ctrl.Result{}, nil
}

// Continue with existing logic...
```

### Part 2: Component Controller - Condition Reasons

**File:** `internal/controller/component/controller_conditions.go`

Add new condition reasons (around line 50):

```go
// ReasonComponentWorkflowNotAllowed indicates the referenced ComponentWorkflow is not in allowedWorkflows
ReasonComponentWorkflowNotAllowed controller.ConditionReason = "ComponentWorkflowNotAllowed"

// ReasonComponentWorkflowNotFound indicates the referenced ComponentWorkflow doesn't exist
ReasonComponentWorkflowNotFound controller.ConditionReason = "ComponentWorkflowNotFound"
```

### Part 3: Component Controller - Watch Configuration

**File:** `internal/controller/component/controller_watch.go`

#### 3.1 Add Workflow Index Constant

Add to constants section (around line 29):

```go
// workflowIndex is the field index name for workflow reference
workflowIndex = "spec.workflow.name"
```

#### 3.2 Add Workflow Index Setup Function

Add after `setupTraitsRefIndex()`:

```go
// setupWorkflowRefIndex sets up the field index for workflow references
func (r *Reconciler) setupWorkflowRefIndex(ctx context.Context, mgr ctrl.Manager) error {
    return mgr.GetFieldIndexer().IndexField(ctx, &openchoreov1alpha1.Component{},
        workflowIndex, func(obj client.Object) []string {
            comp := obj.(*openchoreov1alpha1.Component)
            if comp.Spec.Workflow == nil || comp.Spec.Workflow.Name == "" {
                return []string{}
            }
            return []string{comp.Spec.Workflow.Name}
        })
}
```

#### 3.3 Add Watch Handler for ComponentWorkflow

Add after `listComponentsUsingTrait()`:

```go
// listComponentsForComponentWorkflow returns reconcile requests for all Components using this ComponentWorkflow
func (r *Reconciler) listComponentsForComponentWorkflow(ctx context.Context, obj client.Object) []reconcile.Request {
    workflow := obj.(*openchoreov1alpha1.ComponentWorkflow)

    var components openchoreov1alpha1.ComponentList
    if err := r.List(ctx, &components,
        client.InNamespace(workflow.Namespace),
        client.MatchingFields{workflowIndex: workflow.Name}); err != nil {
        logger := ctrl.LoggerFrom(ctx)
        logger.Error(err, "Failed to list components for ComponentWorkflow", "workflow", workflow.Name)
        return nil
    }

    requests := make([]reconcile.Request, len(components.Items))
    for i, comp := range components.Items {
        requests[i] = reconcile.Request{
            NamespacedName: types.NamespacedName{
                Name:      comp.Name,
                Namespace: comp.Namespace,
            },
        }
    }
    return requests
}
```

### Part 4: Component Controller - SetupWithManager Updates

**File:** `internal/controller/component/controller.go`

#### 4.1 Add Workflow Index Setup

In `SetupWithManager()`, add after traits index setup (around line 773):

```go
if err := r.setupWorkflowRefIndex(ctx, mgr); err != nil {
    return fmt.Errorf("failed to setup workflow reference index: %w", err)
}
```

#### 4.2 Add ComponentWorkflow Watch

In the controller builder chain (around line 807), add:

```go
Watches(&openchoreov1alpha1.ComponentWorkflow{},
    handler.EnqueueRequestsFromMapFunc(r.listComponentsForComponentWorkflow)).
```

**Complete builder chain:**

```go
return ctrl.NewControllerManagedBy(mgr).
    For(&openchoreov1alpha1.Component{}).
    Watches(&openchoreov1alpha1.ComponentRelease{},
        handler.EnqueueRequestsFromMapFunc(r.findComponentsForComponentRelease)).
    Watches(&openchoreov1alpha1.ReleaseBinding{},
        handler.EnqueueRequestsFromMapFunc(r.findComponentsForReleaseBinding)).
    Watches(&openchoreov1alpha1.ComponentWorkflowRun{},
        handler.EnqueueRequestsFromMapFunc(r.findComponentsForComponentWorkflowRun)).
    Watches(&openchoreov1alpha1.ComponentType{},
        handler.EnqueueRequestsFromMapFunc(r.listComponentsForComponentType)).
    Watches(&openchoreov1alpha1.Trait{},
        handler.EnqueueRequestsFromMapFunc(r.listComponentsUsingTrait)).
    Watches(&openchoreov1alpha1.ComponentWorkflow{},
        handler.EnqueueRequestsFromMapFunc(r.listComponentsForComponentWorkflow)).
    Watches(&openchoreov1alpha1.Workload{},
        handler.EnqueueRequestsFromMapFunc(r.listComponentsForWorkload)).
    // ... rest of watches
    Named("component").
    Complete(r)
```

### Part 5: ComponentWorkflowRun Controller - Validation Logic

**File:** `internal/controller/componentworkflowrun/controller.go`

#### 5.1 Add Validation Function

Add new validation function near the top of the file:

```go
// validateComponentWorkflowAllowed fetches the Component and ComponentType to validate
// that the referenced ComponentWorkflow is in the allowedWorkflows list.
// Returns (true, nil) if validation passes.
// Returns (false, nil) if validation fails (terminal error, condition set, don't requeue).
// Returns (false, err) if there's a transient error fetching resources (should requeue).
func (r *Reconciler) validateComponentWorkflowAllowed(
    ctx context.Context,
    componentWorkflowRun *openchoreodevv1alpha1.ComponentWorkflowRun,
    workflowName string,
) (bool, error) {
    logger := log.FromContext(ctx)

    // Fetch the owning Component
    component := &openchoreodevv1alpha1.Component{}
    err := r.Get(ctx, types.NamespacedName{
        Name:      componentWorkflowRun.Spec.Owner.ComponentName,
        Namespace: componentWorkflowRun.Namespace,
    }, component)
    if err != nil {
        if apierrors.IsNotFound(err) {
            msg := fmt.Sprintf("Component %q not found", componentWorkflowRun.Spec.Owner.ComponentName)
            setWorkflowNotAllowedCondition(componentWorkflowRun, msg)
            logger.Info(msg, "workflowrun", componentWorkflowRun.Name)
            return false, nil
        }
        // Transient error - requeue
        return false, err
    }

    // Parse and fetch ComponentType
    workloadType, ctName, err := parseComponentType(component.Spec.ComponentType)
    if err != nil {
        msg := fmt.Sprintf("Invalid componentType format in Component %q: %v",
            component.Name, err)
        setWorkflowNotAllowedCondition(componentWorkflowRun, msg)
        logger.Error(err, "Failed to parse componentType")
        return false, nil
    }

    componentType := &openchoreodevv1alpha1.ComponentType{}
    err = r.Get(ctx, types.NamespacedName{
        Name:      ctName,
        Namespace: componentWorkflowRun.Namespace,
    }, componentType)
    if err != nil {
        if apierrors.IsNotFound(err) {
            msg := fmt.Sprintf("ComponentType %q not found for Component %q",
                ctName, component.Name)
            setWorkflowNotAllowedCondition(componentWorkflowRun, msg)
            logger.Info(msg, "workflowrun", componentWorkflowRun.Name)
            return false, nil
        }
        // Transient error - requeue
        return false, err
    }

    // Validate workloadType matches
    if componentType.Spec.WorkloadType != openchoreodevv1alpha1.WorkloadType(workloadType) {
        msg := fmt.Sprintf("WorkloadType mismatch: Component specifies %s but ComponentType has %s",
            workloadType, componentType.Spec.WorkloadType)
        setWorkflowNotAllowedCondition(componentWorkflowRun, msg)
        logger.Error(fmt.Errorf("%s", msg), "WorkloadType mismatch")
        return false, nil
    }

    // Check if workflow is in allowedWorkflows list
    if len(componentType.Spec.AllowedWorkflows) > 0 {
        allowedSet := make(map[string]bool, len(componentType.Spec.AllowedWorkflows))
        for _, name := range componentType.Spec.AllowedWorkflows {
            allowedSet[name] = true
        }

        if !allowedSet[workflowName] {
            msg := fmt.Sprintf("ComponentWorkflow %q is not allowed by ComponentType %q; allowed workflows: %v",
                workflowName, componentType.Name, componentType.Spec.AllowedWorkflows)
            setWorkflowNotAllowedCondition(componentWorkflowRun, msg)
            logger.Info(msg, "workflowrun", componentWorkflowRun.Name)
            return false, nil
        }
    } else {
        // If allowedWorkflows is empty, no workflows are allowed
        msg := fmt.Sprintf("No ComponentWorkflows are allowed by ComponentType %q",
            componentType.Name)
        setWorkflowNotAllowedCondition(componentWorkflowRun, msg)
        logger.Info(msg, "workflowrun", componentWorkflowRun.Name)
        return false, nil
    }

    return true, nil
}

// parseComponentType parses the componentType format: {workloadType}/{componentTypeName}
func parseComponentType(componentType string) (workloadType string, ctName string, err error) {
    parts := strings.SplitN(componentType, "/", 2)
    if len(parts) != 2 {
        return "", "", fmt.Errorf("invalid componentType format: expected {workloadType}/{name}, got %s", componentType)
    }
    return parts[0], parts[1], nil
}
```

**Key Design Decisions:**
- Fetches Component and ComponentType to get allowedWorkflows context
- Returns `(false, nil)` for terminal validation failures (don't requeue)
- Returns `(false, err)` for transient errors (will requeue)
- Sets terminal `WorkflowFailed` condition to prevent further execution

#### 5.2 Add Validation Call in Reconcile

In `Reconcile()`, add validation after BuildPlane client is obtained but BEFORE fetching ComponentWorkflow (replace lines 145-153):

```go
// Extract workflow name for validation
workflowName := componentWorkflowRun.Spec.Workflow.Name

// Validate that the ComponentWorkflow is allowed by the ComponentType
allowed, err := r.validateComponentWorkflowAllowed(ctx, componentWorkflowRun, workflowName)
if err != nil {
    // Transient error - requeue
    logger.Error(err, "failed to validate ComponentWorkflow allowance",
        "workflowrun", componentWorkflowRun.Name)
    return ctrl.Result{Requeue: true}, nil
}
if !allowed {
    // Terminal validation error - condition already set, don't requeue
    return ctrl.Result{}, nil
}

// Fetch ComponentWorkflow (now we know it's allowed)
componentWorkflow := &openchoreodevv1alpha1.ComponentWorkflow{}
if err := r.Get(ctx, types.NamespacedName{
    Name:      workflowName,
    Namespace: componentWorkflowRun.Namespace,
}, componentWorkflow); err != nil {
    logger.Error(err, "failed to get ComponentWorkflow",
        "workflow", workflowName)
    return ctrl.Result{Requeue: true}, nil
}
```

### Part 6: ComponentWorkflowRun Controller - Conditions

**File:** `internal/controller/componentworkflowrun/controller_conditions.go`

#### 6.1 Add Condition Reason

Add new reason constant:

```go
ReasonWorkflowNotAllowed controller.ConditionReason = "WorkflowNotAllowed"
```

#### 6.2 Add Condition Setter Function

Add new helper function:

```go
func setWorkflowNotAllowedCondition(componentWorkflowRun *openchoreodevv1alpha1.ComponentWorkflowRun, message string) {
    meta.SetStatusCondition(&componentWorkflowRun.Status.Conditions, metav1.Condition{
        Type:               string(ConditionWorkflowFailed),
        Status:             metav1.ConditionTrue,
        Reason:             string(ReasonWorkflowNotAllowed),
        Message:            message,
        ObservedGeneration: componentWorkflowRun.Generation,
    })
    meta.SetStatusCondition(&componentWorkflowRun.Status.Conditions, metav1.Condition{
        Type:               string(ConditionWorkflowCompleted),
        Status:             metav1.ConditionTrue,
        Reason:             string(ReasonWorkflowNotAllowed),
        Message:            "Workflow is not allowed by ComponentType",
        ObservedGeneration: componentWorkflowRun.Generation,
    })
}
```

**Why both conditions:**
- `WorkflowFailed=True`: Indicates the workflow failed validation
- `WorkflowCompleted=True`: Marks this as a terminal state (prevents further reconciliation)

### Part 7: RBAC Updates

**File:** `internal/controller/componentworkflowrun/controller.go`

Add RBAC annotation for accessing Components and ComponentTypes:

```go
// +kubebuilder:rbac:groups=openchoreo.dev,resources=componenttypes,verbs=get;list;watch
```

**Note:** Component RBAC permissions already exist, so no additional annotation needed for Component access.

## Critical Files to Modify

1. **`internal/controller/component/controller.go`**
   - Add `validateComponentWorkflow()` function
   - Integrate validation into `reconcileWithComponentType()`
   - Update `SetupWithManager()` to add index and watch

2. **`internal/controller/component/controller_conditions.go`**
   - Add `ReasonComponentWorkflowNotAllowed` and `ReasonComponentWorkflowNotFound`

3. **`internal/controller/component/controller_watch.go`**
   - Add `workflowIndex` constant
   - Add `setupWorkflowRefIndex()` function
   - Add `listComponentsForComponentWorkflow()` handler

4. **`internal/controller/componentworkflowrun/controller.go`**
   - Add `validateComponentWorkflowAllowed()` function
   - Add `parseComponentType()` helper
   - Integrate validation into `Reconcile()`
   - Add RBAC annotation

5. **`internal/controller/componentworkflowrun/controller_conditions.go`**
   - Add `ReasonWorkflowNotAllowed` constant
   - Add `setWorkflowNotAllowedCondition()` function

## Testing Strategy

### Unit Tests

**Component Controller** (`internal/controller/component/controller_test.go`):

1. ✅ `Test_ValidateComponentWorkflow_WorkflowAllowed` - Happy path
2. ✅ `Test_ValidateComponentWorkflow_WorkflowNotInAllowedList` - Validation failure
3. ✅ `Test_ValidateComponentWorkflow_EmptyAllowedWorkflows` - No workflows allowed
4. ✅ `Test_ValidateComponentWorkflow_NoWorkflowSpecified` - Optional workflow (nil)
5. ✅ `Test_ValidateComponentWorkflow_WorkflowNotFound` - ComponentWorkflow doesn't exist

**ComponentWorkflowRun Controller** (`internal/controller/componentworkflowrun/controller_test.go`):

1. ✅ `Test_ValidateWorkflowAllowed_Success` - Happy path
2. ✅ `Test_ValidateWorkflowAllowed_NotInAllowedList` - Validation failure
3. ✅ `Test_ValidateWorkflowAllowed_ComponentNotFound` - Component missing
4. ✅ `Test_ValidateWorkflowAllowed_ComponentTypeNotFound` - ComponentType missing
5. ✅ `Test_ValidateWorkflowAllowed_EmptyAllowedWorkflows` - No workflows allowed

### Integration Tests

1. **ComponentWorkflow Watch Integration:**
   - Create Component with workflow X
   - Create ComponentType with workflow X in allowedWorkflows
   - Verify Component is Ready
   - Update ComponentType to remove workflow X
   - Verify Component reconciles and sets error condition

2. **ComponentType allowedWorkflows Update:**
   - Create Component with workflow X (ComponentType has empty allowedWorkflows)
   - Verify Component has error condition
   - Update ComponentType to add workflow X
   - Verify Component becomes Ready

3. **ComponentWorkflowRun Validation:**
   - Create ComponentWorkflowRun with workflow X
   - ComponentType doesn't allow workflow X
   - Verify ComponentWorkflowRun fails with terminal condition
   - Verify no workflow resources created in BuildPlane

### Manual Testing Scenarios

```bash
# Scenario 1: Developer tries to use disallowed workflow
kubectl apply -f - <<EOF
apiVersion: openchoreo.dev/v1alpha1
kind: ComponentType
metadata:
  name: web-service
spec:
  workloadType: deployment
  allowedWorkflows:
    - docker-build
    - buildpacks-build
---
apiVersion: openchoreo.dev/v1alpha1
kind: Component
metadata:
  name: my-service
spec:
  componentType: deployment/web-service
  workflow:
    name: nodejs-custom-build  # NOT in allowedWorkflows
EOF

# Expected: Component status shows error condition
kubectl get component my-service -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'
# Should show: status=False, reason=ComponentWorkflowNotAllowed

# Scenario 2: Platform Engineer updates allowedWorkflows
kubectl patch componenttype web-service --type='json' -p='[
  {"op": "add", "path": "/spec/allowedWorkflows/-", "value": "nodejs-custom-build"}
]'

# Expected: Component automatically reconciles and becomes Ready
kubectl get component my-service -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'
# Should show: status=True
```

## Edge Cases and Error Scenarios

| Edge Case | Behavior |
|-----------|----------|
| `Component.spec.workflow` is `nil` | ✅ Validation passes (workflows are optional) |
| `ComponentType.spec.allowedWorkflows` is `[]` (empty) | ❌ Reject any Component with non-nil workflow |
| ComponentWorkflow deleted while ComponentWorkflowRun executing | ⚠️ Current run completes, new runs will fail validation |
| ComponentType.allowedWorkflows updated to remove active workflow | ⚠️ Component sets error condition, existing runs complete, new runs fail |
| Multiple Components use same ComponentWorkflow | ✅ All reconcile independently when workflow changes |

## Performance Optimizations

1. **Check allowedWorkflows before fetching ComponentWorkflow**
   - Avoids unnecessary API call if workflow already known to be disallowed
   - Implemented in both Component and ComponentWorkflowRun controllers

2. **Use field indexes for efficient watches**
   - `workflowIndex` on Component enables O(1) lookup instead of O(n) list scan
   - Scales efficiently even with thousands of Components

3. **Terminal vs Transient error handling**
   - Don't requeue validation failures (configuration errors)
   - Only requeue on system errors (API server issues)

## Migration and Rollout

### Phase 1: Implementation (Non-Breaking)
- Add validation logic to both controllers
- Existing Components without workflows continue to work
- Only affects new validations for Components with workflows

### Phase 2: Watch Integration (Enhanced Reactivity)
- Add ComponentWorkflow watch to Component controller
- Components automatically revalidate on workflow/type changes

### Phase 3: Documentation
- Update ComponentType examples to show allowedWorkflows usage
- Document error messages and troubleshooting steps

### Rollback Plan
- Validation can be disabled by commenting out validation calls
- Watches can be removed from SetupWithManager
- No CRD schema changes, so rollback is safe

## Code Generation

After implementation, run:

```bash
make code.gen
```

This regenerates:
- DeepCopy methods (if any type changes)
- RBAC manifests (for new RBAC annotations)
- CRD YAML files

## Verification

After implementation, verify:

1. **Component validation works:**
   ```bash
   # Component with disallowed workflow should fail
   kubectl get component -o json | jq '.items[] | select(.status.conditions[]?.reason == "ComponentWorkflowNotAllowed")'
   ```

2. **ComponentWorkflowRun validation works:**
   ```bash
   # WorkflowRun with disallowed workflow should fail
   kubectl get componentworkflowrun -o json | jq '.items[] | select(.status.conditions[]?.reason == "WorkflowNotAllowed")'
   ```

3. **Watches trigger reconciliation:**
   ```bash
   # Update ComponentType allowedWorkflows and verify Component status changes
   kubectl patch componenttype <name> ...
   kubectl get component <name> -w
   ```

4. **Field index is working:**
   ```bash
   # Controller logs should show efficient lookups, not full list scans
   kubectl logs -n openchoreo-system deployment/openchoreo-controller-manager | grep "listComponentsForComponentWorkflow"
   ```
