# Implementation: OpenAPI V3 Schema Support for OpenChoreo

## Summary

This document tracks the implementation of dual schema format support (`ocSchema` and `openAPIV3Schema`) for OpenChoreo CRDs. The design moves `parameters` and `environmentConfigs` to top-level fields under `spec`, with each section independently declaring its schema format.

See [Discussion.md](./Discussion.md) for design rationale and [TASK2.md](./TASK2.md) for detailed task breakdown.

---

## Target Structure

```yaml
# ComponentType / Trait
spec:
  parameters:
    ocSchema:              # Simple schema with inline $types
      $types:
        ResourceQuantity:
          cpu: "string | default=100m"
          memory: "string | default=256Mi"
      replicas: "integer | default=1"
      resources: "ResourceQuantity | default={}"
    # --- OR (mutually exclusive) ---
    openAPIV3Schema:       # JSON Schema document
      type: object
      $defs: { ... }
      properties: { ... }

  environmentConfigs:
    ocSchema: { ... }
    # --- OR ---
    openAPIV3Schema: { ... }

# Workflow (parameters only, no environmentConfigs)
spec:
  parameters:
    ocSchema: { ... }
    # --- OR ---
    openAPIV3Schema: { ... }
```

### Key changes from previous design
- `spec.schema` wrapper removed — `parameters` and `environmentConfigs` are directly under `spec`
- Separate `types` field removed — replaced by `$types` embedded within each `ocSchema` section
- `envOverrides` renamed to `environmentConfigs` (plural)
- Schema format is chosen per-section (not per-schema wrapper)

---

## Go Type Design

```go
// SchemaSection holds one schema in either ocSchema or openAPIV3Schema format.
// The two formats are mutually exclusive.
type SchemaSection struct {
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    // +kubebuilder:validation:Type=object
    OCSchema *runtime.RawExtension `json:"ocSchema,omitempty"`

    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    // +kubebuilder:validation:Type=object
    OpenAPIV3Schema *runtime.RawExtension `json:"openAPIV3Schema,omitempty"`
}

type ComponentTypeSpec struct {
    // ...existing fields...
    Parameters         *SchemaSection `json:"parameters,omitempty"`
    EnvironmentConfigs *SchemaSection `json:"environmentConfigs,omitempty"`
    // ...resources, validations, etc...
}
```

### `$types` handling

`$types` is embedded within the `ocSchema` raw blob. The schema extractor parses it out at processing time:

```
ocSchema RawExtension
  → yaml.Unmarshal → map[string]any
  → extract "$types" key → types map
  → remaining keys → field definitions
  → extractor.ExtractSchema(fields, types) → apiext.JSONSchemaProps
```

### Source Interface

```go
type Source interface {
    GetParameters() *runtime.RawExtension
    GetEnvironmentConfigs() *runtime.RawExtension
    IsOpenAPIV3() bool
}
```

`GetTypes()` is removed — types are extracted from the ocSchema blob internally.

---

## Processing Pipeline

Both formats converge to the same intermediate representation:

```
OCSchema:
  RawExtension → map[string]any
    → extract $types from map
    → extractor.ExtractSchema(fields, types) → apiext.JSONSchemaProps
    → apiextschema.NewStructural() → Structural
    → [Prune → Defaults → Validate → CEL]

OpenAPIV3Schema (structural — webhooks/pipeline/validation):
  RawExtension → map[string]any
    → ResolveRefs() → resolved map (no $ref/$defs)
    → stripVendorExtensions() → map (no x-* keys)
    → json round-trip → extv1.JSONSchemaProps → apiext.JSONSchemaProps
    → apiextschema.NewStructural() → Structural
    → [Prune → Defaults → Validate → CEL]

OpenAPIV3Schema (API response — preserves x-* vendor extensions):
  RawExtension → map[string]any
    → ResolveRefs() → resolved map (no $ref/$defs, x-* preserved)
    → return map[string]any directly (no extv1.JSONSchemaProps conversion)
```

Everything downstream of `apiext.JSONSchemaProps` is format-agnostic. API responses use the raw map path to preserve vendor extensions for frontend/portal consumption.

---

## Phases

### Phase 1: Restructure to top-level `parameters`/`environmentConfigs` with `ocSchema`

