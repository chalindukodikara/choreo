## Problem

Currently, there is no abstraction layer for OpenChoreo workflows. Argo Workflows is hardcoded as the only supported CI engine. The tight coupling exists in two places:

1. **RunTemplate** — `Workflow.spec.runTemplate` must contain an Argo Workflow CR. No other K8s-native engine (Tekton, etc.) can be used.
2. **Status sync** — `WorkflowRun` controller directly reads Argo Workflow node status (`syncWorkflowRunStatus`) and maps Argo-specific phases/node types to `WorkflowTask`. This is ~60 lines of Argo-specific code.

Beyond K8s-native engines, there is no path to support API-based CI systems (Azure Pipelines, GitHub Actions, Jenkins, CircleCI) that are triggered via HTTP calls rather than K8s resource creation.

## Requirements

1. Different CI engines should be pluggable as OpenChoreo modules.
2. Must support K8s-native engines: Argo Workflows, Tekton Pipelines.
3. Must support API-based engines: Azure Pipelines, GitHub Actions, Jenkins, CircleCI.
4. Must preserve backward compatibility with existing Argo Workflow-based `ClusterWorkflow` definitions.
5. Status from any engine must be normalized into the existing vendor-neutral `WorkflowTask` abstraction.
6. Module controllers must be independently deployable and upgradable.

## Current Architecture

```
WorkflowRun (control plane)
  → resolves Workflow/ClusterWorkflow
  → resolves WorkflowPlane/ClusterWorkflowPlane
  → renders runTemplate via CEL pipeline
  → applies rendered Argo Workflow to workflow plane cluster
  → syncs Argo node status back to WorkflowRun.Status.Tasks (Argo-specific)
```

Key existing abstractions that are already engine-agnostic:
- `WorkflowTask` — vendor-neutral task/step representation in `WorkflowRunStatus`
- `ResourceReference` — generic reference (apiVersion/kind/name/namespace) used for `RunReference`
- CEL rendering pipeline — renders any template, not Argo-specific
- `ExternalRef` resolution — engine-agnostic secret/config injection

The **only** Argo-specific code is in `syncWorkflowRunStatus()` in the WorkflowRun controller.

## Proposal

### Option A: Interface-Based Adapter (Recommended for K8s-Native Engines)

Introduce a `WorkflowEngine` interface in the WorkflowRun controller. Each engine implements three methods: apply the rendered resource, sync status back, and clean up on deletion.

```go
// WorkflowEngine abstracts CI engine lifecycle operations.
type WorkflowEngine interface {
    // Apply creates or updates the engine-specific resource on the workflow plane.
    Apply(ctx context.Context, wpClient client.Client, rendered map[string]any, owner *v1alpha1.WorkflowRun) (*v1alpha1.ResourceReference, error)

    // SyncStatus reads the engine-specific resource and returns normalized status.
    SyncStatus(ctx context.Context, wpClient client.Client, ref *v1alpha1.ResourceReference) (*EngineStatus, error)

    // Cleanup deletes the engine-specific resource and any prerequisites.
    Cleanup(ctx context.Context, wpClient client.Client, ref *v1alpha1.ResourceReference) error
}

// EngineStatus is the normalized status contract all engines must produce.
type EngineStatus struct {
    Phase       EnginePhase       // Pending, Running, Succeeded, Failed, Error
    Tasks       []v1alpha1.WorkflowTask
    StartedAt   *metav1.Time
    CompletedAt *metav1.Time
    Message     string            // Human-readable summary (error details, etc.)
}

type EnginePhase string

const (
    EnginePhasePending   EnginePhase = "Pending"
    EnginePhaseRunning   EnginePhase = "Running"
    EnginePhaseSucceeded EnginePhase = "Succeeded"
    EnginePhaseFailed    EnginePhase = "Failed"
    EnginePhaseError     EnginePhase = "Error"
)
```

**Engine selection** — add a field to `WorkflowSpec`:

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: dockerfile-builder
spec:
  engine: argo          # argo (default) | tekton | <future engines>
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
  parameters:
    ...
  runTemplate:
    apiVersion: argoproj.io/v1alpha1
    kind: Workflow
    ...
