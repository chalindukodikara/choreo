## PROBLEM

The OpenChoreo workflow system is tightly coupled to Argo Workflows. This coupling exists in three concrete places:

1. **Status sync** (`internal/controller/workflowrun/controller.go:184`) — The WorkflowRun controller hardcodes `argoproj.Workflow{}` to fetch the run resource and extract status.
2. **Task extraction** (`controller.go:310`) — `extractArgoTasksFromWorkflowNodes()` parses Argo-specific node structures to build the vendor-neutral `WorkflowTask` list.
3. **Prerequisites/RBAC** (`run_engine.go:89-95`) — The RBAC Role hardcodes `argoproj.io` API group and `workflowtaskresults` resource.

Additionally, the rendering pipeline has Argo-specific post-processing (`convertParameterValuesToStrings` — Argo requires all parameters as strings).

There is no abstraction layer that would allow plugging in alternative CI/CD engines (Tekton, Azure Pipelines, Jenkins, GitHub Actions, etc.) without modifying the core WorkflowRun controller.

## Requirement

- Different CI/CD engines should be pluggable as OpenChoreo modules.
- The WorkflowRun controller must remain engine-agnostic — it should not contain engine-specific code.
- The CEL-based rendering pipeline and the existing Workflow/ClusterWorkflow CRDs should be preserved.
- Backward compatibility with existing Argo-based Workflow definitions must be maintained.
- Must support both Kubernetes-native engines (Argo, Tekton) and external engines (Azure Pipelines, Jenkins, GitHub Actions).

## Design Analysis: Two Approaches

### Approach A — Intermediate CR (Original Proposal, Modified)

Introduce a new CRD (`WorkflowExecution`) that serves as a **status contract** between the WorkflowRun controller and engine-specific controllers.

**Flow:**
```
WorkflowRun controller
  → Renders runTemplate via CEL pipeline (as today)
  → Creates a WorkflowExecution CR with the rendered resource embedded
  → Watches WorkflowExecution status

Engine controller (per-module)
  → Watches WorkflowExecution CRs (filtered by engine type annotation/label)
  → Extracts embedded template, creates the actual engine resource (Argo Workflow, Tekton PipelineRun, etc.)
  → Manages prerequisites (RBAC, namespaces, etc.) specific to the engine
  → Writes standardized status back to WorkflowExecution
```

**Pros:**
- Clean separation — WorkflowRun controller never touches engine-specific types.
- Engine controllers are fully self-contained modules.
- External engines (Azure, Jenkins) fit naturally — their controller makes API calls and writes status back.
- Adding a new engine = deploying a new controller. No core code changes.

**Cons:**
- Extra CR per workflow run (WorkflowExecution + the actual engine resource).
- Extra reconciliation hop adds latency (~seconds).
- Engine controllers need to be deployed and managed as separate components.

### Approach B — Engine Interface in the WorkflowRun Controller

Define a Go interface that engine-specific adapters implement. The WorkflowRun controller dispatches to the correct adapter based on the rendered resource's GVK or a workflow annotation.

**Flow:**
```
WorkflowRun controller
  → Renders runTemplate via CEL pipeline (as today)
  → Detects engine from rendered resource GVK or workflow annotation
  → Calls engine.EnsurePrerequisites()
  → Calls engine.Apply() to create/update the engine resource
  → Calls engine.SyncStatus() to read status back

Engine adapter (compiled into the controller binary)
  → Implements the WorkflowEngine interface
  → Contains all engine-specific logic (RBAC, status parsing, post-processing)
```

```go
type WorkflowEngine interface {
    // EnsurePrerequisites creates engine-specific RBAC, namespaces, etc.
    EnsurePrerequisites(ctx context.Context, namespace, serviceAccount string, client client.Client) error
    // PostProcess applies engine-specific transformations to rendered output.
    PostProcess(resource map[string]any) (map[string]any, error)
    // Apply creates or updates the engine resource in the workflow plane.
    Apply(ctx context.Context, workflowRun *v1alpha1.WorkflowRun, resource map[string]any, client client.Client) error
    // SyncStatus reads engine resource status and returns standardized status.
    SyncStatus(ctx context.Context, ref *v1alpha1.ResourceReference, client client.Client) (*WorkflowStatus, error)
}

type WorkflowStatus struct {
    Phase      WorkflowPhase
    Tasks      []v1alpha1.WorkflowTask
    StartedAt  *metav1.Time
    FinishedAt *metav1.Time
    Message    string
}
```