**Status:** Complete.

Restructure API types for ComponentTypes, Traits, and Workflows. Move from `spec.schema.ocSchema.{types, parameters, envOverrides}` to `spec.parameters.ocSchema` / `spec.environmentConfigs.ocSchema` with `$types` inline.

#### Naming convention

The field was renamed through three stages:
1. **Original:** `envOverrides` — used everywhere (CRD fields, Go types, template context, CEL variables)
2. **Intermediate:** `environmentConfig` (singular) — brief transitional name
3. **Final:** `environmentConfigs` (plural) — current target name

All three names must be updated to `environmentConfigs`. This includes:
- CRD JSON tags: `environmentConfig` → `environmentConfigs`, `envOverrides` → `environmentConfigs`
- Go struct fields: `EnvironmentConfig` → `EnvironmentConfigs`, `EnvOverrides` → `EnvironmentConfigs`
- Go interface: `GetEnvOverrides()` → `GetEnvironmentConfigs()`
- Go variables: `envOverrides` → `envConfigs`
- CRD field: `ComponentTypeEnvOverrides` → `ComponentTypeEnvironmentConfigs` (in ReleaseBinding)
- Template context JSON: `json:"envOverrides"` → `json:"environmentConfigs"` (affects CEL: `${environmentConfigs.*}`)
- YAML keys in samples/testdata: `environmentConfig:` → `environmentConfigs:`

#### What's done

**API type changes (Task 1.1):**
- `SchemaSection` shared type added to `componenttype_types.go` with `GetRaw()` and `IsOpenAPIV3()` helper methods
- `ComponentTypeSpec`: `Parameters *SchemaSection` + `EnvironmentConfigs *SchemaSection`
- `ClusterComponentTypeSpec`: same
- `TraitSpec`: `Parameters *SchemaSection` + `EnvironmentConfigs *SchemaSection`
- `ClusterTraitSpec`: same
- `WorkflowSpec` / `ClusterWorkflowSpec`: `Parameters *SchemaSection`
- Old types removed: `ComponentTypeOCSchema`, `ComponentTypeSchema`, `TraitOCSchema`, `TraitSchema`, `WorkflowOCSchema`, `WorkflowSchema`
- `ComponentTypeTrait.EnvOverrides` → `ComponentTypeTrait.EnvironmentConfigs` (json tag `environmentConfigs`)
- `ClusterComponentTypeTrait.EnvOverrides` → `ClusterComponentTypeTrait.EnvironmentConfigs`
- `ReleaseBindingSpec.ComponentTypeEnvOverrides` → `ReleaseBindingSpec.ComponentTypeEnvironmentConfigs`

**Source interface update (Task 1.2):**
- `Source` interface: `GetEnvOverrides()` → `GetEnvironmentConfigs()`
- `SimpleSource`: `EnvOverrides` → `EnvironmentConfigs` field
- `Definition` struct: `Types` field removed
- `extractor.ExtractSchema()` updated to extract `$types` from fields map

**Consuming code updates (Task 1.3) — core pipeline updated:**
- `schemautil.ExtractStructuralSchemas()` — uses `GetEnvironmentConfigs()`, field path `"environmentConfigs"`
- Pipeline context `SchemaInput` — `EnvOverridesSchema` → `EnvironmentConfigsSchema`
- `ComponentContext` / `TraitContext` — `EnvOverrides map[string]any` → `EnvironmentConfigs map[string]any` (json `environmentConfigs`)
- `EmbeddedTraitContextInput` — `ResolvedEnvOverrides` → `ResolvedEnvironmentConfigs`
- `component.go`, `trait.go`, `embedded_trait.go` — fully updated to use `EnvironmentConfigs`/`envConfigs` variables
- `BuildStructuralSchemas()` — uses `EnvironmentConfigsSchema`
- `pipeline/workflow/pipeline.go` — uses `wf.Spec.Parameters.GetRaw()`

**CRD manifests + deepcopy (Task 1.4):**
- `make generate manifests` completed
- `zz_generated.deepcopy.go` reflects new `SchemaSection` type (needs re-run after rename)

**OpenAPI spec (Task 1.7):**
- `openapi/openchoreo-api.yaml` updated with `environmentConfigs` field names

