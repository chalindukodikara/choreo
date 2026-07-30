# CI Module Adapter — API Contract

This document defines the **standardized adapter API contract** for the pluggable CI
Module abstraction described in [`DISCUSSION.md`](./DISCUSSION.md). It is derived from
an audit of the current CI/CD surface in `internal/openchoreo-api/` and
`internal/controller/workflowrun/`.

The goal is to answer three questions:

1. **What does the OpenChoreo API server do for CI today, and how much of it is
   Argo-specific?**
2. **Which of those responsibilities stay in the core API server, which move into the
   adapter, and which are new APIs needed to support non-Kubernetes-native engines**
   (GitHub Actions, Azure Pipelines, Jenkins) **and other K8s-native engines (Tekton)?**
3. **What is the concrete HTTP contract every adapter must implement?** (Formalized in
   [`openapi-claude.yaml`](./openapi-claude.yaml).)

---

## 1. Current CI surface in `internal/openchoreo-api/`

The CI/CD functionality lives across these services:

| Service package | Responsibility | Engine coupling |
|-----------------|----------------|-----------------|
| `services/workflow` | `Workflow`/`ClusterWorkflow` CR CRUD + schema | **Agnostic** — just CR CRUD, the `runTemplate` is opaque |
| `services/workflowrun` | `WorkflowRun` CR CRUD, **trigger**, **status**, **logs**, **events** | **Mixed** — CRUD is agnostic; logs/events/liveness are **100% Argo** |
| `services/workflowplane` | `WorkflowPlane`/`ClusterWorkflowPlane` CR CRUD + plane K8s client | Agnostic (plane plumbing), but assumes a K8s plane exists |
| `services/git` | Webhook parse/validate for GitHub/GitLab/Bitbucket | SCM-specific, **engine-agnostic** |
| `services/autobuild` | Webhook → `TriggerWorkflow` | Engine-agnostic |
| `services/gitsecret` | Git credential secrets in the `workflows-<ns>` namespace of the workflow plane | Plane/engine-specific |
| `controller/workflowrun` | Renders the Argo template, applies it to the plane, parses Argo node names into `WorkflowTask`, sets `WorkflowRun.Status` | **100% Argo** — this *is* the per-module controller |

### 1.1 What is already engine-agnostic (the runtime/template side)

As `DISCUSSION.md` notes, the **rendering** side is already abstract. The
`Workflow.spec.runTemplate` is an opaque `runtime.RawExtension` rendered by the CEL
engine into a raw resource. The `WorkflowRun` CR and its `WorkflowRunSpec` /
`WorkflowRunStatus` are vendor-neutral:

- `WorkflowRun.Status.Tasks []WorkflowTask` — already documented as
  *"a vendor-neutral abstraction over workflow engine-specific steps (Argo Workflow
  nodes, Tekton TaskRuns)"* (`api/v1alpha1/workflowrun_types.go`).
- `WorkflowRun.Status.RunReference` — an `apiVersion/kind/name/namespace` handle to the
  engine-side object (today an Argo `Workflow`, tomorrow a Tekton `PipelineRun`, or an
  external opaque ID for GitHub Actions).
- `WorkflowRun.Status.Conditions` — `WorkflowRunning` / `WorkflowSucceeded` /
  `WorkflowFailed` are engine-neutral.

So the **CR schema needs no fundamental change** to support other engines. The coupling
lives entirely in (a) the controller that populates the status, and (b) the API methods
that read live operational data.

### 1.2 Where the Argo coupling actually is

**`workflowrun/service.go` — `GetWorkflowRunLogs` / `GetWorkflowRunEvents`:**
These methods are entirely Argo-aware:

- They fetch an `argoproj.Workflow` from the workflow plane by `RunReference`.
- They list pods with the label `workflows.argoproj.io/workflow=<name>`.
- They filter pods by parsing the Argo node-name annotation
  `workflows.argoproj.io/node-name` (`"<workflow>[N].<task>"`) — `matchesTaskName`.
- They strip Argo sidecar containers (`wait`, `init`).
- They fetch pod logs/events via the cluster **gateway client**
  (`GetPodLogsFromPlane` / `GetPodEventsFromPlane`).

**`workflowrun/service.go` — `GetWorkflowRunStatus`:**
Mostly agnostic (it reads `WorkflowRun.Status.Tasks` / `.Conditions`), **except**
`argoWorkflowExists()`, which checks whether the Argo `Workflow` still exists on the
plane to compute `hasLiveObservability`. That liveness probe is Argo-specific.

**`controller/workflowrun/`:** renders the Argo template, ensures plane prerequisites
(namespace, ServiceAccount, Role/RoleBinding for `argoproj.io/workflowtaskresults`),
applies the Argo `Workflow`, watches it, and translates Argo node status into
`WorkflowTask`. This is the reconcile half of the Argo module.

