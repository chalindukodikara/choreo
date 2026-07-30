# OpenChoreo CI Adapter API Contract

## Scope

This document defines the first-cut standard API contract between the OpenChoreo API server and a CI module adapter. It is based on the current `internal/openchoreo-api` workflow-run code and extends it for non-Kubernetes CI systems such as GitHub Actions, Jenkins, Azure Pipelines, and Kubernetes-native engines such as Argo and Tekton.

The OpenChoreo API server remains the owner of platform-facing concerns:

- Workflow, ClusterWorkflow, WorkflowRun, WorkflowPlane, and ClusterWorkflowPlane CRUD.
- Namespace, project, component, and authorization checks.
- WorkflowRun creation from components and user requests.
- Adapter resolution from `Workflow.spec.class`, `WorkflowPlane.spec.adapterServiceName`, or a workflow-level override for engines without a workflow plane.
- Listing WorkflowRuns from OpenChoreo CRs, including label, project, component, and workflow filtering.

The adapter owns CI-runtime concerns:

- Normalizing engine-specific run state into OpenChoreo status, task, log, event, artifact, annotation, and external-link models.
- Fetching live or archived logs from the correct backend.
- Translating events from Kubernetes Events, Argo/Tekton status, GitHub check annotations, Jenkins console metadata, Azure timeline records, etc.
- Exposing optional operational actions such as cancel and rerun.
- Exposing optional source-control helper APIs when the module has provider credentials.
- Receiving and normalizing CI-engine webhooks when the engine needs callbacks for state sync.

## Current Code Findings

The current API has these WorkflowRun runtime endpoints in `openapi/openchoreo-api.yaml` and `internal/openchoreo-api/api/handlers/workflows.go`:

- `GET /api/v1/namespaces/{namespaceName}/workflowruns/{runName}/status`
- `GET /api/v1/namespaces/{namespaceName}/workflowruns/{runName}/logs`
- `GET /api/v1/namespaces/{namespaceName}/workflowruns/{runName}/events`

The current service implementation in `internal/openchoreo-api/services/workflowrun/service.go` is Argo-specific in these places:

- Resolves a WorkflowPlane from the referenced Workflow.
- Reads an Argo Workflow from the workflow plane.
- Lists pods using Argo labels.
- Parses Argo node names and pod annotations to match tasks.
- Calls the cluster gateway for pod logs and Kubernetes events.
- Computes `hasLiveObservability` by checking whether the Argo Workflow still exists.

The current controller in `internal/controller/workflowrun/controller.go` is also Argo-specific:

- Applies the rendered run resource to the workflow plane.
- Reads Argo Workflow phase and Argo node status.
- Converts Argo pod nodes to `WorkflowRun.status.tasks`.
- Cleans up Kubernetes resources through a workflow-plane client.

These behaviors should move into the Argo CI module. OpenChoreo core should call the adapter for read-side runtime data, while each module's controller reconciles only WorkflowRuns for its `spec.class`.

## Proposed Routing Fields

These fields are required by the module design, but they do not exist in the current CRD types yet.

| Field | Resource | Purpose |
| --- | --- | --- |
| `spec.class` | `Workflow`, `ClusterWorkflow` | CI module key, for example `argo`, `tekton`, `github-actions`, `jenkins`, `azure-pipelines`. Module controllers watch WorkflowRuns whose resolved workflow has this class. |
| `spec.adapterServiceName` | `WorkflowPlane`, `ClusterWorkflowPlane` | Service name of the adapter for workflows using that plane. |
| `spec.adapterServiceName` | `Workflow`, `ClusterWorkflow` | Override for external engines that do not use a WorkflowPlane. |

For non-Kubernetes engines, the current `WorkflowRun.status.runReference` type is too Kubernetes-shaped. Add a generic runtime reference either by extending it or adding a sibling field:

```yaml
status:
  runtimeRef:
    class: github-actions
    provider: github
    id: "12345678901"
    attempt: 1
    url: "https://github.com/org/repo/actions/runs/12345678901"
    repositoryUrl: "https://github.com/org/repo"
    branch: main
    commit: "abc1234..."
```

