# Phase 1 Implementation: Remove `ocSchema` from Public API

## What changed

### 1. CRD Type Definition — `api/v1alpha1/componenttype_types.go`

**Before:**
```go
type SchemaSection struct {
    OCSchema       *runtime.RawExtension `json:"ocSchema,omitempty"`
    OpenAPIV3Schema *runtime.RawExtension `json:"openAPIV3Schema,omitempty"`
}
```

**After:**
```go
type SchemaSection struct {
    OpenAPIV3Schema *runtime.RawExtension `json:"openAPIV3Schema,omitempty"`
}
```

- Removed `OCSchema` field
- Removed kubebuilder `XValidation` rule for mutual exclusivity
- Simplified `GetRaw()` — now directly returns `OpenAPIV3Schema`
- Kept `IsOpenAPIV3()` — still used by `definition.go` (will be removed in Phase 2)

### 2. OpenAPI Spec — `openapi/openchoreo-api.yaml`

Removed `ocSchema` property from the `SchemaSection` component schema definition. Updated description.

**Before:**
```yaml
SchemaSection:
  type: object
  description: Schema section supporting either ocSchema or openAPIV3Schema format (mutually exclusive)
  properties:
    ocSchema:
      type: object
      description: Schema using OpenChoreo's simple schema format...
      additionalProperties: true
    openAPIV3Schema:
      type: object
      description: Schema using standard OpenAPI V3 / JSON Schema format
      additionalProperties: true
```

**After:**
```yaml
SchemaSection:
  type: object
  description: Schema section using openAPIV3Schema format
  properties:
    openAPIV3Schema:
      type: object
      description: Schema using standard OpenAPI V3 / JSON Schema format
      additionalProperties: true
```

### 3. Schema Processing — `internal/schema/definition.go`

Replaced field-based format detection (`section.IsOpenAPIV3()`) with content-based detection (`isOpenAPIV3Content(fields)`) in four functions:

- `ResolveSectionToStructural()`
- `ResolveSectionToBundle()`
- `SectionToJSONSchema()`
- `SectionToRawJSONSchema()`

**Why:** After removing the `OCSchema` field, `IsOpenAPIV3()` always returns true. But many internal test fixtures still use shorthand values (e.g., `replicas: "integer | default=1"`) under the `openAPIV3Schema:` YAML key. Content-based detection routes shorthand content to the extractor, and proper JSON Schema content to the OpenAPI V3 parser.

**New helper:** `isOpenAPIV3Content(fields)` checks if any top-level key is a recognized JSON Schema keyword (from a comprehensive set including `type`, `properties`, `allOf`, `oneOf`, `anyOf`, `$ref`, `enum`, `const`, Kubernetes extensions, etc.). Empty schemas are treated as OpenAPI V3. If no JSON Schema keyword is found, content is routed to the shorthand extractor.

This is a temporary bridge for Phase 1. Phase 2 will convert all test data to proper OpenAPI V3 and remove this function.

### 4. Validation Utilities — `internal/validation/schemautil/extract.go`

- Removed `validateSchemaFormatConsistency()` function entirely — no longer two formats that can mismatch
- Removed the call to it from `ExtractStructuralSchemas()`
- Removed corresponding test cases from `extract_test.go` (`TestExtractStructuralSchemas_FormatConsistency`)

### 5. Comment Updates

Updated comments that referenced "handles both ocSchema and openAPIV3Schema" in:
- `internal/openchoreo-api/legacyservices/clustertrait_service.go`
- `internal/openchoreo-api/legacyservices/clustercomponenttype_service.go`
- `internal/openchoreo-api/legacyservices/trait_service.go`
- `internal/openchoreo-api/legacyservices/componenttype_service.go`
- `internal/openchoreo-api/legacyservices/workflow_service.go`
- `internal/openchoreo-api/legacyservices/component_service.go`
- `internal/webhook/componentrelease/webhook.go`
- `internal/pipeline/workflow/pipeline.go`
- `internal/pipeline/component/context/component.go`

### 6. Test Fixture Updates

Replaced `OCSchema:` with `OpenAPIV3Schema:` in Go struct literals across ~22 test files:

**Webhook tests:**
- `internal/webhook/componentrelease/webhook_test.go`
- `internal/webhook/trait/webhook_test.go`
- `internal/webhook/componenttype/webhook_test.go`
- `internal/webhook/clustertrait/webhook_test.go`
- `internal/webhook/clustercomponenttype/webhook_test.go`

**API service tests:**
- `internal/openchoreo-api/services/trait/service_test.go`
- `internal/openchoreo-api/services/componenttype/service_test.go`
- `internal/openchoreo-api/services/clustercomponenttype/service_test.go`
- `internal/openchoreo-api/services/clustertrait/service_test.go`
- `internal/openchoreo-api/services/component/service_test.go`
- `internal/openchoreo-api/api/handlers/cluster_scoped_handlers_test.go`
- `internal/openchoreo-api/legacyservices/clustertrait_service_test.go`
- `internal/openchoreo-api/legacyservices/clustercomponenttype_service_test.go`

