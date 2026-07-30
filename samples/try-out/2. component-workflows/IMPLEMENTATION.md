# Implementation Plan: Merge ComponentWorkflows into Workflows

## Context

We are merging the `ComponentWorkflow`/`ComponentWorkflowRun` CRD pair into the existing
`Workflow`/`WorkflowRun` CRD pair. Instead of structured system parameters (repository.url,
revision.branch, etc.), a `Workflow` CR uses an annotation
(`openchoreo.dev/component-workflow-parameters`) to declare which parameter paths hold
component-build-specific values (repoUrl, branch, appPath, secretRef, commit).

### Key Design: Annotation-Based Mapping

A `Workflow` CR that powers component builds carries an annotation like:

```
openchoreo.dev/component-workflow-parameters: "repoUrl: parameters.repository.url, branch: parameters.repository.revision.branch, appPath: parameters.repository.appPath, secretRef: parameters.repository.secretRef, commit: parameters.repository.revision.commit"
```

This comma-separated `key: dotted.path` format tells the platform which parameter path to
read/write for each logical concept. The actual values come from
`Component.Spec.Workflow.Parameters` (a free-form `runtime.RawExtension`).

---

## Progress

### ✅ Task 1 — Delete ComponentWorkflow and ComponentWorkflowRun API types

Both files were **deleted entirely** (not just removing SchemeBuilder.Register):
- ~~`api/v1alpha1/componentworkflow_types.go`~~ — deleted
- ~~`api/v1alpha1/componentworkflowrun_types.go`~~ — deleted

**Side effects resolved:**
- `ResourceReference` and `WorkflowTask` types (previously in `componentworkflowrun_types.go`)
  moved to `api/v1alpha1/workflowrun_types.go` (they are used by `WorkflowRunStatus`).
- `api/v1alpha1/component_types.go`: `Workflow *ComponentWorkflowRunConfig` changed to
  `Workflow *WorkflowRunConfig` (same shape without SystemParameters; comment updated).
- `api/v1alpha1/zz_generated.deepcopy.go` regenerated via `make generate`.

**Follow-up status:** resolved in Task 11. Deleted controller/pipeline packages and migrated
component controller + scaffold generator to `Workflow`/`WorkflowRun`.

---

### ✅ Task 2 — Remove SystemParameters from the Component API

Handled as part of Task 1: `component_types.go` now references `WorkflowRunConfig` (which
has only `Name` and `Parameters`) instead of the deleted `ComponentWorkflowRunConfig`.

---

### ✅ Task 3.1 — Remove component-workflow endpoints from handlers.go

**File:** `internal/openchoreo-api/handlers/handlers.go`

Removed all 12 ComponentWorkflow route registrations. Kept the
`PATCH .../workflow-parameters` endpoint (updated in Task 3.3).

---

### ✅ Task 3.2 — Add new Workflow endpoints

**File:** `internal/openchoreo-api/handlers/handlers.go`

Added:
```go
api.HandleFunc("POST "+v1+"/namespaces/{namespaceName}/workflows/definition", h.CreateWorkflowDefinition)
api.HandleFunc("GET "+v1+"/namespaces/{namespaceName}/workflow-runs/{runName}/logs", h.GetWorkflowRunLogs)
```

**New code added:**

| File | What was added |
|------|---------------|
| `internal/openchoreo-api/handlers/resource_crud.go` | `CreateWorkflowDefinition` handler — mirrors `CreateComponentWorkflowDefinition`, validates `kind == "Workflow"`, uses `WorkflowService.AuthorizeCreate` |
| `internal/openchoreo-api/handlers/workflowruns.go` | `GetWorkflowRunLogs` handler — `step` and `sinceSeconds` query params, calls `WorkflowRunService.GetWorkflowRunLogs` |
| `internal/openchoreo-api/legacyservices/workflow_service.go` | `AuthorizeCreate` method |
| `internal/openchoreo-api/legacyservices/workflowrun_service.go` | `GetWorkflowRunLogs`, `getArgoWorkflowRunLogs`, `getArgoWorkflowPodLogs` methods |
| `internal/openchoreo-api/legacyservices/constants.go` | `SystemActionCreateWorkflow = "workflow:create"` constant |

