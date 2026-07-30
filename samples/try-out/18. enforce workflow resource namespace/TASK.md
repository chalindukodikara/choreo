# Task: Enforce Workflow Resource Namespace

**Status: IN PROGRESS**

## Context

Workflow and ClusterWorkflow CRDs have a `resources[]` field that defines additional Kubernetes resources (secrets, configmaps, etc.) deployed alongside the workflow run. These resources must always be applied to the enforced workflow execution namespace (`workflows-<namespaceName>`) and workflow authors must not be able to deploy resources into arbitrary namespaces.

## What Was Done

### Runtime Enforcement in the Rendering Pipeline - DONE

The workflow rendering pipeline (`internal/pipeline/workflow/pipeline.go`) now force-sets the `metadata.namespace` on all rendered resources to the enforced namespace, regardless of what the template specifies. This is a defense-in-depth measure that guarantees resources always land in the correct namespace after CEL evaluation.

### What Remains

### Admission Webhook Validation - TODO (separate PR)

Add validating webhooks for Workflow and ClusterWorkflow that inspect each `resources[].template` and validate that `metadata.namespace` is either:
- Absent (the pipeline will set it)
- Set to `${metadata.namespace}` (the expected CEL expression)
- Reject any other value

This provides early feedback at admission time rather than silently overriding at runtime. Note: CRD-level CEL validation (`XValidation`) is **not possible** here because:
1. `Template` is a `*runtime.RawExtension` with `PreserveUnknownFields` -- opaque to the structural schema
2. Template values contain unevaluated CEL expressions at admission time

The webhook approach works because Go code can unmarshal the raw JSON and inspect the namespace field directly.

**Files to create/modify:**
- `internal/webhook/workflow/webhook.go` -- validating webhook for Workflow
- `internal/webhook/clusterworkflow/webhook.go` -- validating webhook for ClusterWorkflow
- `cmd/main.go` -- register webhooks with the manager
- `config/webhook/` -- webhook configuration manifests

## Important Considerations

1. **runTemplate namespace**: Consider whether the same enforcement should apply to `runTemplate.metadata.namespace`. Currently runTemplate is not enforced at the pipeline level either.
2. **Webhook pattern**: Follow the existing webhook pattern in `internal/webhook/component/webhook.go`.
3. **Error messages**: Provide clear error messages guiding the user to use `${metadata.namespace}` or omit the namespace field.