**Controller & other tests:**
- `internal/schema/definition_test.go`
- `internal/validation/schemautil/extract_test.go`
- `internal/pipeline/component/context/embedded_trait_test.go`
- `internal/pipeline/workflow/pipeline_test.go`
- `internal/controller/clustertrait/controller_test.go`
- `internal/controller/componentrelease/controller_test.go`
- `test/e2e/suites/connections/connections_fixtures_test.go`
- `test/e2e/suites/networkpolicy/networkpolicy_fixtures_test.go`

### 7. YAML Key Renames (`ocSchema:` → `openAPIV3Schema:`)

Renamed the YAML key in all test data and sample files. **Note:** The values remain in shorthand format — Phase 2 will convert them to proper OpenAPI V3.

**Pipeline testdata:**
- `internal/pipeline/component/pipeline_test.go` (19 inline YAML blocks)
- `internal/pipeline/component/benchmark_test.go` (2 inline YAML blocks)
- `internal/pipeline/component/context/builder_test.go` (7 inline YAML blocks)
- `internal/pipeline/component/testdata/component-with-traits.yaml`
- `internal/pipeline/component/testdata/configurations-and-secrets/snapshot.yaml`
- `internal/pipeline/component/testdata/configurations-and-secrets/snapshot-with-config-helpers.yaml`

**Scaffold testdata (11 files):**
- `internal/scaffold/component/testdata/basic_types_input.yaml`
- `internal/scaffold/component/testdata/collection_shapes_input.yaml`
- `internal/scaffold/component/testdata/object_defaults_input.yaml`
- `internal/scaffold/component/testdata/validation_and_types_input.yaml`
- `internal/scaffold/component/testdata/with_traits_input.yaml`
- `internal/scaffold/component/testdata/with_workflow_input.yaml`
- `internal/scaffold/component/testdata/map_scaffolding_input.yaml`
- `internal/scaffold/component/testdata/minimal_comments_false_input.yaml`
- `internal/scaffold/component/testdata/minimal_comments_true_input.yaml`
- `internal/scaffold/component/testdata/escaping_quoting_input.yaml`
- `internal/scaffold/component/testdata/array_scaffolding_input.yaml`

**Sample YAML files (21 files):**
- `config/samples/openchoreo_v1alpha1_clustercomponenttype.yaml`
- `config/samples/openchoreo_v1alpha1_clustertrait.yaml`
- `config/samples/openchoreo_v1alpha1_componenttype.yaml`
- `samples/ocSchema/` — all 11 files
- `samples/private-repo-registry/` — 3 files
- `samples/private-registry/` — 2 files
- `samples/component-workflows/advanced-schema/google-cloud-buildpacks.yaml`
- `samples/01-private-repos/component-workflow.yaml`

### 8. Documentation

- **Deleted:** `docs/templating/openchoreo-schema.md` (ocSchema user guide)
- **Updated:** `docs/templating/openapiv3-schema.md` — removed "alternative format" callout referencing ocSchema
- **Updated:** `docs/templating/validations.md` — renamed `ocSchema:` → `openAPIV3Schema:` in examples

### 9. Generated Code

Regenerated via:
```bash
make generate manifests    # deepcopy + CRDs
make openapi-codegen       # models.gen.go
make helm-generate         # Helm chart CRDs
make samples-gen           # combined sample YAML
```

All `ocSchema` references removed from:
- `api/v1alpha1/zz_generated.deepcopy.go`
- `internal/openchoreo-api/api/gen/models.gen.go`
- `config/crd/bases/*.yaml` (7 CRDs)
- `install/helm/openchoreo-control-plane/crds/*.yaml` (7 CRDs)

### 10. Test Name Cleanup

Renamed test names/function names that referenced `ocSchema`/`OCSchema`:
- `TestSectionToJSONSchema_OCSchema` → `TestSectionToJSONSchema_ShorthandSchema`
- `"success with OCSchema params"` → `"success with shorthand schema params"`
- `"success with OCSchema"` → `"success with shorthand schema"`
- `"valid ocSchema"` → `"valid schema"`
- `"invalid ocSchema"` → `"invalid schema"`
- `"ocSchema applies defaults..."` → `"schema applies defaults..."`
- Webhook test names: `"spec.parameters.ocSchema"` → `"spec.parameters.openAPIV3Schema"`

## Verification

```
make test       ✅ All unit tests pass
make lint-fix   ✅ 0 issues
make go.build   ✅ All 6 binaries build (manager, occ, openchoreo-api, observer, cluster-gateway, cluster-agent)
```

## Deferred: Website documentation

The following files still reference `ocSchema` and need updating separately:
- `website/docs/user-guide/workflows/workflow-schema.md` (~11 references)
- `website/docs/reference/api/platform/workflow.md` (~7 references)
- `samples/workflows/ci/README.md` (~2 references)

## What remains (Phase 2)

The shorthand schema extractor (`internal/schema/extractor/`) and its routing logic (`isOpenAPIV3Content()` in `definition.go`) still exist. Many test YAML values are still in shorthand format under `openAPIV3Schema:`. Phase 2 will:

1. Convert all shorthand test values to proper OpenAPI V3 JSON Schema
2. Remove `isOpenAPIV3Content()` content detection
3. Remove the extractor (`internal/schema/extractor/`)
4. Remove `ToJSONSchema()`, `ToStructural()`, `ToStructuralAndJSONSchema()` shorthand entry points
5. Remove `IsOpenAPIV3()` method from `SchemaSection`