---

### ✅ Task 3.3 — Remove systemParameters from workflow-parameters PATCH

**File:** `internal/openchoreo-api/models/request.go`

- Removed `SystemParameters *ComponentWorkflowSystemParams` from `UpdateComponentWorkflowRequest`.
- Removed `ComponentWorkflowSystemParams`, `ComponentWorkflowRepository`,
  `ComponentWorkflowRepositoryRevision` types.
- Removed `SystemParameters` field from `ComponentWorkflow` model type.

**File:** `internal/openchoreo-api/models/response.go`

- Removed `SystemParametersResponse`, `RepositoryResponse`, `RepositoryRevisionResponse` types.
- Simplified `ComponentWorkflowConfigResponse` to just `Name` + `Parameters`.

**File:** `internal/openchoreo-api/legacyservices/component_service.go`

- `createComponentResources`: replaced `ComponentWorkflowRunConfig` with `WorkflowRunConfig`,
  removed `SystemParameters` assignment block (only sets `Name` and `Parameters` now).
- `toComponentResponse`: simplified to just `Name` + `Parameters` (no `SystemParameters`).
- `UpdateComponentWorkflowParameters`: removed the entire `req.SystemParameters` block.
- `UpdateComponentWorkflowSchema`: replaced `ComponentWorkflowRunConfig` with `WorkflowRunConfig`,
  removed the `req.SystemParameters` block.
- `validateComponentWorkflowParameters` → renamed to `validateWorkflowParameters`: now fetches
  `v1alpha1.Workflow{}` instead of `v1alpha1.ComponentWorkflow{}` and checks
  `workflow.Spec.Schema.Parameters`.

**File:** `internal/openchoreo-api/api/handlers/components.go`

- `toGenComponentWorkflowConfig`: removed entire `SystemParameters` conversion block.
- `toModelComponentWorkflow`: removed `SystemParameters` mapping, now only sets `Name` + `Parameters`.

---

### ✅ Task 4.1 — Fix findAffectedComponents (webhook auto-build)

**File:** `internal/openchoreo-api/legacyservices/webhook_service.go`

Method: `extractRepoInfoFromComponent` — **rewritten**

Changes:
- Added `ctx context.Context` parameter (caller `findAffectedComponents` updated accordingly)
- Fetches the `Workflow` CR using `comp.Spec.Workflow.Name`
- Reads `AnnotationKeyComponentWorkflowParameters` annotation from the Workflow CR
- Parses annotation into `map[string]string` (key → dotted parameter path)
- Extracts `repoUrl` and `appPath` values from `comp.Spec.Workflow.Parameters` using dotted paths

**New helper functions added** (package-level in `webhook_service.go`, reused by Task 4.2):
- `parseComponentWorkflowAnnotation(annotation string) map[string]string` — parses the comma-separated annotation
- `getNestedStringFromRawExtension(raw *runtime.RawExtension, dottedPath string) (string, error)` — navigates nested JSON by dotted path
- `setNestedValueInParameters(raw *runtime.RawExtension, dottedPath, value string) (*runtime.RawExtension, error)` — sets a value at a dotted path in JSON

---

### ✅ Task 4.2 — Fix triggerWorkflowInternal (remove systemParameters)

**File:** `internal/openchoreo-api/legacyservices/workflowrun_service.go` (moved from `component_workflow_service.go` in Task 7)

Method: `triggerWorkflowInternal` — **rewritten**

Changes:
1. Fetches the `Workflow` CR using `component.Spec.Workflow.Name`
2. Parses `AnnotationKeyComponentWorkflowParameters` annotation on the Workflow CR
3. Validates `repoUrl` is present in component parameters (via annotation path)
4. If `commit` key exists in annotation map, injects commit SHA into parameters at the mapped path using `setNestedValueInParameters`
5. Creates a `WorkflowRun` CR (**not** `ComponentWorkflowRun`) with:
   - Name: `{componentName}-workflow-{uuid}`
   - Labels: `openchoreo.dev/project = projectName`, `openchoreo.dev/component = componentName`
   - `Spec.Workflow.Name = component.Spec.Workflow.Name`
   - `Spec.Workflow.Parameters` = updated parameters with commit injected