---

## 2. The split: core API ⟷ adapter ⟷ module controller

```
┌──────────────────────────────────────────── Control Plane ───────────────────────────────────────────┐
│                                                                                                        │
│  ┌────────────────────┐        class-based routing          ┌──────────────── CI Module ────────────┐ │
│  │  OpenChoreo API     │  ── resolve adapterServiceName ──►  │  Adapter (standardized HTTP API)       │ │
│  │  (core, agnostic)   │  ◄── logs / events / status ─────   │   • logs, events, live status          │ │
│  │                     │                                     │   • cancel, artifacts, (list)          │ │
│  │  • Workflow CRUD    │                                     │   • SCM helpers (optional)             │ │
│  │  • WorkflowRun CRUD │                                     ├────────────────────────────────────────┤ │
│  │  • TriggerWorkflow  │                                     │  WorkflowRun Controller (class=argo)    │ │
│  │  • webhook→trigger  │  ── creates WorkflowRun CR ──────►  │   • renders template, applies to plane │ │
│  │  • CR-level status  │                                     │   • writes WorkflowRun.Status (tasks)  │ │
│  └────────────────────┘                                     └────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### 2.1 STAYS in the core OpenChoreo API server (engine-agnostic)

These are control-plane Kubernetes CR operations and routing — they must **not** move,
because they are the contract the CLI/UI/MCP already depend on and they are identical
across engines:

| Operation | Path (unchanged) | Why it stays |
|-----------|------------------|--------------|
| List/Get/Create/Update/Delete `Workflow` | `…/workflows[/{name}]` | CR CRUD on control plane |
| Get workflow schema | `…/workflows/{name}/schema` | Reads CR schema section |
| `Workflow`/`ClusterWorkflow` cluster variants | `…/clusterworkflows…` | CR CRUD |
| List/Get/Create/Update/Delete `WorkflowRun` | `…/workflowruns[/{name}]` | CR CRUD on control plane |
| **Trigger** workflow from a component | `…/components/{c}/builds` (TriggerWorkflow) | Creates a `WorkflowRun` CR; engine-agnostic |
| Git webhook → trigger | `/api/v1alpha1/autobuild` | SCM webhook → `TriggerWorkflow`; engine-agnostic |
| `WorkflowPlane`/`ClusterWorkflowPlane` CRUD | `…/workflowplanes…` | CR CRUD; now also carries `adapterServiceName` |
| **CR-level** run status | `…/workflowruns/{name}/status` | Reads `WorkflowRun.Status` from the CR |

The core API server gains **one new responsibility: routing.** For any operational
request (logs/events/live-status/cancel/artifacts) it:

1. Loads the `WorkflowRun` and resolves its `Workflow`/`ClusterWorkflow` to read
   `spec.class`.
2. Resolves the adapter endpoint: `WorkflowPlane.spec.adapterServiceName` (or the
   `Workflow.spec.adapterServiceName` override for engines with no plane).
3. Proxies the request to that adapter using the **adapter contract** below.

### 2.2 MOVES from the core API server into the adapter

These are the Argo-specific reads currently in `workflowrun/service.go`. They become the
adapter's job; the core server only proxies:

| Current method | Becomes adapter endpoint |
|----------------|--------------------------|
| `GetWorkflowRunLogs` (Argo pods + node-name parsing + gateway logs) | `GET …/workflowruns/{run}/logs` |
| `GetWorkflowRunEvents` (Argo pods + gateway events) | `GET …/workflowruns/{run}/events` |
| `argoWorkflowExists` / live status portion of `GetWorkflowRunStatus` | `GET …/workflowruns/{run}/status` (live) |

> The CR-level status (`steps` derived from `WorkflowRun.Status.Tasks`, overall phase
> from conditions) can still be served by the core server straight from the CR. The
> adapter's `/status` is the **authoritative live** view and is what populates
> `hasLiveObservability`. For engines whose runs don't live in K8s (GitHub Actions), the
> adapter's `/status` is the *only* source of step/live status.

### 2.3 NEW adapter APIs needed for non-K8s-native engines

Argo/Tekton keep their run state in the cluster (a CR + pods), so today the core server
can read it directly. GitHub Actions / Azure Pipelines / Jenkins keep run state in an
**external system**, reachable only via that system's REST API with engine-specific
credentials. The adapter is the only component that holds those credentials and knows
those APIs, so it must additionally expose:

| New endpoint | Purpose | Needed by |
|--------------|---------|-----------|
| `GET …/workflowruns/{run}` (engine-side run detail) | Resolve a run's external URL, external ID, conclusion, timing — data not mirrored into the CR | External engines (GHA, Azure, Jenkins) |
| `GET …/workflowruns` (list) | List runs from the external system for a component/project when they are **not** mirrored as CRs | External engines |
| `POST …/workflowruns/{run}/cancel` | Cancel/abort a run in the engine | All engines (Argo `stop`, GHA `cancel`, Jenkins `stop`) |
| `GET …/workflowruns/{run}/artifacts` | List/download build artifacts the engine produced | Optional, all engines |
| `GET /capabilities` | Advertise which of the above the module supports | All — lets the UI/core hide unsupported actions |
| SCM helper group (optional) | `branches` / `commits` / `pull-requests` using the stored Git PAT | All (value-add called out in `DISCUSSION.md` §"Module Configuration") |

### 2.4 What about Git credentials (`gitsecret`)?

`services/gitsecret` writes git-credential secrets into the `workflows-<ns>` namespace of
the **workflow plane** so Argo pods can clone. For K8s-native engines this stays in the
core server (it's plane plumbing). For **external** engines there is no workflow-plane
namespace and the credential is the module's own config/secret (a GitHub App / PAT bundled
with the module's Helm chart, per `DISCUSSION.md`). Recommendation:

- Keep `gitsecret` in the core server **for K8s-native planes** (unchanged).
- For external engines, credentials are owned by the module install (not via this API).
- Optionally expose an adapter `GET /capabilities` flag `requiresGitSecret: false` so the
  UI knows not to prompt for one.

---

## 3. The adapter contract

### 3.1 Design principles

1. **Path-compatible with the core API.** Operational paths mirror the existing core
   paths (`…/namespaces/{namespace}/workflowruns/{runName}/logs|events|status`) so the
   core server can proxy transparently and the response schemas (`WorkflowRunLogEntry`,
   `WorkflowRunEventEntry`, `WorkflowRunStatusResponse`) are **reused unchanged**.
2. **Engine-neutral request/response.** No Argo/Tekton/GHA types cross the boundary.
   Tasks are `WorkflowTask`-shaped; logs/events are the existing flat entry types.
3. **Stateless-friendly.** The core server injects everything the adapter needs to locate
   the engine-side run as headers, so an adapter is *not required* to have control-plane
   RBAC (though a K8s-native adapter MAY read the CR itself):
   - `X-OpenChoreo-Workflow-Class` — e.g. `argo`, `tekton`, `github-actions`.
   - `X-OpenChoreo-Run-Reference` — JSON `{apiVersion,kind,name,namespace}` from
     `WorkflowRun.Status.RunReference`.
   - `X-OpenChoreo-Plane-Type` / `-Plane-Id` / `-Plane-Namespace` / `-Plane-Name` —
     resolved workflow/observability plane coordinates (omitted for external engines).
   - `X-OpenChoreo-Subject` — caller identity, forwarded for the adapter's own auditing.
4. **Capability negotiation.** `GET /capabilities` declares which optional features
   (`cancel`, `artifacts`, `list`, `streamingLogs`, `scm`) the module implements.
5. **Uniform errors.** Same envelope as the core API (`{success,error,code}`), so the
   core server can pass adapter errors through without translation.

### 3.2 Endpoint summary

Base path: `/api/v1` on the adapter `Service` named by `adapterServiceName`.

| Method & path | Maps from / purpose | Required? |
|---------------|---------------------|-----------|
| `GET /healthz`, `GET /readyz` | Liveness/readiness | Required |
| `GET /capabilities` | Feature & metadata advertisement | Required |
| `GET /namespaces/{ns}/workflowruns/{run}/status` | Live status + steps (replaces `argoWorkflowExists` + live portion of `GetWorkflowRunStatus`) | Required |
| `GET /namespaces/{ns}/workflowruns/{run}/logs` | Logs (replaces `GetWorkflowRunLogs`). Query: `task`, `sinceSeconds`, `tailLines`, `follow` | Required |
| `GET /namespaces/{ns}/workflowruns/{run}/events` | Events (replaces `GetWorkflowRunEvents`). Query: `task` | Required |
| `GET /namespaces/{ns}/workflowruns/{run}` | Engine-side run detail (external URL, external id, conclusion) | Required |
| `POST /namespaces/{ns}/workflowruns/{run}/cancel` | Cancel/abort the run | If `capabilities.cancel` |
| `GET /namespaces/{ns}/workflowruns/{run}/artifacts` | List artifacts | If `capabilities.artifacts` |
| `GET /namespaces/{ns}/workflowruns` | List runs from the engine (external engines). Query: `project`, `component`, `limit`, `cursor` | If `capabilities.list` |
| `GET /namespaces/{ns}/scm/branches` etc. | SCM helpers using the module's Git credential | If `capabilities.scm` |

### 3.3 Reused / new response schemas

**Reused as-is from `internal/openchoreo-api/models`:**

- `WorkflowRunStatusResponse { status, steps[], hasLiveObservability }`
- `WorkflowStepStatus { name, phase, startedAt, finishedAt }`
- `WorkflowRunLogEntry { timestamp, log }`
- `WorkflowRunEventEntry { timestamp, type, reason, message }`

**New (adapter-only):**

- `Capabilities { class, displayName, kubernetesNative, features{ logs, events, cancel,
  artifacts, list, streamingLogs, scm }, requiresGitSecret }`
- `RunDetail { name, externalId, externalUrl, status, conclusion, startedAt, finishedAt,
  steps[], runReference }` — the engine-side view, including a deep-link `externalUrl`
  (e.g. the GitHub Actions run page) which today has no home in the CR.
- `RunSummary` (list item) and `RunSummaryList { items[], pagination }`.
- `Artifact { name, sizeBytes, url, contentType }` and `ArtifactList`.

### 3.4 End-to-end example (logs, Argo module)

1. UI calls core: `GET /api/v1/namespaces/acme/workflowruns/build-x/logs?task=build`.
2. Core loads `WorkflowRun build-x`, resolves its `Workflow` → `class=argo`, resolves the
   `WorkflowPlane` → `adapterServiceName=argo-ci-adapter`, reads `Status.RunReference`.
3. Core proxies: `GET http://argo-ci-adapter/api/v1/namespaces/acme/workflowruns/build-x/logs?task=build`
   with headers `X-OpenChoreo-Workflow-Class: argo`,
   `X-OpenChoreo-Run-Reference: {…argo Workflow…}`,
   `X-OpenChoreo-Plane-Type: workflowplane`, `X-OpenChoreo-Plane-Id: production`.