For Kubernetes-native engines, the same structure can include a Kubernetes resource reference:

```yaml
status:
  runtimeRef:
    class: argo
    provider: kubernetes
    id: "workflows/build-abc123"
    kubernetesRef:
      apiVersion: argoproj.io/v1alpha1
      kind: Workflow
      namespace: workflows-default
      name: build-abc123
```

## Adapter Invocation Model

The OpenChoreo API server calls the adapter only after it has authenticated and authorized the user. The adapter should still perform basic tenant checks and must not return data for a different namespace or WorkflowRun than requested.

Recommended assumptions:

- The adapter can read WorkflowRun CRs in the control plane, or it keeps a module-local index keyed by `{namespaceName, runName}`.
- The module controller writes enough runtime identity into WorkflowRun status for the adapter to find the upstream run.
- The adapter service is internal to the control plane and is protected with Kubernetes network policy and either mTLS, service-account token authentication, or another internal auth mechanism.
- The adapter returns engine-neutral shapes. Provider-specific fields go under `metadata`.

Common forwarded headers:

- `X-OpenChoreo-Request-Id`: Correlates API server and adapter logs.
- `X-OpenChoreo-User`: User or service account identity for audit only.
- `X-OpenChoreo-Namespace`: Authorized OpenChoreo namespace.

## Required Adapter APIs

These are the minimum APIs the OpenChoreo API server needs in order to remove runtime-specific logic from `internal/openchoreo-api`.

| API | Required | Purpose |
| --- | --- | --- |
| `GET /api/v1/healthz` | Yes | Liveness/readiness for routing and diagnostics. |
| `GET /api/v1/capabilities` | Yes | Declares supported features, log modes, actions, and source/webhook support. |
| `GET /api/v1/workflowruns/{namespaceName}/{runName}/status` | Yes | Overall status, task/stage/job status, external run reference, live-observability availability. Replaces `GetWorkflowRunStatus` runtime logic. |
| `GET /api/v1/workflowruns/{namespaceName}/{runName}/tasks` | Yes | Task/job/stage list when callers need it independently from status. |
| `GET /api/v1/workflowruns/{namespaceName}/{runName}/logs/streams` | Yes | Discovers available log streams before reading logs. Needed for engines with jobs, matrix builds, stages, attempts, or multiple containers. |
| `GET /api/v1/workflowruns/{namespaceName}/{runName}/logs` | Yes | Returns normalized log entries. Replaces Argo pod lookup, container filtering, and timestamp parsing in OpenChoreo API. |
| `GET /api/v1/workflowruns/{namespaceName}/{runName}/events` | Yes | Returns normalized timeline events. Replaces Kubernetes-event-only behavior. |
| `GET /api/v1/workflowruns/{namespaceName}/{runName}/external-link` | Yes | Returns canonical upstream URL and display identity. Required for GitHub Actions, Jenkins, Azure Pipelines, and useful for Argo/Tekton dashboards. |

## Optional Adapter APIs

These are needed for a complete CI experience across external systems, but adapters can advertise support through `/capabilities`.

| API | Purpose |
| --- | --- |
| `GET /api/v1/workflowruns/{namespaceName}/{runName}/annotations` | Build warnings/errors such as GitHub annotations, Azure issues, Jenkins warnings, Tekton task messages. |
| `GET /api/v1/workflowruns/{namespaceName}/{runName}/artifacts` | Lists build artifacts. |
| `GET /api/v1/workflowruns/{namespaceName}/{runName}/artifacts/{artifactId}` | Returns artifact metadata and a signed/proxied download URL. |
| `POST /api/v1/workflowruns/{namespaceName}/{runName}:cancel` | Cancels an active run if supported by the engine. |
| `POST /api/v1/workflowruns/{namespaceName}/{runName}:rerun` | Reruns a whole run or failed tasks if supported. |
| `POST /api/v1/webhooks/{webhookType}` | Module-owned CI callbacks, for example GitHub Actions `workflow_run`, Jenkins build notifications, Azure service hooks. |
| `GET /api/v1/source/branches` | Lists repository branches using module credentials. |
| `GET /api/v1/source/commits` | Lists commits for a repository/branch. |
| `GET /api/v1/source/pullrequests` | Lists pull requests for repository integration UX. |