6. Returns `models.ComponentWorkflowResponse`

Also updated:
- `TriggerWorkflow` authorization changed from `SystemActionCreateComponentWorkflow` / `ResourceTypeComponentWorkflow` to `SystemActionCreateWorkflowRun` / `ResourceTypeWorkflowRun`

---

### ✅ Task 4.3 — Audit APIs for issues after ComponentWorkflow/ComponentWorkflowRun deletion

**Status:** Resolved. All model types cleaned up in Task 3.3. All orphaned code removed in Task 6 and Task 7.

---

### ✅ Task 5 — Add project/component labels to WorkflowRun and support filtering

**Confirmed:** `triggerWorkflowInternal` (Task 4.2) already adds labels when creating
WorkflowRun from a component:
```go
Labels: map[string]string{
    ocLabels.LabelKeyProjectName:   projectName,   // "openchoreo.dev/project"
    ocLabels.LabelKeyComponentName: componentName,  // "openchoreo.dev/component"
},
```

**Added query param filtering to `ListWorkflowRuns`:**

| File | Change |
|------|--------|
| `internal/openchoreo-api/legacyservices/workflowrun_service.go` | `ListWorkflowRuns` now accepts `projectName` and `componentName` params; uses `client.MatchingLabels` to filter by `openchoreo.dev/project` and `openchoreo.dev/component` labels when provided. Added `ocLabels` import. |
| `internal/openchoreo-api/handlers/workflowruns.go` | `ListWorkflowRuns` handler reads optional `projectName` and `componentName` query params from URL and passes to service |
| `internal/openchoreo-api/api/handlers/workflows.go` | Updated `ListWorkflowRuns` call to pass empty strings for the new params (generated API doesn't expose these query params yet) |

---

### ✅ Task 6 — Remove orphaned ComponentWorkflow code from API server

**Cleaned up `component_workflow_service.go`:**

Removed all orphaned methods that referenced deleted `ComponentWorkflow`/`ComponentWorkflowRun`
types. The file was left with only:
- `ComponentWorkflowService` struct + `NewComponentWorkflowService`
- `TriggerWorkflow` (with authorization)
- `triggerWorkflowInternal` — core logic
- `generateShortUUID` — helper

These were subsequently moved to `WorkflowRunService` in Task 7 and the file was deleted.

**Deleted handler file:** `internal/openchoreo-api/handlers/component_workflows.go`
- All handlers orphaned after route removal in Task 3.1.

**Removed from `resource_crud.go`:**
- `CreateComponentWorkflowDefinition`, `GetComponentWorkflowDefinition`,
  `UpdateComponentWorkflowDefinition`, `DeleteComponentWorkflowDefinition` handlers

**Removed orphaned constants** (`constants.go`):
- `SystemActionViewComponentWorkflow`, `SystemActionCreateComponentWorkflow`,
  `SystemActionViewComponentWorkflowRun`, `ResourceTypeComponentWorkflow`,
  `ResourceTypeComponentWorkflowRun`

**Updated MCP handlers** (`mcphandlers/components.go`):
- Removed `ListComponentWorkflowsResponse` and `ListComponentWorkflowRunsResponse` types
- `ListComponentWorkflows`, `GetComponentWorkflowSchema`, `ListComponentWorkflowRuns` —
  converted to interface-satisfying stubs that return errors
- `TriggerComponentWorkflow` — updated to use `WorkflowRunService.TriggerWorkflow`

**Updated `api/handlers/component_workflows.go`** (OpenAPI-generated interface stubs):
- `ListComponentWorkflows` — returns empty list (interface required)
- `ListComponentWorkflowRuns` — returns empty list (interface required)
- `GetComponentWorkflowRun` — returns 404 (interface required)
- `CreateComponentWorkflowRun` — delegates to `WorkflowRunService.TriggerWorkflow`
- `UpdateComponentWorkflowParameters` — delegates to `ComponentService`
- `toModelsUpdateComponentWorkflowRequest` — simplified, only converts `Parameters`

---

### ✅ Task 7 — Remove component_workflow_service.go and move reusable functions

**File deleted:** `internal/openchoreo-api/legacyservices/component_workflow_service.go`

**Moved to `workflowrun_service.go`:**
- `TriggerWorkflow` method (now on `*WorkflowRunService`)
- `triggerWorkflowInternal` method (now on `*WorkflowRunService`)
- `generateShortUUID` helper function

**Updated references:**

| File | Change |
|------|--------|
| `internal/openchoreo-api/legacyservices/services.go` | Removed `ComponentWorkflowService` field from `Services` struct; removed `NewComponentWorkflowService` instantiation; `WebhookService` now receives `workflowRunService` instead of `componentWorkflowService` |
| `internal/openchoreo-api/legacyservices/webhook_service.go` | `workflowService` field type changed from `*ComponentWorkflowService` to `*WorkflowRunService`; `NewWebhookService` signature updated |
| `internal/openchoreo-api/mcphandlers/components.go` | `TriggerWorkflowRunForComponent` now calls `h.Services.WorkflowRunService.TriggerWorkflow` |
| `internal/openchoreo-api/api/handlers/component_workflows.go` | `CreateComponentWorkflowRun` now calls `h.services.WorkflowRunService.TriggerWorkflow` |

---

### ✅ Task 8 — Fix createComponentResources to remove system parameters

Handled as part of Task 3.3 above. All changes to `component_service.go`:
- `createComponentResources`: uses `WorkflowRunConfig{Name, Parameters}` directly
- `toComponentResponse`: returns `ComponentWorkflow{Name, Parameters}` without SystemParameters
- `UpdateComponentWorkflowParameters`: removed SystemParameters block
- `UpdateComponentWorkflowSchema`: uses `WorkflowRunConfig`, removed SystemParameters block
- `validateComponentWorkflowParameters` → `validateWorkflowParameters`: fetches `Workflow` CR

---

### ✅ Task 9 — Fix MCP tools for component workflow changes

**Status:** Completed

**What was implemented**

| Area | File(s) | Change |
|------|---------|--------|
| Interface cleanup | `pkg/mcp/tools/types.go` | Removed ComponentWorkflow-oriented methods from component/infrastructure interfaces. Added workflow-run methods (`CreateWorkflowRun`, `ListWorkflowRuns`, `GetWorkflowRun`, `GetWorkflowRunLogs`, `GetWorkflowRunEvents`) and renamed component trigger contract to `TriggerWorkflowRunForComponent`. |
| Component tool updates | `pkg/mcp/tools/component.go` | Removed `systemParameters` mapping from `create_component`. Removed deprecated component-workflow tools and added `trigger_component_workflow_run` that calls component-scoped workflow run trigger. |
| Infrastructure tool updates | `pkg/mcp/tools/infrastructure.go` | Added workflow run tools: `create_workflow_run`, `list_workflow_runs`, `get_workflow_run`, `get_workflow_run_logs`, `get_workflow_run_events`. Removed org-level component-workflow tools. |
| MCP registration wiring | `pkg/mcp/tools/register.go` | Replaced removed component-workflow registrations with new workflow-run registrations for infrastructure and component toolsets. |
| MCP handler wiring | `internal/openchoreo-api/mcphandlers/components.go`, `internal/openchoreo-api/mcphandlers/infrastructure.go`, `internal/openchoreo-api/mcphandlers/helpers.go`, `internal/openchoreo-api/handlers/handlers.go` | Added concrete handler methods for workflow run create/list/get/logs/events and component-triggered run. Passed `GatewayURL` into MCP handler so logs/events retrieval works through the gateway client. |
| Test/mocks updates | `pkg/mcp/tools/mock_test.go`, `pkg/mcp/tools/component_specs_test.go`, `pkg/mcp/tools/infrastructure_specs_test.go` | Removed component-workflow tool expectations and added workflow-run tool expectations/mocks. |

**Verification**
- `GOCACHE=/tmp/go-build go test ./pkg/mcp/tools`
- `GOCACHE=/tmp/go-build go test ./internal/openchoreo-api/mcphandlers ./internal/openchoreo-api/handlers`

---

### ✅ Task 10 — Remove component workflow related files from root `config/` folder

**Status:** Completed

**What was removed**
- Deleted CRD base manifests:
  - `config/crd/bases/openchoreo.dev_componentworkflows.yaml`
  - `config/crd/bases/openchoreo.dev_componentworkflowruns.yaml`
- Deleted RBAC helper roles:
  - `config/rbac/componentworkflow_admin_role.yaml`
  - `config/rbac/componentworkflow_editor_role.yaml`
  - `config/rbac/componentworkflow_viewer_role.yaml`
  - `config/rbac/componentworkflowrun_admin_role.yaml`
  - `config/rbac/componentworkflowrun_editor_role.yaml`
  - `config/rbac/componentworkflowrun_viewer_role.yaml`
- Deleted sample manifests:
  - `config/samples/v1alpha1_componentworkflow.yaml`
  - `config/samples/v1alpha1_componentworkflowrun.yaml`

**Kustomization / RBAC cleanup**
- Removed references from:
  - `config/crd/kustomization.yaml`
  - `config/rbac/kustomization.yaml`
  - `config/samples/kustomization.yaml`
- Removed `componentworkflows` and `componentworkflowruns` (including `/status` and `/finalizers`)
  from `config/rbac/role.yaml`.

**Validation**
- `kubectl kustomize config/crd` passes.
- `kubectl kustomize config/rbac` passes.

---

### ✅ Task 11 — Remove remaining usages of ComponentWorkflow / ComponentWorkflowRun

**Status:** Completed (core runtime/config paths)

**Controller and pipeline cleanup**
- Deleted controller packages:
  - `internal/controller/componentworkflow/`
  - `internal/controller/componentworkflowrun/`
- Deleted pipeline package:
  - `internal/pipeline/componentworkflow/`
- Removed controller wiring in `cmd/main.go` for the deleted reconciler/pipeline.

**Component controller migration**
- Migrated `internal/controller/component/*` from `ComponentWorkflow*` to `Workflow*`:
  - `validateComponentWorkflow` → `validateWorkflow`
  - watches now use `Workflow` + `WorkflowRun`
  - finalization deletes `WorkflowRun` objects instead of `ComponentWorkflowRun`
  - `WorkflowRun` ownership lookup now uses labels:
    - `openchoreo.dev/project`
    - `openchoreo.dev/component`

**Scaffold migration**
- Updated `internal/scaffold/component/generator.go` and tests to accept `Workflow`
  (not `ComponentWorkflow`).
- Removed scaffolder emission of `spec.workflow.systemParameters`.
- Updated fixtures:
  - `internal/scaffold/component/testdata/with_workflow_input.yaml`
  - `internal/scaffold/component/testdata/with_workflow_want.yaml`

**Helm/install alignment**
- Removed component workflow CRDs from helm package:
  - `install/helm/openchoreo-control-plane/crds/openchoreo.dev_componentworkflows.yaml`
  - `install/helm/openchoreo-control-plane/crds/openchoreo.dev_componentworkflowruns.yaml`
- Removed `componentworkflow*` RBAC resources from helm templates:
  - `install/helm/openchoreo-control-plane/templates/controller-manager/controller-manager-role.yaml`
  - `install/helm/openchoreo-control-plane/templates/openchoreo-api/clusterrole.yaml`
- Synced helm CRD copies with `config/crd/bases` for `components`, `componenttypes`,
  `clustercomponenttypes`, `componentreleases`, and `workflowruns`.
- Updated quick-start script to query `workflowrun` instead of `componentworkflowrun`:
  - `install/quick-start/build-deploy-greeter.sh`

**Validation**
- `GOCACHE=/tmp/go-build go test ./... -run TestNonExistent` passes.
- `GOCACHE=/tmp/go-build go test ./internal/scaffold/component` passes.

---

## Files Modified Summary

| File | Change | Status |
|------|--------|--------|
| `api/v1alpha1/componentworkflow_types.go` | Deleted entirely | ✅ |
| `api/v1alpha1/componentworkflowrun_types.go` | Deleted entirely | ✅ |
| `api/v1alpha1/workflowrun_types.go` | Added `ResourceReference`, `WorkflowTask` types | ✅ |
| `api/v1alpha1/component_types.go` | `Workflow` field changed from `*ComponentWorkflowRunConfig` to `*WorkflowRunConfig` | ✅ |
| `api/v1alpha1/zz_generated.deepcopy.go` | Regenerated via `make generate` | ✅ |
| `internal/controller/annotations.go` | Add `AnnotationKeyComponentWorkflowParameters` constant | ✅ |
| `internal/openchoreo-api/legacyservices/constants.go` | Add `SystemActionCreateWorkflow`; remove orphaned ComponentWorkflow constants | ✅ |
| `internal/openchoreo-api/legacyservices/workflow_service.go` | Add `AuthorizeCreate` method | ✅ |
| `internal/openchoreo-api/legacyservices/workflowrun_service.go` | Add `GetWorkflowRunLogs` + helpers; add label-based filtering to `ListWorkflowRuns`; add `TriggerWorkflow`/`triggerWorkflowInternal`/`generateShortUUID` (moved from deleted `component_workflow_service.go`) | ✅ |
| `internal/openchoreo-api/handlers/handlers.go` | Remove component-workflow routes (3.1); add `CreateWorkflowDefinition` and `GetWorkflowRunLogs` routes (3.2); wire MCP handler with `GatewayURL` for workflow run logs/events | ✅ |
| `internal/openchoreo-api/handlers/resource_crud.go` | Add `CreateWorkflowDefinition` handler; remove all ComponentWorkflow definition handlers | ✅ |
| `internal/openchoreo-api/handlers/workflowruns.go` | Add `GetWorkflowRunLogs` handler; add `projectName`/`componentName` query params to `ListWorkflowRuns` | ✅ |
| `internal/openchoreo-api/handlers/component_workflows.go` | **Deleted** — all handlers orphaned after route removal | ✅ |
| `internal/openchoreo-api/legacyservices/component_workflow_service.go` | **Deleted** — `TriggerWorkflow`/`triggerWorkflowInternal`/`generateShortUUID` moved to `workflowrun_service.go` | ✅ |
| `internal/openchoreo-api/legacyservices/webhook_service.go` | Rewrite `extractRepoInfoFromComponent`; add helper functions; update `workflowService` field type to `*WorkflowRunService` | ✅ |
| `internal/openchoreo-api/legacyservices/services.go` | Remove `ComponentWorkflowService` field and instantiation; pass `workflowRunService` to `NewWebhookService` | ✅ |
| `internal/openchoreo-api/mcphandlers/components.go` | Remove deprecated ComponentWorkflow MCP methods; add `TriggerWorkflowRunForComponent` mapped to `WorkflowRunService.TriggerWorkflow` | ✅ |
| `internal/openchoreo-api/mcphandlers/helpers.go` | Extend `MCPHandler` with `GatewayURL` for workflow run logs/events retrieval | ✅ |
| `internal/openchoreo-api/mcphandlers/infrastructure.go` | Add workflow run MCP handlers: create/list/get/logs/events | ✅ |
| `internal/openchoreo-api/api/handlers/component_workflows.go` | Stub orphaned interface methods; `CreateComponentWorkflowRun` uses `WorkflowRunService`; simplify converters | ✅ |
| `internal/openchoreo-api/api/handlers/components.go` | Remove `SystemParameters` from `toGenComponentWorkflowConfig` and `toModelComponentWorkflow` | ✅ |
| `internal/openchoreo-api/api/handlers/workflows.go` | Update `ListWorkflowRuns` call signature | ✅ |
| `internal/openchoreo-api/models/request.go` | Remove `SystemParameters` from `UpdateComponentWorkflowRequest` and `ComponentWorkflow`; remove `ComponentWorkflowSystemParams`, `ComponentWorkflowRepository`, `ComponentWorkflowRepositoryRevision` types | ✅ |
| `internal/openchoreo-api/models/response.go` | Remove `SystemParametersResponse`, `RepositoryResponse`, `RepositoryRevisionResponse`; simplify `ComponentWorkflowConfigResponse` | ✅ |
| `internal/openchoreo-api/legacyservices/component_service.go` | Remove `SystemParameters` usage in create/update/response methods; rename `validateComponentWorkflowParameters` → `validateWorkflowParameters` using `Workflow` CR | ✅ |
| `pkg/mcp/tools/types.go` | Remove ComponentWorkflow methods from interfaces; add workflow-run operations for infrastructure and component-scoped trigger contract | ✅ |
| `pkg/mcp/tools/component.go` | Remove systemParameters handling; replace component-workflow tools with `trigger_component_workflow_run` | ✅ |
| `pkg/mcp/tools/infrastructure.go` | Remove org-level ComponentWorkflow tools; add workflow run tools (`create/list/get/logs/events`) | ✅ |
| `pkg/mcp/tools/register.go` | Replace ComponentWorkflow registrations with workflow-run registrations | ✅ |
| `pkg/mcp/tools/mock_test.go` | Replace component-workflow mock methods with workflow-run mock methods | ✅ |
| `pkg/mcp/tools/component_specs_test.go` | Replace component workflow specs with component workflow-run trigger spec | ✅ |
| `pkg/mcp/tools/infrastructure_specs_test.go` | Replace component-workflow org-level specs with workflow-run specs | ✅ |

## Authorization Context (Post-Merge Check)

**Summary:** WorkflowRun authorization is currently mostly namespace-scoped for read paths. Project/component filters are implemented as list filters, not authorization boundaries.

**Current behavior**
- `ListWorkflowRuns`, `GetWorkflowRun`, `GetWorkflowRunLogs`, and `GetWorkflowRunEvents` authorize using `authz.ResourceHierarchy{Namespace: namespace}`.
- `TriggerWorkflow` (component-scoped run creation) authorizes with namespace + project + component hierarchy and then creates `WorkflowRun` with project/component labels.
- `list_workflow_runs` filters (`projectName`, `componentName`) are label filters only and do not enforce per-project/per-component visibility.

**Observed gaps**
- A developer with namespace-level `workflowrun:view` can view all workflow runs in that namespace.
- There is no per-run auth derivation from run labels during read operations.
- `TriggerWorkflow` fetches the component by `{namespace, name}` but does not verify that the component owner project matches the `projectName` in the request.
- Workflow definition handlers have auth for create, but get/update/delete handlers do not currently perform explicit auth checks.

**Suggested follow-up implementation**
- Resolve WorkflowRun read auth using run labels (`openchoreo.dev/project`, `openchoreo.dev/component`) and enforce project/component hierarchy per run.
- Validate `component.Spec.Owner.ProjectName == projectName` in `TriggerWorkflow`.
- Add explicit auth checks to workflow definition get/update/delete handlers.

### ✅ Task 13 — Rename workflow-runs endpoints to workflowruns

**Status:** Completed

**File:** `internal/openchoreo-api/handlers/handlers.go`

Deleted the old `workflow-runs` (hyphenated) route registrations. The new `workflowruns` (no hyphen) routes
were already added previously.

---

### ✅ Task 13.1 — CreateWorkflowRun accepts full YAML, no annotation dependency

**Status:** Completed

**Requirement:** `CreateWorkflowRun` should not depend on annotations. It should expect the whole
WorkflowRun YAML in the request body. It should not add any labels (caller's responsibility). Labels
from the request body should be extracted and used for authorization.

**Changes:**

| File | Change |
|------|--------|
| `internal/openchoreo-api/handlers/workflowruns.go` | Rewrote `CreateWorkflowRun` handler to accept raw YAML/JSON body (like `CreateWorkflowDefinition`). Decodes body as `map[string]interface{}`, validates `kind == "WorkflowRun"`, extracts `openchoreo.dev/project` and `openchoreo.dev/component` labels from the body for authorization, sets namespace from URL, and applies to Kubernetes via `applyToKubernetes`. No longer uses `models.CreateWorkflowRunRequest` or the service-layer `CreateWorkflowRun`. Returns `ResourceCRUDResponse`. |
| `internal/openchoreo-api/legacyservices/workflowrun_service.go` | Added `AuthorizeCreate(ctx, namespaceName, projectName, componentName)` method that builds `ResourceHierarchy` with optional project/component fields from labels. |

**Key design decisions:**
- The handler no longer constructs the WorkflowRun CR internally — the caller passes the complete resource.
- Labels are not injected by the handler; they must be set by the caller.
- Labels (`openchoreo.dev/project`, `openchoreo.dev/component`) are read from the submitted resource and
  used to build the authorization resource hierarchy, enabling project/component-scoped auth.
- The existing `WorkflowRunService.CreateWorkflowRun` method is preserved for other callers (MCP handlers,
  OpenAPI generated handlers) that still use the structured request model.

---

### ✅ Task 14 — Add workload creation logic to workflow run controller

**Status:** Completed

**Requirement:** Port the workload creation logic from the deleted `componentworkflowrun` controller
to the `workflowrun` controller. After a workflow succeeds and has a `generate-workload-cr` task,
the controller should extract the workload CR YAML from the Argo Workflow outputs and apply it.

**Changes:**

| File | Change |
|------|--------|
| `internal/controller/workflowrun/controller_conditions.go` | Added `ConditionWorkloadUpdated` condition type, `ReasonWorkloadUpdated` and `ReasonWorkloadUpdateFailed` reasons. Added `isWorkloadUpdated`, `setWorkloadUpdatedCondition`, `setWorkloadUpdateFailedCondition` functions. |
| `internal/controller/workflowrun/controller.go` | Added `isWorkloadUpdated` early return check in `Reconcile` (skip processing if workload already created). Replaced simple `isWorkflowCompleted` block with `handleCompletedWorkflow` which handles both timestamp setting and workload creation. Added `handleWorkloadCreation`, `createWorkloadFromWorkflowRun`, `extractWorkloadCRFromRunResource`, `hasGenerateWorkloadTask` functions ported from deleted componentworkflowrun controller. |

**Key flow:**
1. `Reconcile` checks `isWorkloadUpdated` early — if workload already applied, no-op.
2. `handleCompletedWorkflow` is called when workflow completes:
   - Sets `CompletedAt` timestamp.
   - If workflow succeeded AND has a `generate-workload-cr` task, calls `handleWorkloadCreation`.
3. `handleWorkloadCreation` → `createWorkloadFromWorkflowRun`:
   - Gets the Argo Workflow run resource from the build plane via `RunReference`.
   - Calls `extractWorkloadCRFromRunResource` to find the `workload-cr` output parameter
     from the `generate-workload-cr` step.
   - Unmarshals the YAML into a `Workload` CR and applies it via server-side apply
     with `FieldOwner("workflowrun-controller")`.
4. Sets `WorkloadUpdated` condition on success or `WorkloadUpdateFailed` on failure.

**Note:** `ImageStatus` tracking was not ported because the `WorkflowRun` status type does not
have an `ImageStatus` field (it was only on the deleted `ComponentWorkflowRun` type).

---

## Out of Scope (Follow-up PRs)

- Running `make manifests generate` to remove CRDs from generated YAML
- Updating `api/gen/` (OpenAPI generated code)
- Full compatibility-surface migration (observer/CLI/OpenAPI naming still uses `ComponentWorkflow*` terms in several places for backward compatibility)
- Task 12: SecretRef annotation support in workflow rendering
- Task 15: Update samples with Workflows
