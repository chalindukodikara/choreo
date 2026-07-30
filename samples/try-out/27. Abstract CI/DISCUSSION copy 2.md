# Abstract OpenChoreo CI Module

## Problem

Currently, OpenChoreo's CI system is tightly coupled to Argo Workflows with no mechanism for extension:

- The **WorkflowRun controller** directly imports Argo types, parses Argo-specific node structures, and polls the Argo Workflow object for status.
- **Log and event retrieval** assumes pod-based execution with Argo-specific pod selectors.
- **Task extraction** is hardcoded to parse Argo node name patterns (e.g., `workflow-name[N].step-name`).
- There is no abstraction layer, adapter interface, or `class` field that would allow plugging in an alternative CI engine.

This means supporting a new engine (GitHub Actions, Jenkins, Tekton, Azure Pipelines) would require forking the existing controller logic rather than extending it.

## Existing Foundation

The `Workflow`/`ClusterWorkflow` CR's `runTemplate` field already accepts any arbitrary Kubernetes resource definition rendered via CEL templates. The template engine itself is CI-agnostic -- it evaluates expressions and produces a raw resource. This is a key building block: the rendering pipeline does not need to change.

What is missing is the **runtime layer** -- how the system reconciles, monitors, and retrieves information from the rendered resource once it is applied.

## Proposal

Introduce a **CI Module** abstraction that decouples the control plane from any specific CI engine.

### Architecture

Each CI module consists of two components:

![CI Module Architecture](1.png)

1. **WorkflowRun Controller** -- Knows how to reconcile workflow runs for a specific CI engine (e.g., create an Argo Workflow, trigger a GitHub Actions run, submit a Jenkins job). Each module ships its own controller that watches only the WorkflowRuns relevant to it.

2. **Adapter (Service)** -- Exposes a standardized API for retrieving logs, run status, events, and other operational data. The OpenChoreo API calls the adapter instead of directly querying engine-specific resources.

Multiple CI modules can be installed in the same control plane simultaneously.

### Routing: How Workflows Find Their Module

![Multi-module Routing](2.png)

Two new fields enable routing:

| Field | CR | Purpose |
|-------|-----|---------|
| `spec.class` | `Workflow` / `ClusterWorkflow` | Identifies which CI engine this workflow targets (e.g., `argo`, `tekton`, `github-actions`). Defined by the module author. Each WorkflowRun controller watches only runs whose resolved workflow matches its class. |
| `spec.adapterServiceName` | `WorkflowPlane` / `ClusterWorkflowPlane` | The Kubernetes service name of the adapter API for this plane. The OpenChoreo API uses this to route log/status/event requests to the correct adapter. |

**For non-Kubernetes-native engines** (e.g., GitHub Actions, Jenkins) that have no WorkflowPlane, the `adapterServiceName` field is also available on the `Workflow`/`ClusterWorkflow` CR as an override.

#### Example CRs

```yaml
# Argo-based workflow -- class tells the Argo module's controller to pick this up
kind: Workflow
metadata:
  name: build-and-push
spec:
  class: argo
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
  parameters:
    ocSchema:
      image: "string"
  runTemplate: ...  # Argo Workflow template
```

```yaml
# WorkflowPlane points to the Argo adapter service
kind: ClusterWorkflowPlane
metadata:
  name: default
spec:
  adapterServiceName: argo-ci-adapter
  planeID: production
  clusterAgent: ...
```

```yaml
# GitHub Actions workflow -- no WorkflowPlane, adapter on the Workflow itself
kind: ClusterWorkflow
metadata:
  name: gh-actions-ci
spec:
  class: github-actions
  adapterServiceName: github-actions-ci-adapter
  parameters:
    ocSchema:
      repository: "string"
      workflow_id: "string"
  runTemplate: ...  # Could be a lightweight CR or config for the GHA controller
```

### How It Works End-to-End

1. A `WorkflowRun` is created referencing a `Workflow` with `class: argo`.
2. The **Argo CI module's controller** (watching for `class: argo`) picks it up, renders the template, and applies the Argo Workflow to the workflow plane cluster.
3. The controller reconciles status, extracts tasks, and updates the WorkflowRun CR -- all using Argo-specific logic encapsulated within the module.
4. When the OpenChoreo API needs logs or status, it resolves the `adapterServiceName` from the WorkflowPlane (or Workflow) and calls the adapter's standardized API.
5. The adapter translates the request into engine-specific calls (e.g., fetch pod logs for WorkflowPlane/ObservabilityPlane, call GitHub API for GHA) and returns a unified response.

## Options

### Option 1: Pluggable CI Modules (proposed above)

**Pros:**
- Supports any CI engine -- Kubernetes-native (Argo, Tekton) and external (GitHub Actions, Jenkins, Azure Pipelines)
- Standardized adapter API enables consistent experience across CLI, MCP, and the OpenChoreo API for logs, status, and events
- Each module is independently deployable and maintainable
- If moving away from Backstage, current Jenkins/GitHub Actions support can migrate to this model
- Third-party module authors can extend the platform without modifying core code

**Cons:**
- More complex initial implementation -- requires defining the adapter API contract and refactoring the existing Argo logic into a module
- Operators need to install and manage per-engine modules

### Option 2: Support only Kubernetes-native engines in-tree

Support k8s native tools (Argo and Tekton) in the workflow run controller. Other engines remain supported through Backstage (or similar).

**Pros:**
- Simpler architecture -- fewer moving parts, no adapter API to define
- Less operational overhead for users who only need Argo/Tekton

**Cons:**
- Core controller must understand every supported engine -- grows in complexity over time
- Not pluggable -- adding a new engine requires changes to the core codebase
- External engines (GitHub Actions, Jenkins) remain second-class citizens with a different integration path

**Recommendation:** Option 1. The upfront complexity is manageable and pays off as the ecosystem grows.

## Open Questions

1. **Option 1 or 2?** I prefer Option 1 for the extensibility it provides.
2. **WorkflowPlane for non-plane engines?** Currently WorkflowPlane is only created when there is a physical plane (a cluster running the engine). Should we still create a WorkflowPlane CR for engines like GitHub Actions that don't have one, purely as a configuration anchor? Or is the Workflow-level `adapterServiceName` override sufficient?
3. **Adapter API contract** -- What should the standardized adapter API look like? At minimum: `GetLogs`, `GetStatus`, `GetEvents`, `ListRuns`. Should it also support `TriggerRun` / `CancelRun`, or do those remain controller-only operations?
4. **Module discovery** -- Should CI modules self-register (e.g., via a CRD or label convention), or is manual configuration via `adapterServiceName` sufficient?