**Pros:**
- No extra CR — direct and efficient.
- No extra reconciliation latency.
- Simpler operational model (one controller binary).

**Cons:**
- All engine adapters must be compiled into the controller binary (or loaded as plugins — Go plugins are fragile).
- Cannot support external (non-K8s) engines without the controller making outbound API calls itself.
- Adding a new engine requires recompiling and redeploying the controller.

## Decision: Approach A (Intermediate CR)

**Rationale:**

1. **Extensibility is the primary goal.** The whole point is to let third parties plug in engines without modifying core OpenChoreo code. Approach B requires recompilation for every new engine.
2. **External engine support.** Azure Pipelines, Jenkins, and GitHub Actions are non-Kubernetes systems. An intermediate CR gives their controllers a clean place to write status without the core controller making outbound HTTP calls.
3. **The latency cost is acceptable.** CI/CD runs take minutes to hours. An extra reconciliation hop adding 1-3 seconds is negligible.
4. **Operational complexity is manageable.** Engine controllers are Kubernetes-native and can be deployed via Helm subcharts, matching the existing plane architecture (WorkflowPlane, DataPlane, etc.).

## Detailed Design

### New CRD: `WorkflowExecution`

The name `WorkflowExecution` is chosen over alternatives because:
- ~~`WorkflowAbstraction`~~ — "abstraction" is a design concept, not a resource noun.
- ~~`WorkflowTask`~~ — conflicts with the existing `WorkflowTask` status type.
- ~~`WorkflowJob`~~ — overloaded with Kubernetes Job semantics.
- ~~`PipelineRun`~~ — overloaded with Tekton's PipelineRun.
- `WorkflowExecution` — clearly describes what it is: an execution of a workflow, engine-agnostic.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: WorkflowExecution
metadata:
  name: my-build-run-abc123
  namespace: default
  labels:
    openchoreo.dev/workflowrun: my-build-run
    openchoreo.dev/engine: argo       # Engine selector — engine controllers filter on this
  ownerReferences:
    - apiVersion: openchoreo.dev/v1alpha1
      kind: WorkflowRun
      name: my-build-run
      uid: ...
spec:
  # The fully rendered engine-specific resource (output of CEL pipeline).
  # The engine controller interprets this based on its apiVersion/kind.
  template:
    apiVersion: argoproj.io/v1alpha1
    kind: Workflow
    metadata:
      name: my-build-run-abc123
      namespace: workflows-default
    spec:
      # ... full Argo Workflow spec as rendered by CEL pipeline

  # Target workflow plane for resource creation.
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default

status:
  # Standardized status written by the engine controller
  phase: Running          # Pending | Running | Succeeded | Failed | Error
  message: "Step 2/5: Building container image"
  startedAt: "2026-03-30T10:00:00Z"
  completedAt: null

  # Engine resource reference (set by engine controller after creating the actual resource)
  engineReference:
    apiVersion: argoproj.io/v1alpha1
    kind: Workflow
    name: my-build-run-abc123
    namespace: workflows-default

  # Vendor-neutral task list (same schema as current WorkflowRun.Status.Tasks)
  tasks:
    - name: clone
      phase: Succeeded
      startedAt: "2026-03-30T10:00:00Z"
      completedAt: "2026-03-30T10:00:30Z"
      order: 0
    - name: build
      phase: Running
      startedAt: "2026-03-30T10:00:31Z"
      order: 1

  conditions:
    - type: Ready
      status: "True"
      reason: EngineResourceCreated
      message: "Argo Workflow created successfully"
```

### Go Types

```go
type WorkflowExecutionSpec struct {
    // Template is the fully rendered engine-specific resource.
    // +required
    // +kubebuilder:pruning:PreserveUnknownFields
    // +kubebuilder:validation:Type=object
    Template *runtime.RawExtension `json:"template"`

    // WorkflowPlaneRef references the target workflow plane.
    // +required
    WorkflowPlaneRef *WorkflowPlaneRef `json:"workflowPlaneRef"`
}