**Sample YAML files (Task 1.4/1.6):**
- All sample YAML files updated to `environmentConfigs:` (config/samples, samples/getting-started, samples/component-types, etc.)
- `samples/workflows/generic/scm-create-repo/` — updated
- `samples/getting-started/all.yaml` — regenerated via `make samples-gen`
- YAML testdata files updated (`component-with-traits.yaml`, snapshot files, scaffold testdata)

**Test fixture updates (Task 1.6):**
- All Go test files updated for struct field renames and YAML key changes

**Rename propagation (completed):**
- `internal/validation/component/*.go` — CEL environment variable names, struct fields, parameters all renamed
- `internal/pipeline/component/pipeline.go` and `pipeline_test.go` — variable names, test descriptions, error messages
- `internal/pipeline/component/context/embedded_trait_test.go` — variable names, struct fields, test descriptions
- `internal/webhook/trait/webhook_test.go` — test descriptions and error expectations
- `internal/openchoreo-api/models/request.go` and `response.go` — model field names + JSON tags
- `internal/openchoreo-api/models/request_test.go` — test field references
- `internal/openchoreo-api/mcphandlers/transform_resources.go` — K8s API field references
- `internal/openchoreo-api/mcphandlers/components.go` — references generated code (`gen.ReleaseBindingSpec.ComponentTypeEnvOverrides`) which will update after regen
- `internal/openchoreo-api/handlers/components_test.go` — JSON body + Go field references
- `internal/template/custom_functions.go` — comments
- `internal/schema/definition.go` — comments
- `internal/occ/fsmode/typed/componenttype.go` and `trait.go` — field references + map keys
- `pkg/mcp/legacytools/component.go` and `pkg/mcp/tools/component.go` — struct fields, JSON tags, schema keys
- `pkg/cli/common/constants/definitions.go` — help text
- `test/e2e/suites/connections/` and `networkpolicy/` — template expressions + struct fields
- All sample YAML files — template expressions `${environmentConfigs.*}` and field names
- All pipeline testdata YAML files — template/CEL expressions and field names
- `openapi/openchoreo-api.yaml` — `componentTypeEnvironmentConfigs` field name

**Remaining references that are correct (referencing generated code):**
- `internal/openchoreo-api/mcphandlers/components.go` — `req.ComponentTypeEnvOverrides` (references `gen.ReleaseBindingSpec`)
- `pkg/mcp/tools/component.go` — `patchReq.ComponentTypeEnvOverrides` (references `gen.ReleaseBindingSpec`)
- These will auto-resolve after `openapi/openchoreo-api.yaml` regen + `make generate`

**`traitOverrides` → `traitEnvironmentConfigs` rename (completed):**

Consistent with the `envOverrides` → `environmentConfigs` rename, the `traitOverrides` field on `ReleaseBindingSpec` was renamed to `traitEnvironmentConfigs`. This includes:
- CRD JSON tag: `traitOverrides` → `traitEnvironmentConfigs`
- Go struct field: `TraitOverrides` → `TraitEnvironmentConfigs`
- MCP tool schema/struct: `trait_overrides` → `trait_environment_configs`
- OpenAPI spec: `traitOverrides` → `traitEnvironmentConfigs` property name
- API request/response models: `TraitOverrides` → `TraitEnvironmentConfigs`
- All sample YAML files, testdata, documentation, and README files
- RCA agent: openapi.yaml, Python models, Jinja2 templates
- Generated files regenerated: `zz_generated.deepcopy.go`, `models.gen.go`, CRD YAMLs, Helm CRDs