## Status Contract

Canonical run statuses:

- `Pending`: accepted by OpenChoreo or upstream CI but not running.
- `Running`: actively executing.
- `Succeeded`: completed successfully.
- `Failed`: completed unsuccessfully.
- `Cancelled`: stopped by user/system cancellation.
- `Skipped`: intentionally not executed.
- `Error`: adapter or CI engine could not evaluate the run normally.
- `Unknown`: adapter cannot determine state yet.

Canonical task statuses use the same enum. Adapters should map engine terms as follows:

| Engine | Native status examples | Canonical status |
| --- | --- | --- |
| Argo | `Pending`, `Running`, `Succeeded`, `Failed`, `Error`, `Omitted` | Same, with `Omitted` -> `Skipped` |
| Tekton | `Unknown`, `True`, `False`; TaskRun conditions | `Running`, `Succeeded`, `Failed`, `Error` based on reason |
| GitHub Actions | `queued`, `in_progress`, `completed` + conclusion | `Pending`, `Running`, conclusion mapped to `Succeeded`/`Failed`/`Cancelled`/`Skipped` |
| Jenkins | `building`, `result` | `Running`, result mapped to `Succeeded`/`Failed`/`Cancelled` |
| Azure Pipelines | `notStarted`, `inProgress`, `completed` + result | `Pending`, `Running`, result mapped to canonical status |

The adapter response should include both canonical fields and provider-native fields:

- `status`: canonical status.
- `nativeStatus`: provider status.
- `nativeConclusion`: provider conclusion/result if separate from status.
- `tasks[].status`: canonical task status.
- `tasks[].nativeStatus`: provider task status.
- `metadata`: provider-specific escape hatch.

## Logs Contract

The current OpenChoreo API returns a flat array of `{timestamp, log}`. That is insufficient for GitHub Actions jobs, Jenkins stages, Azure timeline records, Tekton pods, retries, and matrix builds.

The adapter should return:

- `entries[]`: normalized log entries.
- `pagination.nextCursor`: for large logs and remote CI APIs.
- `complete`: whether all matching logs were returned.
- `stream`: stream metadata when a single stream is requested.

Query parameters:

- `taskName`: logical task/job/stage name.
- `streamId`: adapter-provided stream ID from `/logs/streams`.
- `containerName`: useful for Kubernetes-native engines.
- `sinceSeconds` or `sinceTime`: incremental polling.
- `tailLines`: tail mode.
- `cursor` and `limit`: pagination.
- `follow`: request live mode. If unsupported, the adapter returns normal paginated logs and advertises no `sse` or `websocket` log mode.

Adapters should preserve log order and return RFC3339 timestamps when the source provides them. If the source has no timestamp, omit it rather than fabricating one.

## Events Contract

Events are engine-neutral timeline records, not only Kubernetes Events.

Examples:

- Argo/Tekton pod scheduling events.
- GitHub job queued/started/completed events.
- Jenkins stage start/end events.
- Azure timeline records and task issues.

Each event has:

- `timestamp`
- `type`
- `severity`
- `reason`
- `message`
- optional `taskName`, `source`, `count`, `externalUrl`, and `metadata`

## Artifacts And Annotations

External CI systems commonly expose artifacts and code/build annotations. Kubernetes-native engines may not support them directly, but the API should be part of the standard contract because frontends and CLI clients will otherwise need engine-specific branches.

Artifacts should be returned as metadata plus a time-limited `downloadUrl` or an adapter-proxied URL. The first version does not need binary streaming in the OpenAPI contract.

Annotations should normalize warnings/errors from GitHub check annotations, Azure task issues, Jenkins warnings plugins, or Tekton/Argo task messages.

## Actions

Cancel and rerun are optional because not all engines support them equally.