4. Adapter does exactly what `getArgoWorkflowRunLogs` does today (list pods, parse
   node-names, fetch via gateway) and returns `[]WorkflowRunLogEntry`.
5. Core returns the array to the UI unchanged.

For a **GitHub Actions** module the same call routes to `github-actions-ci-adapter`,
which calls the GitHub Actions REST API with its PAT and returns the same
`[]WorkflowRunLogEntry` shape — the UI/core are none the wiser.

---

## 4. Open questions (carried from `DISCUSSION.md`)

1. **WorkflowPlane for external engines?** The contract supports both: a `WorkflowPlane`
   carrying `adapterServiceName`, or the `Workflow`-level override. Recommendation: allow
   the `Workflow`-level override and do **not** force a synthetic `WorkflowPlane`.
2. **CR-level vs adapter `/status`.** Two sources of step status exist (the CR, written by
   the module controller, and the adapter live view). Recommendation: the core server
   prefers the adapter `/status` when `hasLiveObservability` is true, else falls back to
   `WorkflowRun.Status` from the CR (so completed/GC'd runs still show their last state).
3. **Streaming logs.** `follow=true` is declared as an optional capability
   (`streamingLogs`) returning `text/event-stream`. Non-streaming `application/json`
   array is the required baseline (matches today's behavior).
4. **Auth between core and adapter.** In-cluster service-to-service. The contract assumes
   the core server is the only caller and forwards `X-OpenChoreo-Subject` for audit;
   network policy restricts the adapter to the core server. (Out of scope here; flagged.)

---

## 5. Summary table — disposition of every current CI API

| Current behavior | Disposition | New home |
|------------------|-------------|----------|
| `Workflow`/`ClusterWorkflow` CRUD + schema | **Stay** | Core API |
| `WorkflowRun` CRUD | **Stay** | Core API |
| `TriggerWorkflow` (component build) | **Stay** | Core API |
| Autobuild webhook → trigger | **Stay** | Core API |
| `WorkflowPlane` CRUD (+ new `adapterServiceName`) | **Stay** | Core API |
| `GetWorkflowRunStatus` (CR-level steps/phase) | **Stay** | Core API (from CR) |
| `GetWorkflowRunStatus` liveness (`argoWorkflowExists`) | **Move** | Adapter `/status` |
| `GetWorkflowRunLogs` | **Move** | Adapter `/logs` |
| `GetWorkflowRunEvents` | **Move** | Adapter `/events` |
| `controller/workflowrun` reconcile | **Move** | Module WorkflowRun Controller |
| `gitsecret` (K8s-native planes) | **Stay** | Core API |
| Engine-side run detail / external URL | **New** | Adapter `/{run}` |
| List runs from external system | **New** | Adapter `/workflowruns` |
| Cancel run | **New** | Adapter `/cancel` |
| Artifacts | **New** | Adapter `/artifacts` |
| Capability advertisement | **New** | Adapter `/capabilities` |
| SCM helpers (branches/commits/PRs) | **New (optional)** | Adapter `/scm/*` |