Files changed:
- `api/v1alpha1/releasebinding_types.go` — field + JSON tag
- `pkg/mcp/tools/component.go` — MCP tool schema property name, struct field, logic
- `pkg/mcp/tools/deployment_specs_test.go` — test expected param name
- `pkg/mcp/tools/component_specs_test.go` — test expected param name
- `internal/pipeline/component/context/trait.go` — comments + code references
- `internal/pipeline/component/context/types.go` — comment
- `internal/pipeline/component/context/builder_test.go` — test YAML + comment
- `internal/pipeline/component/pipeline_test.go` — test YAML
- `internal/pipeline/component/testdata/component-with-traits.yaml` — YAML key
- `internal/openchoreo-api/models/request.go` — field + JSON tag
- `internal/openchoreo-api/models/response.go` — field + JSON tag
- `internal/openchoreo-api/models/request_test.go` — test data
- `internal/openchoreo-api/mcphandlers/components.go` — logic
- `internal/openchoreo-api/legacyservices/component_service.go` — schema keys + logic
- `internal/openchoreo-api/services/component/service.go` — schema keys
- `internal/openchoreo-api/services/component/service_test.go` — test assertions
- `openapi/openchoreo-api.yaml` — property name
- `rca-agent/openapi.yaml`, `rca-agent/src/agent/tool_registry.py`, `rca-agent/src/models/remediation_result.py`, `rca-agent/src/templates/prompts/remed_agent_prompt.j2`
- All sample YAML files (`samples/component-alerts/`, `samples/component-types/`, `samples/try-out/`)
- All documentation files (`docs/`, `website/docs/`, READMEs)
- `CLAUDE.md`

**Remaining steps:**
- [ ] Run `make generate manifests` to regenerate deepcopy + CRDs + OpenAPI gen models
- [ ] Run `make samples-gen` to regenerate `all.yaml`
- [ ] Run full test suite
- [ ] 1.9 End-to-end cluster validation

### Phase 2: OpenAPI V3 Schema for ComponentTypes + Traits

**Status:** Implementation complete. Test fixtures, sample files, and vendor extension preservation fixed (all 6 schema endpoints return `map[string]any` with `x-*` extensions preserved). E2E cluster validation remaining.

Add `openAPIV3Schema` support: `$ref`/`$defs` resolver, JSON Schema → K8s type converter, pipeline branching, API service updates.

#### What's done

**2.1 — `$ref`/`$defs` Resolver (`internal/schema/ref_resolver.go`):**
- `ResolveRefs(schema map[string]any)` — inlines all `$ref` references
- Supports both `$defs` (JSON Schema 2020-12) and `definitions` (Draft 4/7)
- Cycle detection with descriptive error paths (e.g., `"circular $ref: A → B → A"`)
- Remote/URL ref rejection with clear error messages
- Sibling key merging (JSON Schema 2020-12: `$ref` with siblings)
- Depth limit of 64 levels to prevent runaway resolution
- Deep-copies input to prevent mutation
- 12 unit tests in `ref_resolver_test.go`

**2.2 — OpenAPIV3 → K8s Type Converter (`internal/schema/openapiv3.go`):**
- `OpenAPIV3ToStructural()` — resolves refs → strips vendor extensions → K8s structural schema
- `OpenAPIV3ToJSONSchema()` — resolves refs → v1 JSONSchemaProps (preserves vendor extensions)
- `OpenAPIV3ToStructuralAndJSONSchema()` — returns both formats in one pass
- `stripVendorExtensions()` — recursively removes `x-*` keys for structural path
- 14 unit tests + 4 vendor extension tests in `openapiv3_test.go` (18 total)

**2.3 — Branching in `definition.go`:**
- `ResolveSectionToStructural()` — branches on `IsOpenAPIV3()` → `OpenAPIV3ToStructural()`
- `ResolveSectionToBundle()` — branches on `IsOpenAPIV3()` → `OpenAPIV3ToStructuralAndJSONSchema()`
- All downstream consumers (webhooks, pipeline, scaffold) automatically get OpenAPIV3 support
- No changes needed in `schemautil.ExtractStructuralSchemas`, `BuildStructuralSchemas`, or any webhook

**2.4 — `SectionToJSONSchema()` Helper + API Service Updates:**
- New `SectionToJSONSchema(section *SchemaSection)` in `definition.go` — handles both formats
- Updated 6 service files: `componenttype`, `trait`, `clustercomponenttype`, `clustertrait`, `workflow`, `clusterworkflow`
- Updated 5 legacy service files: `componenttype_service`, `trait_service`, `clustercomponenttype_service`, `clustertrait_service`, `workflow_service`
- Updated 2 component service files (`services/component/service.go`, `legacyservices/component_service.go`) — environmentConfigs and trait schema conversion
- Removed manual unmarshal + `ToJSONSchema(def)` pattern → single `SectionToJSONSchema(section)` call
- Removed unused `yaml` imports from all updated files

