# Add Admission Controller (Webhook) for Workflow and ClusterWorkflow

Add validating and defaulting webhooks for the `Workflow` and `ClusterWorkflow` CRDs. Currently no webhook exists for either resource. The WorkflowRun controller assumes certain fields are present at runtime (e.g., `serviceAccountName`, `metadata.namespace`) and will fail if they're missing — these must be enforced or injected at admission time.

## Reference Files

| Purpose | Path |
|---|---|
| Workflow type definitions | `api/v1alpha1/workflow_types.go` |
| ClusterWorkflow type definitions | `api/v1alpha1/clusterworkflow_types.go` |
| WorkflowRun controller (consumer of Workflow) | `internal/controller/workflowrun/controller.go` |
| WorkflowRun run engine (SA creation) | `internal/controller/workflowrun/run_engine.go` |
| Example webhook to follow (validation-only) | `internal/webhook/trait/webhook.go` |
| Example webhook to follow (with defaulting) | `internal/webhook/releasebinding/webhook.go` |
| Webhook registration in main | `cmd/main.go` (lines ~486-538) |
| Sample Workflow resources | `samples/getting-started/ci-workflows/*` |
| Resource structure validation utility | `internal/validation/component/` (`ValidateResourceTemplateStructure`) |
| Schema extraction utility | `internal/validation/schemautil/` (`ExtractStructuralSchemas`) |

## Files to Create

- `internal/webhook/workflow/webhook.go` — Validator + Defaulter for namespace-scoped `Workflow`
- `internal/webhook/clusterworkflow/webhook.go` — Validator + Defaulter for cluster-scoped `ClusterWorkflow`
- Unit tests: `internal/webhook/workflow/webhook_test.go`, `internal/webhook/clusterworkflow/webhook_test.go`
- Test suites: `internal/webhook/workflow/suite_test.go`, `internal/webhook/clusterworkflow/suite_test.go`

## Files to Modify

- `cmd/main.go` — Register both webhooks in the `ENABLE_WEBHOOKS` block (follow existing pattern at lines 486-538)

---

## 1. Validations (ValidateCreate + ValidateUpdate)

Apply these on both `Workflow` and `ClusterWorkflow`.

### 1.1 RunTemplate `metadata.namespace` must be `${metadata.namespace}`

The `runTemplate` is a `runtime.RawExtension` containing an arbitrary K8s resource. Unmarshal it and verify:

- `runTemplate.metadata.namespace` exists and equals the literal string `${metadata.namespace}`

This ensures the workflow run always executes in the correct namespace (the CEL expression resolves to the WorkflowRun's namespace at runtime). **For ClusterWorkflow**, this validation still applies since cluster-scoped workflows still render into a namespace at runtime.

**Why:** The WorkflowRun controller extracts namespace at `controller.go:249` via `extractRunResourceNamespace()`. If missing or wrong, the run resource lands in the wrong namespace or the controller errors.

### 1.2 Resources `metadata.namespace` must be `${metadata.namespace}`

For each entry in `spec.resources[]`, unmarshal `resources[i].template` and verify:

- `template.metadata.namespace` exists and equals the literal string `${metadata.namespace}`

**Why:** Same reason — auxiliary resources (ExternalSecrets, ConfigMaps) must deploy to the same namespace as the run.

### 1.3 RunTemplate must have valid K8s resource structure

Validate that `runTemplate` contains the required K8s resource fields:

- `apiVersion` — must be present and non-empty
- `kind` — must be present and non-empty
- `metadata.name` — must be present and non-empty

Use the existing `ValidateResourceTemplateStructure()` from `internal/validation/component/` if compatible, or replicate the same checks.

### 1.4 Resources templates must have valid K8s resource structure

Same validation as 1.3 for each `spec.resources[].template`.

### 1.5 Parameters schema validation

If `spec.parameters` is provided, validate the schema is well-formed:

- Use `schemautil.ExtractStructuralSchemas(parameters, nil, basePath)` to validate `openAPIV3Schema` syntax (we dont support "ocSchema")
- Follow the same pattern as the Trait webhook (`internal/webhook/trait/webhook.go:53-56`)

### 1.6 Resource ID uniqueness

Validate that all `spec.resources[].id` values are unique within the workflow. Use `field.Duplicate()` for error reporting.

### 1.7 ExternalRef ID uniqueness

Validate that all `spec.externalRefs[].id` values are unique. (Already enforced by `+listMapKey=id` in CRD, but defense-in-depth at webhook level is good practice.)

### 1.8 ClusterWorkflow scoping constraint

For `ClusterWorkflow` only: validate that `spec.workflowPlaneRef.kind` is `ClusterWorkflowPlane` (not `WorkflowPlane`). Follow the same pattern as `ClusterComponentType` webhook which validates that embedded traits are `ClusterTrait`.

---

## 2. Mutation / Defaulting (CustomDefaulter or admission.Handler)

### 2.1 Inject `serviceAccountName: workflow-sa` into RunTemplate

The WorkflowRun controller creates a ServiceAccount named `workflow-sa` via `ensurePrerequisites()` in `run_engine.go:37`. It then extracts `spec.serviceAccountName` from the rendered run resource at `controller.go:267` via `extractServiceAccountName()` — if missing or empty, the controller returns an error.

**Mutation:** Unmarshal `runTemplate`, set `spec.serviceAccountName` to `"workflow-sa"` (overwrite any existing value), and marshal it back. This ensures consistency — the SA name must match what the controller creates.

**Implementation options:**
- Use `webhook.CustomDefaulter` interface (simpler, preferred if no need to access old object)
- Or use a custom `admission.Handler` like `internal/webhook/releasebinding/webhook.go` if you need access to the request context

---

## 3. Implementation Pattern

Follow the existing webhook conventions:

```go
// Kubebuilder markers for code generation:
// +kubebuilder:webhook:path=/validate-openchoreo-dev-v1alpha1-workflow,...
// +kubebuilder:webhook:path=/mutate-openchoreo-dev-v1alpha1-workflow,...

func SetupWorkflowWebhookWithManager(mgr ctrl.Manager) error {
    return ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.Workflow{}).
        WithValidator(&Validator{}).
        WithDefaulter(&Defaulter{}).
        Complete()
}
```

Registration in `cmd/main.go`:
```go
if os.Getenv("ENABLE_WEBHOOKS") != "false" {
    if err := workflowwebhook.SetupWorkflowWebhookWithManager(mgr); err != nil {
        setupLog.Error(err, "unable to create webhook", "webhook", "Workflow")
        os.Exit(1)
    }
}
```

Use `field.ErrorList` with proper field paths for all validation errors (e.g., `field.NewPath("spec", "runTemplate", "metadata", "namespace")`).

---

## 4. Post-Implementation Checklist

- [ ] Run `make manifests` to regenerate webhook config from kubebuilder markers
- [ ] Run `make generate` if any types were modified
- [ ] Run `make test` to verify unit/integration tests pass
- [ ] Run `make lint-fix` to fix formatting/lint issues
- [ ] Run `make go.build` to verify compilation
- [ ] Verify the sample workflows in `samples/getting-started/ci-workflows/` pass validation (they should — they already have `namespace: ${metadata.namespace}` and `serviceAccountName: workflow-sa`)
