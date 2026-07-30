# Plan: Merge ComponentWorkflow/ComponentWorkflowRun into Workflow/WorkflowRun

## Context

Currently OpenChoreo has two separate workflow CRD pairs (ComponentWorkflow/ComponentWorkflowRun and Workflow/WorkflowRun) with ~90% code overlap. The goal is to unify them into a single Workflow/WorkflowRun CRD pair, using an annotation (`backstage.io/component-workflow-parameters`) on the Workflow CR to indicate component-awareness.

Reference: `samples/try-out/2. component-workflows/discussion.md`

---

## Task 1: Update OpenChoreo API Endpoints

**Goal**: Replace all `component-workflows` API paths with unified `workflows` paths.

### Files to Change

- `internal/openchoreo-api/handlers/handlers.go` — Route registration
- `internal/openchoreo-api/handlers/component_workflows.go` — Handler implementations
- `internal/openchoreo-api/handlers/workflows.go` — Handler implementations (merge target)

### Steps

1. **Merge route registrations** in `handlers.go`:
   - Remove all `component-workflows` routes
   - Ensure the `workflows` routes cover both component-scoped and namespace-scoped use cases
   - New unified routes:

   ```
   # Workflow definition endpoints (namespace-scoped, PE-facing)
   POST   /v1/namespaces/{namespaceName}/workflows/definition
   GET    /v1/namespaces/{namespaceName}/workflows
   GET    /v1/namespaces/{namespaceName}/workflows/{wfName}/schema
   GET    /v1/namespaces/{namespaceName}/workflows/{wfName}/definition
   PUT    /v1/namespaces/{namespaceName}/workflows/{wfName}/definition
   DELETE /v1/namespaces/{namespaceName}/workflows/{wfName}/definition

   # Component workflow parameters (component-scoped, developer-facing)
   PATCH  /v1/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-parameters

   # Workflow run endpoints (component-scoped, developer-facing)
   POST   /v1/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs
   GET    /v1/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs
   GET    /v1/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs/{runName}
   GET    /v1/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs/{runName}/status
   GET    /v1/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs/{runName}/logs
   GET    /v1/namespaces/{namespaceName}/projects/{projectName}/components/{componentName}/workflow-runs/{runName}/events

   # Generic workflow run endpoints (namespace-scoped)
   POST   /v1/namespaces/{namespaceName}/workflow-runs
   GET    /v1/namespaces/{namespaceName}/workflow-runs
   GET    /v1/namespaces/{namespaceName}/workflow-runs/{runName}
   ```

2. **Merge handler logic**:
   - Consolidate `component_workflows.go` handler functions into `workflows.go`
   - The component-scoped handlers (workflow-runs under a component) should use the unified Workflow/WorkflowRun CRDs but filter by project/component labels
   - The underlying service layer should read the `backstage.io/component-workflow-parameters` annotation to determine if a Workflow is component-aware
   - Remove or deprecate the `ComponentWorkflowService` in favor of `WorkflowService`

3. **Update service layer** (if separate service files exist under `internal/openchoreo-api/`):
   - Merge `ComponentWorkflowService` logic into `WorkflowService`
   - Ensure WorkflowRun creation populates project/component labels when invoked from component-scoped endpoints

4. **Update generated API models** (if OpenAPI/gen types exist):
   - Unify `gen.ComponentWorkflowTemplate` → `gen.Workflow`
   - Unify `gen.ComponentWorkflowRun` → `gen.WorkflowRun`
   - Update converter functions (`toGen*`) accordingly
5. When a run is created from a component, it should add the following labels to the WorkflowRun for filtering:
   ```
   openchoreo.dev/project-name: <projectName>
   openchoreo.dev/component-name: <componentName>
   ```
---

## Task 2: SecretRef via Annotation on Workflow CR

**Goal**: The WorkflowRun controller resolves SecretReference CRs using a `secretRef` annotation on the Workflow CR and injects resolved data into the CEL context.

### Files to Change

- `api/v1alpha1/workflow_types.go` — No CRD schema changes needed (annotation-based)
- `internal/controller/workflowrun/controller.go` — Add secret resolution logic

### Steps

1. **Define annotation convention**:
   - Annotation key: `openchoreo.dev/secret-ref` (or reuse the existing `backstage.io/component-workflow-parameters` secretRef mapping)
   - Annotation value: Name of the SecretReference CR in the same namespace
   - Example: `openchoreo.dev/secret-ref: "reading-list-repo-credentials-dev"`

2. **Port secret resolution to WorkflowRun controller**:
   - Extract `resolveGitSecret()` logic from `componentworkflowrun/controller.go` (lines 740-783) into a shared utility or directly into the WorkflowRun controller
   - In `workflowrun/controller.go`, before rendering the workflow template:
     a. Read the Workflow CR's annotations
     b. If `openchoreo.dev/secret-ref` annotation exists, fetch the SecretReference CR
     c. Resolve secret data (type, key, remoteKey, property)
     d. Inject `secretRef.*` variables into the CEL rendering context
   - If the annotation references a secretRef field path from `backstage.io/component-workflow-parameters`, resolve it from the WorkflowRun's parameter values instead