**2.5 — Scaffold Generator Update:**
- `NewGenerator()` in `generator.go` — uses `openchoreoschema.SectionToJSONSchema()` instead of `extractAndConvertSchema()`
- Removed unused `extractAndConvertSchema()` from `schema_defaults.go`
- All existing scaffold tests pass unchanged

**2.6 — Webhook Validation:**
- No code changes needed — webhooks call `ExtractStructuralSchemas()` → `ResolveSectionToStructural()` → automatically handles OpenAPIV3 via 2.3 branching
- Invalid `$ref`, circular refs, and malformed openAPIV3Schema will be caught at admission time

**2.7 — Tests (64 tests total across schema package):**
- 15 tests in `internal/schema/ref_resolver_test.go` (simple ref, nested chain, siblings, circular, missing, remote, items, allOf, oneOf, additionalProperties, depth limit, no refs, backward compat, nil, no mutation)
- 22 tests in `internal/schema/openapiv3_test.go` (primitives, refs, JSON schema, vendor extensions lost in JSONSchemaProps, vendor extensions preserved/stripped, both formats, nested object, array, enum, map, empty, strip vendor extensions, array of objects, boolean/number, pattern/format, nested defaults through refs, defaults applied, + 4 new: ResolvedSchema preserves extensions, ref siblings, nested extensions, no mutation)
- 27 tests in `internal/schema/definition_test.go`:
  - Existing: array field behavior, array items defaulting
  - OpenAPIV3 structural: basic, with refs, defaults work
  - OpenAPIV3 bundle: basic, nil section
  - SectionToJSONSchema: OpenAPIV3, OCSchema, nil, empty OpenAPIV3
  - Testdata-based: simple structural, with_refs structural, nested structural, invalid circular error, simple JSON schema, with_refs JSON schema preserves fields
  - Vendor extensions: preserves in JSON schema, stripped from structural
  - Validation: ValidateWithJSONSchema OpenAPIV3, ValidateAgainstSchema OpenAPIV3
  - End-to-end: defaults + validation chain with $ref resolution
  - SectionToRawJSONSchema: OpenAPIV3 preserves vendor extensions, $ref with vendor extension siblings, nil section, ocSchema fallback
- All tests pass

**2.8 — Test Fixtures (`internal/schema/testdata/`):**
- `simple_openapiv3.yaml` — basic types (string, integer, boolean), constraints (minimum, maximum, minLength, enum), defaults, required fields, descriptions
- `with_refs_openapiv3.yaml` — `$defs`/`$ref` for ResourceQuantity/ResourceRequirements/Port, enum constraints, defaults through refs
- `nested_openapiv3.yaml` — deeply nested objects (autoscaling.metrics.cpu.targetUtilization), `$ref` for AutoscalingConfig, database with credentials and pooling, array with items/minItems/maxItems, required at multiple levels
- `invalid_circular_ref.yaml` — circular `$ref` (NodeA → NodeB → NodeA) for error testing

**2.11 — API Vendor Extension Preservation (Fixed):**
- **Problem found:** `SectionToJSONSchema()` returned `*extv1.JSONSchemaProps`, which silently drops arbitrary `x-*` vendor extensions during `json.Unmarshal` (only Kubernetes-specific `x-kubernetes-*` fields are supported by the struct). Extensions like `x-openchoreo-backstage-portal` and `x-openchoreo-pull-portal` were lost in the `/schema` API responses.
- **Fix:** Added two new functions:
  - `OpenAPIV3ToResolvedSchema(rawSchema map[string]any) (map[string]any, error)` in `openapiv3.go` — resolves `$ref`s and returns the raw map directly, preserving all vendor extensions
  - `SectionToRawJSONSchema(section *SchemaSection) (map[string]any, error)` in `definition.go` — routes OpenAPIV3 through the raw path; falls back to extractor + JSON round-trip for ocSchema
- **Interface change:** All 6 schema service interfaces changed return type from `*extv1.JSONSchemaProps` to `map[string]any`:
  - `componenttype.Service.GetComponentTypeSchema()`
  - `trait.Service.GetTraitSchema()`
  - `clustercomponenttype.Service.GetClusterComponentTypeSchema()`
  - `clustertrait.Service.GetClusterTraitSchema()`
  - `workflow.Service.GetWorkflowSchema()`
  - `clusterworkflow.Service.GetClusterWorkflowSchema()`