```

- `spec.engine` defaults to `argo` for backward compatibility — existing ClusterWorkflow definitions work unchanged.
- The WorkflowRun controller looks up the engine adapter from a registry and delegates `Apply`, `SyncStatus`, `Cleanup` calls.
- Each engine adapter is a Go package (e.g., `internal/controller/workflowrun/engine/argo/`, `engine/tekton/`).

**Pros:**
- No new CRDs. No double indirection. Direct status reading.
- Explicit, testable interface. Each engine is unit-testable in isolation.
- The rendering pipeline remains unchanged — engines receive already-rendered resources.
- Simpler failure model — fewer resources to track and reconcile.

**Cons:**
- Adding a new K8s-native engine requires a code change and controller rebuild.
- All engine code runs in the control plane binary.

### Option B: CRD-Based Delegation (Required for API-Based Engines)

For non-K8s CI engines (Azure Pipelines, GitHub Actions, Jenkins), the control plane cannot directly trigger or poll external APIs — it may not have network access or credentials. A separate controller on the workflow plane handles the external integration.

Introduce a new CRD: **`WorkflowExecution`**.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: WorkflowExecution
metadata:
  name: ${metadata.workflowRunName}
  namespace: ${metadata.namespace}
  labels:
    openchoreo.dev/engine: "azure"    # Routing label for module controllers
spec:
  engine: azure                       # Discriminator — which module controller handles this
  timeout: "1h"                       # Max execution duration
  template:                           # Engine-specific payload (opaque to OpenChoreo)
    organization: "my-org"
    project: "my-project"
    pipelineId: 42
    parameters:
      branch: "main"
      imageTag: "v1.2.3"
status:
  phase: ""                           # Pending | Running | Succeeded | Failed | Error
  tasks: []                           # Normalized WorkflowTask list
  startedAt: null
  completedAt: null
  message: ""                         # Human-readable status/error summary
  engineRef:                          # Engine-specific execution reference (for debugging)
    id: ""                            # e.g., Azure run ID, GitHub run URL
    url: ""                           # Link to external CI UI
```

#### Controller Routing

Each module controller watches `WorkflowExecution` CRs filtered by the `spec.engine` field (or the `openchoreo.dev/engine` label for efficient list watches):

```go
// Azure module controller — only reconciles WorkflowExecution with engine=azure
ctrl.NewControllerManagedBy(mgr).
    For(&v1alpha1.WorkflowExecution{}).
    WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
        we := obj.(*v1alpha1.WorkflowExecution)
        return we.Spec.Engine == "azure"
    })).
    Complete(r)
```

- Each module controller only processes CRs matching its engine type.
- If no controller matches (engine module not installed), the CR stays in `Pending` with no status update. The WorkflowRun controller can detect this via a timeout and surface the error.
- The `spec.engine` field is required and validated by a webhook — unknown engines are rejected if no matching module is registered.

#### Status Contract (WorkflowExecution CRD)

Every module controller **must** write status conforming to this contract:

```go
type WorkflowExecutionStatus struct {
    // Phase is the high-level execution state.
    // +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Error
    Phase WorkflowExecutionPhase `json:"phase"`

    // Tasks is the normalized list of steps/stages in the execution.
    // Module controllers must map engine-specific concepts to this:
    //   Argo nodes (type=Pod) → tasks
    //   Tekton TaskRuns → tasks
    //   Azure stages/jobs → tasks
    //   GitHub Actions jobs → tasks
    Tasks []WorkflowTask `json:"tasks,omitempty"`

    // StartedAt is when the execution began.
    StartedAt *metav1.Time `json:"startedAt,omitempty"`

    // CompletedAt is when the execution finished (succeeded or failed).
    CompletedAt *metav1.Time `json:"completedAt,omitempty"`

    // Message is a human-readable summary, typically populated on failure.
    Message string `json:"message,omitempty"`

    // EngineRef contains engine-specific identifiers for debugging.
    EngineRef *EngineReference `json:"engineRef,omitempty"`

    // Conditions for fine-grained status signaling.
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type EngineReference struct {
    // ID is the engine-specific execution identifier (e.g., Azure run ID, GitHub run number).
    ID string `json:"id,omitempty"`
    // URL is a link to the execution in the engine's UI.
    URL string `json:"url,omitempty"`
}
```

**State machine:**

```
                ┌─────────┐
                │ Pending  │  (CR created, no controller has acted yet)
                └────┬─────┘
                     │ controller picks up CR
                     v
                ┌─────────┐
                │ Running  │  (engine execution in progress)
                └────┬─────┘
                     │
            ┌────────┴────────┐
            v                 v
      ┌───────────┐    ┌──────────┐
      │ Succeeded │    │  Failed  │  (execution completed)
      └───────────┘    └──────────┘

      Error — controller hit an unrecoverable error (bad credentials, API unreachable, etc.)
```

#### How WorkflowRun Controller Consumes WorkflowExecution

When `spec.engine` is a CRD-delegated engine:

