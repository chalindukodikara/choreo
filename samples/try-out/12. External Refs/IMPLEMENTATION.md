# Implementation Notes: `externalRefs` in Workflow

## What Was Done

Added an `externalRefs` field to the Workflow spec that allows declarative references to external CRs (starting with `SecretReference`). The controller resolves these at runtime and injects their full spec into the CEL context.

## Design Decisions

### Field name: `externalRefs`
- Self-describing: immediately conveys "references to resources external to this Workflow"
- Follows K8s `Ref` convention (consistent with `buildPlaneRef`, `secretRef`, etc.)
- Other candidates considered: `contextRefs`, `refs`, `references`, `lookups`, `inputs`
- `contextRefs` was rejected because "context" is overloaded in K8s and requires insider knowledge of the CEL rendering internals

### Flat structure (no nested `ref` sub-field)
- Since the array is already called `externalRefs`, nesting under `ref` would be redundant (`externalRefs[].ref`)
- Fields: `id`, `apiVersion`, `kind`, `name`

### Container map for CEL access
- CEL variable names cannot contain hyphens (`git-secret-reference` is parsed as `git - secret - reference`)
- External refs are stored under an `externalRefs` map, accessed via `${externalRefs['git-secret-reference'].spec.template.type}`
- This preserves K8s-idiomatic hyphenated IDs while working within CEL's syntax constraints

### Whole spec exposed
- The entire `SecretReference.Spec` is marshaled to map and injected (not just selected fields like the old `secretRef` approach)
- Users can access any field: `externalRefs['id'].spec.template.type`, `externalRefs['id'].spec.data`, etc.

### Empty name = silent skip
- If an externalRef's `name` evaluates to empty/nil (e.g., `${parameters.repository.secretRef}` when secretRef param is empty), the ref is silently skipped
- No `includeWhen` needed on the externalRef itself — the controller handles this automatically

### Kind restricted to `SecretReference`
- Kubebuilder `Enum=SecretReference` validation on the `kind` field
- Extensible to other CR types in the future by expanding the enum

## Files Changed

### CRD & Types
- `api/v1alpha1/workflow_types.go` — Added `ExternalRef` struct and `ExternalRefs []ExternalRef` field to `WorkflowSpec`
- `api/v1alpha1/zz_generated.deepcopy.go` — Auto-generated
- `config/crd/bases/openchoreo.dev_workflows.yaml` — Auto-generated

### Controller
- `internal/controller/workflowrun/externalref.go` — **New** resolver: `resolveExternalRefs()` evaluates CEL names, fetches CRs, returns full spec as map
- `internal/controller/workflowrun/controller.go` — Replaced annotation-based `secretRef` resolution with `externalRefs` resolution; uses `BuildCELContext` for preliminary context
- `internal/controller/workflowrun/secretref.go` — Removed `resolveSecretRefInfo` (kept `getNestedStringFromRawExtension` for legacy services)

### Pipeline
- `internal/pipeline/workflow/types.go` — Replaced `SecretRef *SecretRefInfo` with `ExternalRefs map[string]any`; removed `SecretRefInfo`, `SecretDataInfo`, `RemoteRefInfo`
- `internal/pipeline/workflow/pipeline.go` — Exported `BuildCELContext`; injects externalRefs as container map

### Tests
- `internal/controller/workflowrun/externalref_test.go` — **New** 7 test cases (happy path, multi-ref, empty name skip, not-found, unsupported kind, nil input, full spec access)
- `internal/pipeline/workflow/pipeline_test.go` — Updated old `secretRef` tests to use `externalRefs` container map
- `internal/controller/workflowrun/controller_unit_test.go` — Removed `TestResolveSecretRefInfo`

### OpenAPI
- `openapi/openchoreo-api.yaml` — Added `ExternalRef` schema and `externalRefs` field to `WorkflowSpec`

### Samples Updated
- `samples/getting-started/workflows/docker.yaml`
- `samples/getting-started/workflows/google-cloud-buildpacks.yaml`
- `samples/getting-started/workflows/react.yaml`
- `samples/getting-started/workflows/ballerina-buildpack.yaml`
- `samples/getting-started/all.yaml`
- `samples/component-workflows/docker.yaml`
- `samples/component-workflows/google-cloud-buildpacks.yaml`
- `samples/component-workflows/react.yaml`
- `samples/try-out/12. External Refs/cr.yaml`

### Samples NOT Updated (intentionally)
- `samples/try-out/8. add labels to cel context/` — Workflow kind but still uses old `secretRef` (partially updated, pending)
- `samples/try-out/1. git-secret-creation/component-workflows/` — Uses `kind: ComponentWorkflow` (old CRD, not in scope)
- `samples/try-out/4. workload-creation-optional/` — Uses `kind: ComponentWorkflow`
- `samples/01-private-repos/` — Uses `kind: ComponentWorkflow`

## Migration Pattern

Before (annotation-based):
```yaml
metadata:
  annotations:
    openchoreo.dev/component-workflow-parameters: |
      secretRef: parameters.repository.secretRef
spec:
  resources:
    - id: git-secret
      template:
        spec:
          target:
            template:
              type: ${secretRef.type}
          data: |
            ${secretRef.data.map(secret, { ... })}
```

After (externalRefs):
```yaml
spec:
  externalRefs:
    - id: git-secret-reference
      apiVersion: openchoreo.dev/v1alpha1
      kind: SecretReference
      name: ${parameters.repository.secretRef}
  resources:
    - id: git-secret
      template:
        spec:
          target:
            template:
              type: ${externalRefs['git-secret-reference'].spec.template.type}
          data: |
            ${externalRefs['git-secret-reference'].spec.data.map(secret, { ... })}
```

## Remaining Work

- The `secretRef` annotation line (`secretRef: parameters.repository.secretRef`) was removed from all updated samples; the annotation mechanism is still supported for backwards compatibility via legacy services but new workflows should use `externalRefs`