- **Handler simplification:** All 6 API handlers no longer need `json.Marshal` → `json.Unmarshal` round-trip since the service returns `map[string]any` directly (which is what `gen.SchemaResponse` is)
- **CRUD operations:** CREATE/GET/LIST/UPDATE already preserve `x-*` extensions via `runtime.RawExtension` raw bytes in the `convert()` JSON round-trip — no fix needed
- **Structural schema path:** Unchanged — still strips `x-*` keys correctly via `stripVendorExtensions()` (K8s structural rejects vendor extensions)
- **12 new tests added:**
  - `openapiv3_test.go`: `TestOpenAPIV3ToResolvedSchema_PreservesVendorExtensions`, `TestOpenAPIV3ToResolvedSchema_VendorExtensionsWithRefSiblings`, `TestOpenAPIV3ToResolvedSchema_NestedVendorExtensions`, `TestOpenAPIV3ToResolvedSchema_DoesNotMutateInput`
  - `definition_test.go`: `TestSectionToRawJSONSchema_OpenAPIV3_PreservesVendorExtensions`, `TestSectionToRawJSONSchema_OpenAPIV3_RefWithVendorExtension`, `TestSectionToRawJSONSchema_NilSection`, `TestSectionToRawJSONSchema_OcSchema`

**2.12 — OpenAPIV3Schema Sample Files:**
- `samples/getting-started/component-types/service-openapiv3.yaml` — includes `x-openchoreo-*` vendor extensions, `parameters` + `environmentConfigs` both using openAPIV3Schema, `$defs`/`$ref` for ResourceQuantity/ResourceRequirements
- `samples/getting-started/component-types/webapp-openapiv3.yaml` — same environmentConfigs pattern
- `samples/getting-started/component-types/worker-openapiv3.yaml` — same environmentConfigs pattern (no parameters)
- `samples/getting-started/component-types/scheduled-task-openapiv3.yaml` — both parameters (CronJob-specific) and environmentConfigs with `$defs`/`$ref`
- `samples/getting-started/component-traits/alert-rule-trait-openapiv3.yaml` — complex nested parameters + environmentConfigs with descriptions, enums, defaults, required fields

#### Files changed

New files:
- `internal/schema/ref_resolver.go`
- `internal/schema/ref_resolver_test.go`
- `internal/schema/openapiv3.go`
- `internal/schema/openapiv3_test.go`
- `internal/schema/testdata/simple_openapiv3.yaml`
- `internal/schema/testdata/with_refs_openapiv3.yaml`
- `internal/schema/testdata/nested_openapiv3.yaml`
- `internal/schema/testdata/invalid_circular_ref.yaml`
- `samples/getting-started/component-types/service-openapiv3.yaml`
- `samples/getting-started/component-types/webapp-openapiv3.yaml`
- `samples/getting-started/component-types/worker-openapiv3.yaml`
- `samples/getting-started/component-types/scheduled-task-openapiv3.yaml`
- `samples/getting-started/component-traits/alert-rule-trait-openapiv3.yaml`