1. WorkflowRun controller renders the `runTemplate` (produces a `WorkflowExecution` CR).
2. Applies the `WorkflowExecution` to the workflow plane.
3. Stores a `ResourceReference` pointing to it in `WorkflowRun.Status.RunReference`.
4. On each reconcile, reads `WorkflowExecution.Status` and maps it directly to `WorkflowRun.Status`:
   - `Phase` → conditions (`WorkflowRunning`, `WorkflowSucceeded`, `WorkflowFailed`, `WorkflowCompleted`)
   - `Tasks` → `WorkflowRun.Status.Tasks` (already the same type)
   - `StartedAt`, `CompletedAt` → `WorkflowRun.Status.StartedAt`, `CompletedAt`
5. On deletion, deletes the `WorkflowExecution` CR (module controller handles engine-side cleanup via its own finalizer).

### Recommended Hybrid Approach

Use **Option A** (interface adapter) for K8s-native engines and **Option B** (WorkflowExecution CRD) only for API-based engines:

| Engine Type | Mechanism | Where Code Runs | Status Source |
|---|---|---|---|
| Argo Workflows | Interface adapter | Control plane | Direct CR read |
| Tekton Pipelines | Interface adapter | Control plane | Direct CR read |
| Azure Pipelines | WorkflowExecution CRD | Workflow plane module | WorkflowExecution.Status |
| GitHub Actions | WorkflowExecution CRD | Workflow plane module | WorkflowExecution.Status |
| Jenkins | WorkflowExecution CRD | Workflow plane module | WorkflowExecution.Status |