3. **Update CEL context**:
   - Add `secretRef` to the WorkflowRun rendering context with the same structure as ComponentWorkflowRun:
     - `secretRef.type` — Secret type from SecretReference template
     - `secretRef.key` — Secret key
     - `secretRef.remoteKey` — Remote reference key
     - `secretRef.property` — Remote reference property
   - Support `${has(secretRef)}` or similar CEL expressions for conditional resource rendering (`includeWhen`)

4. **Port component-specific features to WorkflowRun controller** (as needed):
   - **AllowedWorkflows validation**: When WorkflowRun is created from a component-scoped endpoint (has project/component labels), validate against ComponentType's `allowedWorkflows`
   - **Image status tracking**: Add `ImageStatus` to WorkflowRun status if the Workflow is component-aware
   - **Workload auto-generation**: If the Workflow is component-aware and the run succeeds, auto-create Workload CR

5. **Error handling**:
   - SecretReference not found → Set "SecretResolutionFailed" condition, don't requeue
   - Empty data sources → Requeue for transient errors
   - Missing annotation → Skip secret resolution (backward compatible)

---

## Task 3: Update Samples

**Goal**: Convert all ComponentWorkflow samples to use the unified Workflow CRD with annotation-based component awareness.

### Files to Change

- `samples/getting-started/component-workflows/docker.yaml`
- `samples/getting-started/component-workflows/react.yaml`
- `samples/getting-started/component-workflows/ballerina-buildpack.yaml`
- `samples/getting-started/component-workflows/google-cloud-buildpacks.yaml`
- `samples/component-workflows/docker.yaml`
- `samples/component-workflows/react.yaml`
- `samples/component-workflows/google-cloud-buildpacks.yaml`

### Steps

1. **Convert each ComponentWorkflow sample to Workflow format**:
   - Change `kind: ComponentWorkflow` → `kind: Workflow`
   - Remove the structured `systemParameters` schema section
   - Move system parameter fields (repository.url, secretRef, revision, appPath) into the flat `parameters` schema
   - Add scope parameters (projectName, componentName) to the `parameters` schema
   - Add `backstage.io/component-workflow-parameters` annotation with field mappings:
     ```yaml
     annotations:
       backstage.io/component-workflow-parameters: >-
         repository: parameters.repository.url,
         branch: parameters.repository.revision.branch,
         appPath: parameters.repository.appPath,
         secretRef: parameters.repository.secretRef,
         projectName: parameters.scope.projectName,
         componentName: parameters.scope.componentName
     ```

2. **Update template variable references**:
   - `${systemParameters.repository.url}` → `${parameters.repository.url}`
   - `${systemParameters.repository.revision.branch}` → `${parameters.repository.revision.branch}`
   - `${systemParameters.repository.revision.commit}` → `${parameters.repository.revision.commit}`
   - `${systemParameters.repository.appPath}` → `${parameters.repository.appPath}`
   - `${metadata.componentName}` → `${parameters.scope.componentName}`
   - `${metadata.projectName}` → `${parameters.scope.projectName}`
   - `${has(systemParameters.repository.secretRef)}` → `${has(parameters.repository.secretRef)}`

3. **Update `includeWhen` expressions** in resource templates:
   - `${has(systemParameters.repository.secretRef)}` → use the equivalent CEL expression for the unified schema

4. **Update any corresponding WorkflowRun samples**:
   - Change `kind: ComponentWorkflowRun` → `kind: WorkflowRun`
   - Add labels for filtering:
     ```yaml
     labels:
       openchoreo.dev/project-name: "my-project"
       openchoreo.dev/component-name: "my-component"
     ```
   - Move `systemParameters` values into `parameters`

5. **Rename directories** (optional, discuss with team):
   - Consider renaming `samples/getting-started/component-workflows/` → `samples/getting-started/workflows/`
   - Or keep the directory name for backward compatibility and add a note

6. **Update README.md** in `samples/component-workflows/README.md` to reflect the unified Workflow CRD

---

## Task Order & Dependencies

```
Task 2 (SecretRef + controller merge) → Task 1 (API endpoints) → Task 3 (Samples)
```

- Task 2 should be done first since it establishes the core controller changes
- Task 1 depends on the unified CRDs being in place
- Task 3 can be done in parallel with Task 1 but should validate against the new API

## Out of Scope (for now)

- Removing ComponentWorkflow/ComponentWorkflowRun CRD definitions from `api/v1alpha1/` (deprecation period)
- Removing ComponentWorkflow/ComponentWorkflowRun controllers (keep for backward compatibility initially)
- Updating `ComponentType.AllowedWorkflows` field documentation to reference Workflow instead of ComponentWorkflow
- Long-term SecretReference design (this plan uses the interim annotation-based approach)