Modified files:
- `internal/schema/definition.go` — added `IsOpenAPIV3()` branching + `SectionToJSONSchema()` + `SectionToRawJSONSchema()` + `jsonSchemaToMap()`
- `internal/schema/definition_test.go` — added 22 OpenAPIV3 tests (integration, testdata, validation, end-to-end, vendor extension preservation)
- `internal/schema/openapiv3.go` — added `OpenAPIV3ToResolvedSchema()` for raw map path preserving vendor extensions
- `internal/schema/openapiv3_test.go` — added 4 vendor extension tests for `OpenAPIV3ToResolvedSchema`
- `internal/scaffold/component/generator.go` — uses `SectionToJSONSchema()`
- `internal/scaffold/component/schema_defaults.go` — removed unused `extractAndConvertSchema()`
- `internal/openchoreo-api/services/componenttype/interface.go` — `GetComponentTypeSchema` returns `map[string]any`
- `internal/openchoreo-api/services/componenttype/service.go` — uses `SectionToRawJSONSchema()`
- `internal/openchoreo-api/services/componenttype/service_authz.go` — updated signature
- `internal/openchoreo-api/services/trait/interface.go` — `GetTraitSchema` returns `map[string]any`
- `internal/openchoreo-api/services/trait/service.go` — uses `SectionToRawJSONSchema()`
- `internal/openchoreo-api/services/trait/service_authz.go` — updated signature
- `internal/openchoreo-api/services/clustercomponenttype/interface.go` — `GetClusterComponentTypeSchema` returns `map[string]any`
- `internal/openchoreo-api/services/clustercomponenttype/service.go` — uses `SectionToRawJSONSchema()`
- `internal/openchoreo-api/services/clustercomponenttype/service_authz.go` — updated signature
- `internal/openchoreo-api/services/clustertrait/interface.go` — `GetClusterTraitSchema` returns `map[string]any`
- `internal/openchoreo-api/services/clustertrait/service.go` — uses `SectionToRawJSONSchema()`
- `internal/openchoreo-api/services/clustertrait/service_authz.go` — updated signature
- `internal/openchoreo-api/services/workflow/interface.go` — `GetWorkflowSchema` returns `map[string]any`
- `internal/openchoreo-api/services/workflow/service.go` — uses `SectionToRawJSONSchema()`
- `internal/openchoreo-api/services/workflow/service_authz.go` — updated signature
- `internal/openchoreo-api/services/clusterworkflow/interface.go` — `GetClusterWorkflowSchema` returns `map[string]any`
- `internal/openchoreo-api/services/clusterworkflow/service.go` — uses `SectionToRawJSONSchema()`
- `internal/openchoreo-api/services/clusterworkflow/service_authz.go` — updated signature
- `internal/openchoreo-api/api/handlers/componenttypes.go` — simplified (no JSON round-trip)
- `internal/openchoreo-api/api/handlers/traits.go` — simplified
- `internal/openchoreo-api/api/handlers/clustercomponenttypes.go` — simplified
- `internal/openchoreo-api/api/handlers/clustertraits.go` — simplified
- `internal/openchoreo-api/api/handlers/workflows.go` — simplified
- `internal/openchoreo-api/api/handlers/clusterworkflows.go` — simplified
- `internal/openchoreo-api/services/component/service.go`
- `internal/openchoreo-api/legacyservices/componenttype_service.go`
- `internal/openchoreo-api/legacyservices/trait_service.go`
- `internal/openchoreo-api/legacyservices/clustercomponenttype_service.go`
- `internal/openchoreo-api/legacyservices/clustertrait_service.go`
- `internal/openchoreo-api/legacyservices/workflow_service.go`
- `internal/openchoreo-api/legacyservices/component_service.go`

#### Remaining steps
- [ ] 2.9 End-to-end cluster validation (basic)
- [ ] 2.10 Critical review: error messages, CLI display, performance, feature parity
- [ ] 2.13 Test API server schema endpoints via curl for both formats
- [ ] 2.14 Full cluster deployment: build → k3d load → rollout restart → apply samples → verify logs + rendering

### Phase 3: OpenAPI V3 Schema for Workflows

**Status:** Implementation complete. Critical review found and fixed 2 additional bugs (componentrelease webhook and legacy component service using ocSchema-only code paths). E2E cluster validation remaining.

Workflow API services (2.4) and scaffold (2.5) were already updated in Phase 2. Pipeline branching now flows through `ResolveSectionToStructural` correctly.

#### What's done

**3.1 — Pipeline fix (`internal/pipeline/workflow/pipeline.go`):**
- `buildStructuralSchema()` was bypassing `ResolveSectionToStructural()` and calling `schema.ToStructural()` directly (ocSchema-only path)
- Fixed to use `schema.ResolveSectionToStructural(wf.Spec.Parameters)` which transparently handles both ocSchema and openAPIV3Schema
- Removed manual JSON unmarshalling and `Definition{}` construction — single function call now