This avoids over-engineering the common case (K8s-native engines don't need a CRD intermediary) while supporting the exotic case (API-based engines that need a separate controller with credentials and network access).

The `spec.engine` field on `WorkflowSpec` determines which path is taken:
- Known K8s-native engines → interface adapter, direct apply + status sync
- Unknown / API-based engines → render as WorkflowExecution, delegate to module controller

## Non-K8s Engine Integration Details

### Credentials

Engine credentials (API tokens, service account keys) are stored as Kubernetes Secrets on the workflow plane, referenced via one of:

- `WorkflowPlane.spec.engineCredentials` — plane-wide default credentials per engine type
- `WorkflowExecution.spec.credentialRef` — per-execution override (e.g., different Azure org)

Module controllers read credentials from the referenced Secret. Credentials never leave the workflow plane — the control plane does not need access to them.

### Polling vs Webhooks

Module controllers use **polling with exponential backoff** as the default strategy:

1. Trigger external execution (API call).
2. Store external execution ID in `status.engineRef.id`.
3. Requeue with backoff: 10s → 20s → 40s → ... → max 5m.
4. On each reconcile, poll external API for status, update `WorkflowExecution.Status`.

Webhook-based notification is an optimization modules can add later — the polling approach is simpler, requires no ingress, and works behind firewalls.

### Idempotency

Module controllers must handle crash recovery:

1. Before triggering an external execution, check if `status.engineRef.id` is already set.
2. If set, skip triggering and go straight to status polling.
3. If not set, trigger the execution, write `status.engineRef.id`, then requeue for polling.

This ensures a controller restart never triggers duplicate executions.

### Timeout Handling

- `WorkflowExecution.spec.timeout` defines the max execution duration (inherited from `Workflow.spec.ttlAfterCompletion` or set explicitly).
- Module controllers check elapsed time since `status.startedAt` on each reconcile.
- If exceeded, the controller cancels the external execution (if the engine API supports it) and sets `status.phase = Failed` with an appropriate message.

## Module Lifecycle

### Installation

A CI engine module is a Helm chart deployed to the workflow plane containing:
1. A controller `Deployment` watching `WorkflowExecution` for its engine type.
2. RBAC (`Role`/`ClusterRole`) for reading `WorkflowExecution` and writing status.
3. Any engine-specific CRDs or dependencies (e.g., Tekton Pipelines operator).

### Discovery

A new optional CRD, `WorkflowEngineModule`, registered by each module on install:

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: WorkflowEngineModule
metadata:
  name: azure-pipelines
spec:
  engine: azure
  version: "1.0.0"
  description: "Azure DevOps Pipelines integration"
  templateSchema:               # JSON Schema for spec.template validation
    type: object
    required: [organization, project, pipelineId]
    properties:
      organization:
        type: string
      project:
        type: string
      pipelineId:
        type: integer
status:
  ready: true
  conditions: [...]
```

- The control plane can list `WorkflowEngineModule` CRs to discover available engines.
- Webhook validation on `WorkflowExecution` can check that a matching module exists.
- `templateSchema` enables validation of the opaque `spec.template` field per engine type.

### Upgrade

Module controllers follow standard Kubernetes deployment rolling update. Since `WorkflowExecution` CRs persist in etcd, in-flight executions survive controller restarts — the new controller picks them up and continues polling/syncing.

### Removal

Uninstalling a module Helm chart removes the controller. Orphaned `WorkflowExecution` CRs in non-terminal states will time out and be marked `Error` by the WorkflowRun controller's timeout detection.

## Backward Compatibility

### Migration Path

1. **Phase 1**: Add `spec.engine` field to `WorkflowSpec` with default `argo`. Refactor `syncWorkflowRunStatus()` into an Argo engine adapter implementing `WorkflowEngine`. No behavioral change for existing definitions.

2. **Phase 2**: Add Tekton engine adapter. Add `WorkflowExecution` CRD. Existing Argo ClusterWorkflows continue working unchanged.

3. **Phase 3**: Build first API-based module (e.g., GitHub Actions) using `WorkflowExecution` delegation. Validate the design end-to-end.

### No Dual Path in Steady State

The `WorkflowEngine` interface is the **single code path** in the WorkflowRun controller. Both direct K8s adapters and CRD-delegated adapters implement the same interface:

```go
// For Argo/Tekton — applies CR directly, reads status directly
type ArgoEngine struct{}
func (e *ArgoEngine) Apply(...)     // creates Argo Workflow on workflow plane
func (e *ArgoEngine) SyncStatus(...) // reads Argo Workflow status
func (e *ArgoEngine) Cleanup(...)   // deletes Argo Workflow

// For Azure/GitHub — applies WorkflowExecution CR, reads its status
type DelegatedEngine struct{}
func (e *DelegatedEngine) Apply(...)     // creates WorkflowExecution on workflow plane
func (e *DelegatedEngine) SyncStatus(...) // reads WorkflowExecution.Status
func (e *DelegatedEngine) Cleanup(...)   // deletes WorkflowExecution
```

The WorkflowRun controller calls `engine.Apply()` / `engine.SyncStatus()` / `engine.Cleanup()` regardless of engine type. No `if argo { ... } else { ... }`.

## Example: ClusterWorkflow for Each Engine Type

### Argo (unchanged)

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: dockerfile-builder
spec:
  engine: argo
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
  parameters:
    ocSchema:
      repository:
        url: "string"
        branch: "string | default=main"
  runTemplate:
    apiVersion: argoproj.io/v1alpha1
    kind: Workflow
    metadata:
      name: ${metadata.workflowRunName}
      namespace: ${metadata.namespace}
    spec:
      entrypoint: build
      templates:
        - name: build
          container:
            image: docker:dind
            command: [docker, build, -t, "${parameters.repository.url}:${parameters.repository.branch}", .]
```

### Tekton

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: tekton-builder
spec:
  engine: tekton
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
  parameters:
    ocSchema:
      repository:
        url: "string"
        branch: "string | default=main"
  runTemplate:
    apiVersion: tekton.dev/v1
    kind: PipelineRun
    metadata:
      name: ${metadata.workflowRunName}
      namespace: ${metadata.namespace}
    spec:
      pipelineRef:
        name: docker-build
      params:
        - name: repo-url
          value: ${parameters.repository.url}
        - name: branch
          value: ${parameters.repository.branch}
```

### Azure Pipelines (via WorkflowExecution)

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: azure-builder
spec:
  engine: azure
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
  parameters:
    ocSchema:
      branch: "string | default=main"
      imageTag: "string"
  runTemplate:
    apiVersion: openchoreo.dev/v1alpha1
    kind: WorkflowExecution
    metadata:
      name: ${metadata.workflowRunName}
      namespace: ${metadata.namespace}
      labels:
        openchoreo.dev/engine: "azure"
    spec:
      engine: azure
      timeout: "1h"
      template:
        organization: "my-org"
        project: "my-project"
        pipelineId: 42
        parameters:
          branch: ${parameters.branch}
          imageTag: ${parameters.imageTag}
```

## Open Decisions

1. Should `WorkflowEngineModule` be a full CRD or a simpler ConfigMap-based registry?
2. Should we support engine-level log streaming (e.g., forwarding Azure pipeline logs to OpenSearch), or is a URL link to the external CI UI sufficient for Phase 1?
3. Should `WorkflowExecution.spec.template` be validated at admission time (requires module's schema to be available to the control plane webhook) or at reconcile time (simpler, but late feedback)?
4. For Tekton, should the engine adapter run in the control plane (like Argo) or be delegated via `WorkflowExecution` to keep Tekton dependencies out of the core binary?