type WorkflowExecutionStatus struct {
    // Phase is the high-level execution state.
    // +optional
    // +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Error
    Phase WorkflowExecutionPhase `json:"phase,omitempty"`

    // Message is a human-readable description of the current state.
    // +optional
    Message string `json:"message,omitempty"`

    // StartedAt is when the engine began executing the workflow.
    // +optional
    StartedAt *metav1.Time `json:"startedAt,omitempty"`

    // CompletedAt is when the engine finished executing the workflow.
    // +optional
    CompletedAt *metav1.Time `json:"completedAt,omitempty"`

    // EngineReference points to the actual engine resource created by the engine controller.
    // +optional
    EngineReference *ResourceReference `json:"engineReference,omitempty"`

    // Tasks provides a vendor-neutral view of workflow steps/tasks.
    // +optional
    // +listType=map
    // +listMapKey=name
    Tasks []WorkflowTask `json:"tasks,omitempty"`

    // Conditions represent the latest observations of the execution state.
    // +listType=map
    // +listMapKey=type
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type WorkflowExecutionPhase string

const (
    ExecutionPending   WorkflowExecutionPhase = "Pending"
    ExecutionRunning   WorkflowExecutionPhase = "Running"
    ExecutionSucceeded WorkflowExecutionPhase = "Succeeded"
    ExecutionFailed    WorkflowExecutionPhase = "Failed"
    ExecutionError     WorkflowExecutionPhase = "Error"
)
```

### Modified WorkflowRun Controller Flow

```
Reconcile(WorkflowRun)
  1. Handle deletion (unchanged — finalizer cleans up WorkflowExecution, which cascades)
  2. Ensure finalizer (unchanged)
  3. TTL check (unchanged)
  4. Component validation (unchanged)
  5. Resolve Workflow + WorkflowPlane (unchanged)
  6. Resolve externalRefs (unchanged)
  7. Render via CEL pipeline (unchanged)

  === NEW: replaces direct Argo resource creation ===

  8. Determine engine label from:
     a. Explicit annotation on Workflow CR: `openchoreo.dev/engine: tekton`
     b. GVK detection from rendered resource (argoproj.io/* → argo, tekton.dev/* → tekton)
     c. Default: "argo" (backward compat)

  9. Create/update WorkflowExecution CR with:
     - spec.template = rendered resource
     - spec.workflowPlaneRef = resolved plane ref
     - label: openchoreo.dev/engine = detected engine
     - ownerReference → WorkflowRun

  10. Sync status FROM WorkflowExecution.status → WorkflowRun.status
      - Map phase to conditions (same logic as today, but reading from WorkflowExecution)
      - Copy tasks directly (already in vendor-neutral format)
      - Copy startedAt/completedAt
      - Store engineReference as RunReference
```

### Engine Controller Contract

Each engine controller:
1. **Watches** `WorkflowExecution` CRs filtered by `openchoreo.dev/engine=<engine-name>` label.
2. **Reads** `spec.template` to get the fully rendered engine resource.
3. **Connects** to the workflow plane referenced by `spec.workflowPlaneRef` (using the cluster agent gateway, same as existing pattern).
4. **Creates prerequisites** specific to the engine (namespaces, RBAC, etc.).
5. **Applies** the engine resource to the workflow plane.
6. **Polls/watches** the engine resource status and writes back to `WorkflowExecution.status` using the standardized schema.
7. **Handles cleanup** via finalizer on the `WorkflowExecution` (delete engine resources from workflow plane).

### Argo Engine Controller (Built-in)

Ships with OpenChoreo as the default engine. Extracted from the current WorkflowRun controller:
- `ensurePrerequisites()` → moves here (with Argo-specific RBAC rules)
- `syncWorkflowRunStatus()` → moves here (Argo phase → execution phase mapping)
- `extractArgoTasksFromWorkflowNodes()` → moves here (Argo nodes → WorkflowTask)
- `convertParameterValuesToStrings()` → moves here (Argo-specific post-processing)

### Workflow CR Changes

The `Workflow`/`ClusterWorkflow` CRD gains an optional annotation to declare the engine:

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: dockerfile-builder
  annotations:
    openchoreo.dev/engine: "argo"    # Optional. Auto-detected from runTemplate GVK if omitted.
spec:
  # ... unchanged. runTemplate still contains the engine-specific resource.
```

The `runTemplate` continues to hold the actual engine resource directly (no wrapper). This means:
- Existing Argo Workflow definitions work without modification.
- Tekton users write a `tekton.dev/v1 PipelineRun` in their runTemplate.
- External engine users write a custom resource that their engine controller understands.

### Backward Compatibility Strategy

1. **Phase 1 — Add WorkflowExecution CRD.** WorkflowRun controller creates WorkflowExecution instead of applying directly. Built-in Argo engine controller handles WorkflowExecution CRs with `engine=argo`. Functionally identical to today from the user's perspective.
2. **Phase 2 — Extract Argo code.** Move all Argo-specific code out of the WorkflowRun controller into the Argo engine controller package. WorkflowRun controller becomes fully engine-agnostic.
3. **Phase 3 — Additional engines.** Community/vendors can build engine controllers for Tekton, Azure, etc.

Existing Workflow CRs that don't have the `openchoreo.dev/engine` annotation get `argo` by default (detected from `argoproj.io` GVK in the rendered template). No migration required.

### Example: Tekton Engine

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: tekton-builder
  annotations:
    openchoreo.dev/engine: "tekton"
spec:
  parameters:
    ocSchema:
      image: "string | default=myapp"
  runTemplate:
    apiVersion: tekton.dev/v1
    kind: PipelineRun
    metadata:
      name: ${metadata.workflowRunName}
      namespace: ${metadata.namespace}
    spec:
      pipelineRef:
        name: build-pipeline
      params:
        - name: image
          value: ${parameters.image}
```

The Tekton engine controller would:
1. Watch `WorkflowExecution` with label `openchoreo.dev/engine=tekton`
2. Extract the `PipelineRun` from `spec.template`
3. Ensure Tekton-specific RBAC in the workflow plane
4. Apply the `PipelineRun` to the workflow plane
5. Watch `PipelineRun` status → write to `WorkflowExecution.status`

### Example: Azure Pipelines (External Engine)

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: azure-builder
  annotations:
    openchoreo.dev/engine: "azure-pipelines"
spec:
  parameters:
    ocSchema:
      pipelineId: "integer"
      branch: "string | default=main"
  runTemplate:
    apiVersion: openchoreo.dev/v1alpha1
    kind: AzurePipelineTrigger    # Custom resource understood by the Azure engine controller
    metadata:
      name: ${metadata.workflowRunName}
      namespace: ${metadata.namespace}
    spec:
      organization: myorg
      project: myproject
      pipelineId: ${parameters.pipelineId}
      branch: ${parameters.branch}
      credentials:
        secretRef: azure-devops-pat
```

The Azure engine controller would:
1. Watch `WorkflowExecution` with label `openchoreo.dev/engine=azure-pipelines`
2. Read the `AzurePipelineTrigger` spec from `spec.template`
3. Call Azure DevOps REST API to trigger the pipeline
4. Poll Azure DevOps API for status → write to `WorkflowExecution.status`
5. No Kubernetes resources applied to workflow plane (purely API-driven)

### Log Streaming via Engine Adapter API

The `WorkflowExecution` CR handles status, but build logs are streaming data that don't belong in a CR status field. Each engine stores logs differently:
- **Argo/Tekton**: pod logs in the workflow plane cluster (`kubectl logs`)
- **Azure Pipelines**: Azure DevOps REST API (`GET /_apis/build/builds/{id}/logs`)
- **Jenkins**: Jenkins REST API (`/job/{name}/{build}/consoleText`)
- **GitHub Actions**: GitHub REST API (`GET /repos/{owner}/{repo}/actions/runs/{id}/logs`)

#### Design Pattern: Adapter Pattern

This is the **Adapter pattern** (GoF). The structure is:

- **Target interface** — OpenChoreo's standardized log API that the openchoreo-api server calls.
- **Adaptees** — each engine's native log retrieval mechanism (pod logs, external HTTP APIs).
- **Adapters** — each engine module translates the target interface into the adaptee's interface.

Combined with the engine selection via the `openchoreo.dev/engine` label, this also applies the **Strategy pattern** — the runtime selects which adapter to invoke based on the engine type.

#### Adapter Interface

Each engine controller exposes a log streaming endpoint (HTTP or gRPC) that implements:

```
GET /v1/executions/{name}/logs?task={taskName}&follow={bool}&tailLines={int}
```

Response: `text/event-stream` (SSE) for `follow=true`, `text/plain` for `follow=false`.

```go
// LogAdapter is the interface each engine implements for log retrieval.
type LogAdapter interface {
    // GetLogs returns logs for a specific task within a workflow execution.
    // If follow is true, the returned ReadCloser streams logs until the task completes.
    GetLogs(ctx context.Context, execution *WorkflowExecution, taskName string, opts LogOptions) (io.ReadCloser, error)
}

type LogOptions struct {
    Follow    bool
    TailLines *int64
}
```

Each engine adapter implements this by calling the engine-native log source:

| Engine | LogAdapter Implementation |
|--------|--------------------------|
| Argo | `kubectl logs` on pod nodes in workflow plane cluster |
| Tekton | `kubectl logs` on TaskRun pods in workflow plane cluster |
| Azure Pipelines | `GET /_apis/build/builds/{id}/timeline` + per-step log URLs |
| Jenkins | `GET /job/{name}/{build}/consoleText` (or `/logText/progressiveText` for streaming) |
| GitHub Actions | `GET /repos/{owner}/{repo}/actions/jobs/{id}/logs` |

#### Log Endpoint Discovery

The `WorkflowExecution` status includes a reference to the engine's log endpoint so the openchoreo-api server knows where to route log requests:

```go
type WorkflowExecutionStatus struct {
    // ... existing fields ...

    // LogEndpoint is the URL of the engine controller's log API for this execution.
    // Set by the engine controller when it starts managing the execution.
    // +optional
    LogEndpoint string `json:"logEndpoint,omitempty"`
}
```

Example flow:
```
User → openchoreo-api → reads WorkflowExecution.status.logEndpoint
                       → proxies GET /v1/executions/{name}/logs to engine controller
                       → engine controller calls Argo pod logs / Azure API / etc.
                       → streams response back to user
```

This keeps the openchoreo-api server engine-agnostic — it just reads the endpoint from the CR and proxies.

#### Alternative: Log Reference Instead of API

For simpler deployments, engine controllers could write a `logReference` to the status instead of running an API server:

```yaml
status:
  logReferences:
    - task: clone
      type: pod           # pod | url | s3
      podRef:
        namespace: workflows-default
        name: my-build-abc123-clone-pod
        container: main
    - task: build
      type: url
      url: "https://dev.azure.com/myorg/myproject/_build/results?buildId=123&view=logs"
```

This avoids requiring each engine controller to run an HTTP server, at the cost of the openchoreo-api server needing to understand the different `type` values (`pod` → call k8s API, `url` → redirect user, `s3` → fetch from object store). This trades engine-controller complexity for openchoreo-api complexity — the Adapter API approach is cleaner.

**Recommendation:** Use the Adapter API approach (engine controllers expose a log endpoint). It keeps the openchoreo-api fully engine-agnostic and puts log retrieval logic where it belongs — in the engine module.

## Open Questions

1. **Should WorkflowExecution be namespace-scoped or cluster-scoped?** Recommendation: namespace-scoped (same as WorkflowRun), with ownerReference for cascading delete.
2. **Should the engine label be on the WorkflowExecution only, or also on the Workflow CR?** Recommendation: both — the Workflow CR annotation is the source of truth, the WorkflowExecution label is the operational filter.
3. **Should engine controllers run in-process (same binary) or out-of-process (separate deployment)?** Recommendation: the Argo engine ships in-process as default. Third-party engines are out-of-process. This can be a deployment-time decision — the WorkflowExecution CR is the interface regardless.
4. **Log streaming protocol — HTTP SSE vs gRPC streaming?** Recommendation: HTTP SSE. It's simpler, works through standard ingress/proxies, and the openchoreo-api already serves HTTP. gRPC could be added later if needed for performance.