**3.2 — Pipeline tests (`internal/pipeline/workflow/pipeline_test.go`):**
- `TestPipeline_Render_OpenAPIV3Schema_Defaults` — 5 subtests:
  - `openAPIV3Schema applies defaults for missing fields` — basic string/integer defaults
  - `openAPIV3Schema with $defs and $ref applies defaults` — nested object with `$ref` resolution + defaults through refs
  - `ocSchema applies defaults for missing fields` — validates ocSchema path still works through `ResolveSectionToStructural`
  - `openAPIV3Schema with no parameters applies all defaults` — empty WorkflowRun params, all defaults from schema
  - `nil schema section works without defaults` — nil Parameters, no defaults applied

**3.3 — Workflow sample files:**
- `samples/getting-started/workflows/docker-openapiv3.yaml` — Docker workflow with openAPIV3Schema parameters (repository + docker config with defaults, descriptions, required fields)
- `samples/getting-started/workflows/react-openapiv3.yaml` — React workflow with openAPIV3Schema (includes nodeVersion enum constraint)
- `samples/getting-started/workflows/ballerina-buildpack-openapiv3.yaml` — Ballerina buildpack workflow with openAPIV3Schema
- `samples/getting-started/workflows/google-cloud-buildpacks-openapiv3.yaml` — Google Cloud Buildpacks workflow with openAPIV3Schema

**3.4 — ComponentRelease webhook fix (`internal/webhook/componentrelease/webhook.go`):**
- **Bug found during critical review:** `validateComponentParameters()` was using `schema.Definition{}` + `schema.ToJSONSchema()` directly — ocSchema-only path. Would fail or produce wrong validation for ComponentReleases referencing openAPIV3Schema ComponentTypes.
- Fixed to use `schema.SectionToJSONSchema(release.Spec.ComponentType.Parameters)` which handles both formats
- Same fix applied to `validateTraitInstanceParameters()` — was using `schema.Definition{}` + `schema.ToJSONSchema()` for trait schema validation
- Removed manual `yaml.Unmarshal` + `paramsSchema` intermediate variable in both functions

**3.5 — Legacy component service fix (`internal/openchoreo-api/legacyservices/component_service.go`):**
- **Bug found during critical review:** `validateWorkflowParameters()` was using `json.Unmarshal` + `schema.Definition{}` + `schema.ToStructural()` directly — ocSchema-only path. Would fail to resolve `$ref`/`$defs` for openAPIV3Schema workflows.
- Fixed to use `schema.ResolveSectionToStructural(workflowSpec.Parameters)` which handles both formats
- Removed manual `json.Unmarshal` of schema raw bytes and `Definition{}` construction

#### Critical review verification

After completing the implementation, a thorough critical review confirmed:
- **No remaining `schema.Definition{}` usage in production code** — only in test helpers with hardcoded ocSchema data
- **All 6 API schema endpoints** (workflow, clusterworkflow, componenttype, trait, clustercomponenttype, clustertrait) correctly use `SectionToRawJSONSchema()` returning `map[string]any`
- **All webhook validators** (componenttype, clustercomponenttype, trait, clustertrait) use `schemautil.ExtractStructuralSchemas()` → `ResolveSectionToStructural()` — correct
- **Scaffold generator** uses `SectionToJSONSchema()` — correct
- **MCP tools** and **CLI** delegate to service layer — correct
- **Full project compiles** (`go build ./...`) with no errors

#### Files changed

New files:
- `samples/getting-started/workflows/docker-openapiv3.yaml`
- `samples/getting-started/workflows/react-openapiv3.yaml`
- `samples/getting-started/workflows/ballerina-buildpack-openapiv3.yaml`
- `samples/getting-started/workflows/google-cloud-buildpacks-openapiv3.yaml`

Modified files:
- `internal/pipeline/workflow/pipeline.go` — `buildStructuralSchema()` uses `ResolveSectionToStructural()`
- `internal/pipeline/workflow/pipeline_test.go` — added `TestPipeline_Render_OpenAPIV3Schema_Defaults` (5 subtests)
- `internal/webhook/componentrelease/webhook.go` — `validateComponentParameters()` and `validateTraitInstanceParameters()` use `SectionToJSONSchema()`
- `internal/openchoreo-api/legacyservices/component_service.go` — `validateWorkflowParameters()` uses `ResolveSectionToStructural()`

#### Remaining steps
- [ ] Run `make samples-gen` to regenerate `all.yaml`
- [ ] Run full test suite (`make test`)
- [ ] E2E cluster validation