The adapter must return `501 Not Implemented` when a feature is not implemented and should also advertise that through `/capabilities`.

Rerun modes:

- `all`: rerun the whole workflow.
- `failed`: rerun failed jobs/tasks only, when supported.
- `task`: rerun a named task/job, when supported.

The rerun response may reference a newly created OpenChoreo WorkflowRun or only an upstream run. If a new OpenChoreo WorkflowRun is needed, OpenChoreo core should own creating that CR and then call the adapter/controller path.

## Webhooks

There are two different webhook categories:

1. Source repository push webhooks that trigger OpenChoreo component builds.
2. CI engine callbacks that report upstream run state changes.

The current `/api/v1alpha1/autobuild` endpoint handles source webhooks for GitHub, GitLab, and Bitbucket and then creates WorkflowRuns. That can remain in OpenChoreo core if it is treated as source-trigger logic. Provider-specific parsing and repository API calls can move to a source adapter later.

CI engine callbacks should be owned by the CI module. Examples:

- GitHub Actions `workflow_run` and `check_run` callbacks.
- Jenkins generic webhook or notification plugin callbacks.
- Azure DevOps service hooks.

The standard adapter webhook endpoint accepts raw headers and payload and returns a normalized `WebhookIngestResponse`. Adapters can also expose engine-specific webhook paths outside this contract if their provider requires exact URL paths.

## Source APIs

The discussion noted that module config may include Git provider credentials even for Kubernetes-native engines. These APIs are optional but useful for UI/CLI workflows:

- List branches.
- List commits.
- List pull requests.

These belong in the adapter only when the CI module owns the credentials needed to call the provider. Otherwise OpenChoreo should keep source APIs separate from CI runtime APIs.

## OpenChoreo API Changes

The public OpenChoreo API can keep its existing paths. Internally the handlers should proxy or translate to the adapter:

| Public OpenChoreo API | Internal adapter call |
| --- | --- |
| `GET /api/v1/namespaces/{namespaceName}/workflowruns/{runName}/status` | `GET /adapter/v1/workflowruns/{namespaceName}/{runName}/status` |
| `GET /api/v1/namespaces/{namespaceName}/workflowruns/{runName}/logs` | `GET /adapter/v1/workflowruns/{namespaceName}/{runName}/logs` |
| `GET /api/v1/namespaces/{namespaceName}/workflowruns/{runName}/events` | `GET /adapter/v1/workflowruns/{namespaceName}/{runName}/events` |

Additional public OpenChoreo APIs can be added later for streams, artifacts, annotations, external links, cancel, and rerun. The adapter contract already defines them so public API additions do not require engine-specific OpenChoreo core logic.

## Implementation Notes By Engine

Argo adapter:

- Reads Argo Workflow and pods from the workflow plane.
- Moves current pod lookup, Argo node parsing, log parsing, Kubernetes event fetching, and `hasLiveObservability` logic out of OpenChoreo API.
- Can expose Argo UI link if configured.

Tekton adapter:

- Reads PipelineRun, TaskRun, and pod logs.
- Maps Tekton condition status/reason to canonical run/task status.
- Uses TaskRun pod/container references for log streams.

GitHub Actions adapter:

- Stores GitHub run ID, attempt, repository, branch, and commit in WorkflowRun status.
- Calls GitHub API for run, jobs, logs, annotations, artifacts, cancel, rerun.
- Handles GitHub Enterprise API base URL through module config.

Jenkins adapter:

- Stores job full name, build number, queue item, and Jenkins base URL.
- Calls Jenkins API for build result, stages if available, progressive console logs, artifacts, stop/rebuild.
- Handles crumb issuer/auth in module config.

Azure Pipelines adapter:

- Stores organization, project, pipeline definition/build ID, run ID, and attempt.
- Calls Azure DevOps APIs for run status, timeline, logs, artifacts, cancel/rerun.
- Maps timeline records to tasks/events/annotations.

## OpenAPI

The proposed machine-readable adapter contract is in `openapi-codex.yaml`.
