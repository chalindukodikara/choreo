# Task: Research and Implement OpenAPI V3 Schema Support for OpenChoreo

## Goal

Research how OpenAPI V3 Schema (JSON Schema) can replace or complement OpenChoreo's current "Simple Schema" for defining configuration validation in CRDs. Produce reference documentation, converted sample files, a limitations analysis, and a comprehensive scenario reference.

---

## Context: What OpenChoreo Does

OpenChoreo is a Kubernetes-based platform. It uses Custom Resource Definitions (CRDs) where **platform engineers** define configuration schemas, and **developers** supply values against those schemas.

Three CRD types use schemas:

| CRD | Schema Purpose |
|-----|----------------|
| **ComponentType** | Defines `parameters` (required at component creation) and `environmentConfigs` (optional, per-environment config). Also supports reusable `$types` inline in ocSchema. |
| **Trait** | Same as ComponentType: `parameters` + `environmentConfigs`. |
| **Workflow** | Defines `parameters` that must be supplied when triggering a workflow run. |

---

## Context: Current Simple Schema (from Kro)

OpenChoreo currently uses [Kro's Simple Schema](https://kro.run/api/specifications/simple-schema) (v0.8.5). It is concise but unstable, not widely adopted, and lacks extensibility.

### Syntax pattern
```
fieldName: "type | constraint1=value1 constraint2=value2"
```

### Supported features (full reference: `docs/templating/schema.md`)

- **Primitive types**: `string`, `integer`, `number`, `boolean`
- **Arrays**: `[]string`, `[]integer`, `[]CustomType`
- **Maps**: `map<string>`, `map<integer>`
- **Nested objects**: Defined inline as nested YAML keys
- **Custom types**: Defined under `schema.types`, referenced by name (e.g., `ResourceRequirements`)
- **Defaults**: All fields required unless `default=value` is specified. Objects use `$default: {}` or `default={}`.
- **String constraints**: `minLength`, `maxLength`, `pattern`, `enum`
- **Numeric constraints**: `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`, `enum`
- **Array constraints**: `minItems`, `maxItems`
- **Map constraints**: `minProperties`, `maxProperties`
- **Documentation markers**: `title`, `description`, `example`
- **Custom annotations**: `oc:` prefix (e.g., `oc:ui:hidden=true`, `oc:build:inject=git.sha`)
- **Schema evolution**: Additional properties allowed by default (no `additionalProperties: false`)
- The simple schema compiles down to JSON Schema internally

### Example: Simple Schema in a ComponentType (original flat structure)
```yaml
spec:
  schema:
    types:
      ResourceRequirements:
        requests: "ResourceQuantity | default={}"
        limits: "ResourceQuantity | default={}"
      ResourceQuantity:
        cpu: "string | default=100m"
        memory: "string | default=256Mi"

    parameters:
      autoscaling:
        enabled: "boolean | default=false"
        minReplicas: "integer | default=2 | minimum=1"
        maxReplicas: "integer | default=10 | minimum=1"

    envOverrides:
      replicas: "integer | default=1"
      resources: "ResourceRequirements | default={}"
      imagePullPolicy: "string | default=IfNotPresent"
```

### Example: Simple Schema in a Workflow
```yaml
spec:
  schema:
    parameters:
      repository:
        url: string | description="Git repository URL"
        revision:
          branch: string | default=main description="Git branch to checkout"
        appPath: string | default=. description="Path to the application directory within the repository"
      docker:
        context: string | default=. description="Docker build context path relative to the repository root"
        filePath: string | default=./Dockerfile description="Path to the Dockerfile relative to the repository root"
```

> **Note:** In the new design, the separate top-level `types` field is replaced by `$types` embedded within each `ocSchema` section. See [Target Structure](#target-structure).

---

## Design Decision

See [Discussion.md](./Discussion.md) for full rationale.

### Why change from original structure
The original structure wrapped `parameters` and `environmentConfig` inside the format key (`openAPIV3Schema` / `ocSchema`). This felt unnatural — the format key acts as a container for two independent JSON Schema documents rather than a schema property.

### Target Structure

`parameters` and `environmentConfig` are top-level fields under `spec`. Each independently declares its schema format (`openAPIV3Schema` or `ocSchema`), which are mutually exclusive within each section.

**ComponentTypes and Traits:**
```yaml
spec:
  parameters:
    openAPIV3Schema:           # JSON Schema document
      type: object
      $defs: { ... }
      properties: { ... }
    # --- OR (mutually exclusive) ---
    ocSchema:                  # Simple schema with inline $types
      $types:
        ResourceQuantity:
          cpu: "string | default=100m"
          memory: "string | default=256Mi"
      autoscaling:
        enabled: "boolean | default=false"
      resources: "ResourceQuantity | default={}"

  environmentConfig:
    openAPIV3Schema:           # JSON Schema document
      type: object
      $defs: { ... }
      properties: { ... }
    # --- OR (mutually exclusive) ---
    ocSchema:                  # Simple schema with inline $types
      $types:
        ResourceQuantity:
          cpu: "string | default=100m"
          memory: "string | default=256Mi"
      replicas: "integer | default=1"
      resources: "ResourceQuantity | default={}"
```

> **`$types` in ocSchema:** Replaces the previous top-level `types` field. `$types` is embedded within each `ocSchema` section, allowing reusable type definitions scoped to that section. Types must be duplicated if shared across `parameters` and `environmentConfig`.

**Workflows (parameters only):**
```yaml
spec:
  parameters:
    openAPIV3Schema:
      type: object
      properties: { ... }
    # --- OR ---
    ocSchema:
      repository:
        url: string
```

### Key advantages
1. **Industry standard** — OpenAPI/JSON Schema is widely adopted (Kubernetes CRDs, Crossplane, etc.)
2. **Natural grouping** — Top-level organization matches how PEs think: "what can developers configure?" vs. "what varies per environment?"
3. **Single JSON Schema per `openAPIV3Schema`** — Each key is one self-contained JSON Schema document
4. **Mature tooling** — Validation libraries, IDE support, documentation generators
5. **Vendor extensions** — `x-` prefixed keys allow custom metadata (e.g., Backstage UI hints)
6. **Stable** — Not tied to Kro's evolving schema language

---

## Findings: [FINDINGS.md](./FINDINGS.md)

## Testing Guide: [TESTING.md](./TESTING.md)

---

## Architecture Analysis

### How the current schema pipeline works

```
CRD YAML (spec.schema)
  ↓
Webhook admission (schemautil.ExtractStructuralSchemas)
  → Unmarshals Types, Parameters, EnvOverrides from RawExtension
  → Calls schema.ToStructural(Definition{Types, Schemas}) per section
    → extractor.ExtractSchema() converts shorthand syntax → apiext.JSONSchemaProps
    → apiextschema.NewStructural() converts to Kubernetes structural schema
  → Returns parametersSchema, envOverridesSchema for CEL type-checking
  ↓
Pipeline rendering (context.BuildStructuralSchemas)
  → Same extraction: Definition{Types, Schemas} → schema.ToStructuralAndJSONSchema()
  → Returns SchemaBundle{Structural, JSONSchema} per section
  ↓
Context building (processComponentParameters / processTraitParameters)
  → Prune user values against Structural schema (removes unknown fields)
  → Apply defaults from Structural schema
  → Validate against JSONSchema (required fields, types, constraints)
  → Feed resolved parameters/envOverrides into CEL context
  ↓
Template rendering (format-agnostic — works on resolved maps)
```

### Key insight: Both formats converge to the same output

```
OCSchema (shorthand)       → extractor.ExtractSchema()           → apiext.JSONSchemaProps
OpenAPIV3Schema (JSON Sch) → $ref resolution + JSON unmarshal    → apiext.JSONSchemaProps
                                                                       ↓
                                                         apiextschema.Structural + extv1.JSONSchemaProps
                                                                       ↓
                                                         Prune → Defaults → Validate → CEL
```

Everything downstream of `apiext.JSONSchemaProps` is schema-format agnostic.

### Key Design Decision: No New JSON Schema Library Needed

Both schema formats converge to the same output type (`apiext.JSONSchemaProps`). The only new capability needed is `$defs`/`$ref` resolution, which is a straightforward tree walk on `map[string]any` (~100-150 lines).

| Concern | Approach | Why no library needed |
|---------|----------|----------------------|
| `$ref`/`$defs` resolution | Custom resolver on `map[string]any` | Simple recursive tree walk; only supports local `$ref` (no remote/URL refs) |
| Schema validation (values against schema) | Existing `validation.NewSchemaValidator()` | Works on resolved schemas (no `$ref` remaining) |
| Schema compilation check (is schema valid?) | Try building `Structural` from it | If `apiextschema.NewStructural()` fails, schema is invalid |
| `x-openchoreo-*` extensions | Preserved in raw storage, stripped during conversion | Extensions are for UI/tooling, not validation pipeline |

### Processing Flow Comparison

```
OCSchema (new):
  RawExtension (shorthand YAML with inline $types)
    → yaml.Unmarshal → map[string]any
    → extract $types from map, separate from field definitions
    → extractor.ExtractSchema(fields, types) → apiext.JSONSchemaProps
    → apiextschema.NewStructural() → Structural
    → [same downstream: Prune → Defaults → Validate → CEL]

OpenAPIV3Schema (new):
  RawExtension (JSON Schema YAML)
    → yaml.Unmarshal → map[string]any
    → ResolveRefs() → resolved map[string]any (no $ref/$defs remaining)
    → json.Marshal → json.Unmarshal → extv1.JSONSchemaProps
    → Convert_v1_To_apiextensions → apiext.JSONSchemaProps
    → apiextschema.NewStructural() → Structural
    → [same downstream: Prune → Defaults → Validate → CEL]
```

---

## Key Files

| Area | Files |
|------|-------|
| API types | `api/v1alpha1/componenttype_types.go`, `trait_types.go`, `workflow_types.go` |
| Schema core | `internal/schema/definition.go` |
| Schema extractor (ocSchema) | `internal/schema/extractor/schema.go` |
| Validation bridge | `internal/validation/schemautil/extract.go` |
| Pipeline context | `internal/pipeline/component/context/component.go`, `trait.go`, `embedded_trait.go`, `types.go` |
| Workflow pipeline | `internal/pipeline/workflow/pipeline.go` |
| Webhooks | `internal/webhook/componenttype/`, `clustercomponenttype/`, `trait/`, `clustertrait/`, `componentrelease/` |
| Scaffold | `internal/scaffold/component/generator.go`, `schema_defaults.go` |
| API services | `internal/openchoreo-api/services/componenttype/`, `trait/`, `clustertrait/`, `clustercomponenttype/`, `component/`, `workflow/` |
| Legacy API services | `internal/openchoreo-api/legacyservices/componenttype_service.go`, `clustercomponenttype_service.go`, `trait_service.go`, `clustertrait_service.go`, `component_service.go`, `workflow_service.go` |
| CLI typed output | `internal/occ/fsmode/typed/componenttype.go`, `trait.go` |
| OpenAPI spec | `openapi/openchoreo-api.yaml` |

---

## Tasks

### Phase 1: Restructure API Types — Top-level `parameters`/`environmentConfig` with `ocSchema`

**Goal:** Move from the current structure (`spec.schema.ocSchema.{types, parameters, envOverrides}`) to the new top-level structure (`spec.parameters.ocSchema`, `spec.environmentConfig.ocSchema`). Only `ocSchema` is supported in this phase. Add getter methods so consuming code doesn't need to know which format is used.

#### 1.1 API Type Changes

**ComponentType** (`api/v1alpha1/componenttype_types.go`):
- Remove `ComponentTypeSchema`, `ComponentTypeOCSchema`, `ComponentTypeOpenAPIV3Schema` structs
- Add new shared type for schema sections:
```go
// SchemaSection holds one schema in either ocSchema or openAPIV3Schema format.
// The two formats are mutually exclusive.
type SchemaSection struct {
    // OCSchema defines the schema using OpenChoreo's simple schema format.
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    // +kubebuilder:validation:Type=object
    OCSchema *runtime.RawExtension `json:"ocSchema,omitempty"`

    // OpenAPIV3Schema defines the schema using standard OpenAPI V3 / JSON Schema format.
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    // +kubebuilder:validation:Type=object
    OpenAPIV3Schema *runtime.RawExtension `json:"openAPIV3Schema,omitempty"`
}
```
- Update `ComponentTypeSpec`:
```go
type ComponentTypeSpec struct {
    // ...existing fields (workloadType, allowedWorkflows, traits, etc.)...

    // Parameters defines what developers can configure when creating components of this type.
    // +optional
    Parameters *SchemaSection `json:"parameters,omitempty"`

    // EnvironmentConfig defines per-environment overrides developers can set via ReleaseBinding.
    // +optional
    EnvironmentConfig *SchemaSection `json:"environmentConfig,omitempty"`

    // ...resources, validations, etc...
}
```
- Remove `Schema ComponentTypeSchema` field from `ComponentTypeSpec`
- No separate `Types` field — reusable types are now embedded as `$types` inside each `ocSchema` section
- Add getter/helper methods on `ComponentTypeSpec` (or a helper interface)

**`$types` handling in ocSchema:**
- `ocSchema` is a single `RawExtension` blob containing both `$types` and field definitions
- The schema extractor must extract `$types` from the `map[string]any` before processing fields
- `$types` replaces the previous top-level `types` field
- Example: `{"$types": {"ResourceQuantity": {...}}, "replicas": "integer | default=1", "resources": "ResourceQuantity | default={}"}`

**Trait** (`api/v1alpha1/trait_types.go`):
- Same pattern: remove `TraitSchema`, `TraitOCSchema`, `TraitOpenAPIV3Schema`
- Add `Parameters`, `EnvironmentConfig` fields to `TraitSpec` (no separate `Types`)

**Workflow** (`api/v1alpha1/workflow_types.go`):
- Remove `WorkflowSchema`, `WorkflowOCSchema`
- Add `Parameters` field to `WorkflowSpec` (no `EnvironmentConfig` for workflows, no separate `Types`)

#### 1.2 Source Interface Update

**File:** `internal/schema/definition.go`

Update the `Source` interface to match the new structure:
```go
type Source interface {
    GetParameters() *runtime.RawExtension
    GetEnvOverrides() *runtime.RawExtension
    IsOpenAPIV3() bool
}
```

- `GetTypes()` is removed — `$types` is now embedded within each `ocSchema` blob and extracted by the schema extractor at parse time
- `GetParameters()` returns `Parameters.OCSchema` or `Parameters.OpenAPIV3Schema` depending on which is set
- `GetEnvOverrides()` returns `EnvironmentConfig.OCSchema` or `EnvironmentConfig.OpenAPIV3Schema`
- `IsOpenAPIV3()` returns `true` if `Parameters.OpenAPIV3Schema != nil` (Phase 1: always `false`)

**Schema extractor change:** The existing `extractor.ExtractSchema()` must be updated to extract `$types` from the `map[string]any` before processing field definitions. Previously it received types as a separate parameter; now it parses them from the same blob.

#### 1.3 Update Consuming Code

All code that currently accesses `Schema.GetTypes()`, `Schema.GetParameters()`, `Schema.GetEnvOverrides()` must be updated.

**Pipeline context (3 call sites):**
- `internal/pipeline/component/context/component.go` — `input.ComponentType.Spec.Schema.GetTypes()` → new path
- `internal/pipeline/component/context/trait.go` — same
- `internal/pipeline/component/context/embedded_trait.go` — same

**Pipeline types:**
- `internal/pipeline/component/context/types.go` — `SchemaInput` struct

**Validation:**
- `internal/validation/schemautil/extract.go` — `ExtractStructuralSchemas` source parameter

**Webhooks:**
- `internal/webhook/componenttype/webhook.go`
- `internal/webhook/clustercomponenttype/webhook.go`
- `internal/webhook/trait/webhook.go`
- `internal/webhook/clustertrait/webhook.go`
- `internal/webhook/componentrelease/webhook.go`

**API services (~12 call sites):**
- `internal/openchoreo-api/services/componenttype/service.go`
- `internal/openchoreo-api/services/trait/service.go`
- `internal/openchoreo-api/services/clustertrait/service.go`
- `internal/openchoreo-api/services/clustercomponenttype/service.go`
- `internal/openchoreo-api/services/component/service.go`
- `internal/openchoreo-api/legacyservices/componenttype_service.go`
- `internal/openchoreo-api/legacyservices/clustercomponenttype_service.go`
- `internal/openchoreo-api/legacyservices/trait_service.go`
- `internal/openchoreo-api/legacyservices/clustertrait_service.go`
- `internal/openchoreo-api/legacyservices/component_service.go`

**Workflow-specific:**
- `internal/pipeline/workflow/pipeline.go`
- `internal/openchoreo-api/services/workflow/service.go`
- `internal/openchoreo-api/legacyservices/workflow_service.go`

**Scaffold:**
- `internal/scaffold/component/generator.go`
- `internal/scaffold/component/generator_test.go`
- `internal/scaffold/component/schema_defaults.go`

**CLI:**
- `internal/occ/fsmode/typed/componenttype.go`
- `internal/occ/fsmode/typed/trait.go`

#### 1.4 Regenerate & Update

- Run `make generate manifests` — regenerate deepcopy + CRD manifests
- Update `openapi/openchoreo-api.yaml` with new schema structure
- Update all sample YAML files to new structure
- Update all test files and YAML testdata

---

#### 1.5 Verify Implementation Correctness

Run full compilation and test suite to catch regressions:
```bash
make generate manifests     # Ensure generated code is up to date
make build                  # Verify clean compilation
make test                   # Run all unit tests
```

**Manual review checklist:**
- Verify all `Source` interface implementors return correct values from `GetParameters()` and `GetEnvOverrides()`
- Verify `$types` extraction works in `extractor.ExtractSchema()` — step through with a sample that uses custom types
- Confirm no residual references to old `Schema` field, `GetTypes()`, or `spec.schema` path remain in Go code:
  ```bash
  grep -r 'Schema\.' --include='*.go' | grep -v '_test.go' | grep -v 'SchemaSection'
  grep -r 'GetTypes()' --include='*.go'
  grep -r 'spec\.schema' --include='*.go'
  ```
- Verify deepcopy functions are generated for `SchemaSection` and updated spec types
- Verify CRD manifests in `config/crd/` reflect the new `parameters`/`environmentConfig` structure (no `spec.schema` path)

#### 1.6 Update Samples and Tests

**Sample YAML files** — convert all samples from old `spec.schema` structure to new `spec.parameters`/`spec.environmentConfig` structure with `$types` inside `ocSchema`:

| Directory | Files to update |
|-----------|----------------|
| `samples/getting-started/` | All ComponentType, Trait, and Workflow YAML files |
| `samples/` (other subdirectories) | Any YAML referencing `spec.schema.parameters` or `spec.schema.envOverrides` |
| `testdata/` | All test fixture YAML files across webhook, pipeline, scaffold, and service test directories |
- samples/component-types/*
- samples/workflows/*
- samples/component-alerts/

**Verification:**
```bash
# Ensure no sample or testdata files use the old structure
grep -r 'spec:' -A2 --include='*.yaml' samples/ | grep 'schema:'
grep -r 'envOverrides:' --include='*.yaml' samples/ testdata/
```

**Tests:**
- Update all table-driven test cases that construct CRD objects with the old `Schema` field
- Add test cases that verify `$types` extraction from `ocSchema` blob
- Ensure existing test coverage for constraints (minLength, pattern, enum, etc.) still passes with new structure

#### 1.7 Update OpenAPI Spec and Verify API Endpoints

**File:** `openapi/openchoreo-api.yaml`

- Replace `spec.schema` references with `spec.parameters` and `spec.environmentConfig`
- Add `SchemaSection` schema definition with `ocSchema` and `openAPIV3Schema` as mutually exclusive properties
- Update request/response schemas for ComponentType, Trait, and Workflow endpoints
- Update any `envOverrides` references to `environmentConfig`

**Verification — start the API server and test with curl:**
```bash
# Start openchoreo-api locally or port-forward to cluster
# Test ComponentType CRUD
curl -s localhost:8080/api/v1/componenttypes | jq '.items[0].spec.parameters'
curl -s localhost:8080/api/v1/componenttypes | jq '.items[0].spec.environmentConfig'

# Test Trait CRUD
curl -s localhost:8080/api/v1/traits | jq '.items[0].spec.parameters'

# Test Workflow
curl -s localhost:8080/api/v1/workflows | jq '.items[0].spec.parameters'

# Verify old paths return null/missing (not populated)
curl -s localhost:8080/api/v1/componenttypes | jq '.items[0].spec.schema'  # should be null
```

- Validate the OpenAPI spec itself: `make validate-openapi` (or use an OpenAPI linter)

#### 1.8 Update CLI Typed Output

**Files:**
- `internal/occ/fsmode/typed/componenttype.go` — update to read from `spec.Parameters` and `spec.EnvironmentConfig` instead of `spec.Schema`
- `internal/occ/fsmode/typed/trait.go` — same pattern

**Changes:**
- Update struct field access paths for display/formatting logic
- Update column headers if `envOverrides` was displayed (rename to `environmentConfig`)
- Ensure `occ get componenttypes` and `occ get traits` display schema info correctly

**Verification:**
```bash
# Run CLI commands and verify output format
occ get componenttypes -o yaml   # Verify new structure in YAML output
occ get componenttypes           # Verify table output renders correctly
occ get traits -o yaml
occ get traits
```

#### 1.9 End-to-End Cluster Validation

**Step 1 — Apply CRDs and deploy:**
```bash
make install                    # Apply updated CRDs to the k3d cluster
make k3d.build.controller       # Build controller image
make k3d.load.controller        # Load controller image into k3d
make k3d.build.openchoreo-api   # Build openchoreo-api image
make k3d.load.openchoreo-api    # Load openchoreo-api image into k3d

# Rollout restart to pick up new images
kubectl rollout restart deployment/openchoreo-controller -n openchoreo-system
kubectl rollout restart deployment/openchoreo-api -n openchoreo-system

# Wait for pods to be ready
kubectl rollout status deployment/openchoreo-controller -n openchoreo-system --timeout=120s
kubectl rollout status deployment/openchoreo-api -n openchoreo-system --timeout=120s
```

**Step 2 — Apply samples and verify rendering:**
```bash
# Apply a sample ComponentType with the new structure
kubectl apply -f samples/getting-started/componenttype.yaml

# Apply a sample Trait
kubectl apply -f samples/getting-started/trait.yaml

# Apply a sample Workflow
kubectl apply -f samples/getting-started/workflow.yaml

# Apply a Component that uses the ComponentType and verify rendering
kubectl apply -f samples/getting-started/component.yaml
```

**Step 3 — Verify pipeline rendering works end-to-end:**
- Check controller logs for schema parsing errors: `kubectl logs -l app=openchoreo-controller -n openchoreo-system --tail=100`
- Verify Component status shows successful rendering (no schema validation failures)
- Verify that parameter defaults are applied correctly
- Verify that environment config overrides work via ReleaseBinding
- Test with a ComponentType that uses custom `$types` in ocSchema to confirm type resolution works
- Check webhook admission works: apply an invalid Component (missing required field) and verify it is rejected

### Phase 2: Implement OpenAPI V3 Schema Processing for ComponentTypes and Traits

**Goal:** Make the schema processing pipeline handle `openAPIV3Schema` input (standard JSON Schema with `$defs`/`$ref`) alongside the existing `ocSchema` shorthand. Both formats must produce the same downstream types (`apiextschema.Structural` + `extv1.JSONSchemaProps`) so that defaulting, validation, rendering, and API responses all work identically regardless of input format.

**Prerequisite:** Phase 1 complete. `SchemaSection` already has `OCSchema` / `OpenAPIV3Schema` fields with `IsOpenAPIV3()` / `GetRaw()` methods and a kubebuilder XValidation rule for mutual exclusivity.
        - "openAPIV3Schema" will be the default schema format for all API services and the scaffold.
---

#### 2.1 Implement `$ref`/`$defs` Resolver

**New file:** `internal/schema/ref_resolver.go`

**Purpose:** Inline all `$ref` references so downstream code never sees `$ref` — it only works with fully-expanded JSON Schema objects. This is necessary because Kubernetes `apiextschema.NewStructural()` does not understand `$ref`.

**Function:** `ResolveRefs(schema map[string]any) (map[string]any, error)`

**Algorithm:**
1. Deep-copy input to avoid mutation
2. Extract definitions map from `$defs` (JSON Schema 2020-12) or `definitions` (Draft 4/7) — support both
3. Walk the tree depth-first, replacing `$ref` nodes:
   - Parse ref path: only local refs supported (`#/$defs/Foo` or `#/definitions/Foo`)
   - Look up definition, error if not found
   - Recursively resolve any `$ref` within the definition itself before inlining
   - Track visited refs in a stack to detect cycles → error with cycle path (e.g., `"circular $ref: A → B → A"`)
   - Enforce depth limit of 64 levels to prevent runaway resolution
4. Handle `$ref` with sibling keys (JSON Schema 2020-12 allows this):
   - Merge: resolved definition as base, sibling keys override on conflict
   - Example: `{"$ref": "#/$defs/Foo", "default": {}}` → Foo's schema + `default: {}`
5. Walk into all schema-bearing keywords: `properties`, `items`, `additionalProperties`, `allOf`, `oneOf`, `anyOf`, `not`, `if`/`then`/`else`
6. Remove `$defs`/`definitions` from final output
7. Return fully-inlined schema

**Edge cases to handle:**
- `$ref` to non-existent definition → clear error with ref path
- Circular: `A → B → A` → error listing the cycle
- `$ref` inside `items` (array schemas), `additionalProperties`, composition keywords
- Nested chains: `A → B → C` where C has no further refs
- Remote/URL `$ref` (e.g., `http://...`) → reject with error "only local $ref supported"
- Empty `$defs` map → no-op, return schema as-is

**Test file:** `internal/schema/ref_resolver_test.go`

**Test cases:**
| Case | Input | Expected |
|------|-------|----------|
| Simple ref | `{"$ref": "#/$defs/Foo"}` with `$defs.Foo = {type: string}` | `{type: string}` |
| Nested ref chain | A refs B, B refs C | Fully inlined C inside B inside A |
| Ref with siblings | `{"$ref": "#/$defs/Foo", "default": "bar"}` | Foo's schema + `default: bar` |
| Circular ref | A refs B, B refs A | Error: `"circular $ref: A → B → A"` |
| Missing ref | `{"$ref": "#/$defs/Missing"}` | Error: `"$ref #/$defs/Missing not found"` |
| Remote ref | `{"$ref": "http://example.com/schema"}` | Error: `"only local $ref supported"` |
| Ref in items | `{type: array, items: {$ref: "#/$defs/Item"}}` | items inlined |
| Ref in allOf/oneOf | composition keyword with $ref entries | All entries resolved |
| No refs present | Plain schema | Returned unchanged (minus $defs key) |
| Backward compat | `{"$ref": "#/definitions/Foo"}` with `definitions` key | Same resolution |
| Depth limit exceeded | 65+ levels of nested refs | Error |

---

#### 2.2 Implement OpenAPIV3Schema → K8s Type Converter

**New file:** `internal/schema/openapiv3.go`

**Purpose:** Convert a raw `openAPIV3Schema` blob (which may contain `$ref`, `$defs`, `x-` extensions) into the same K8s types that `extractor.ExtractSchema()` produces for ocSchema. This is the format-bridging layer — after this, all downstream code works identically.
- "openAPIV3Schema" will be the default schema format for all API services and the scaffold.

**Functions:**
```go
// OpenAPIV3ToStructural resolves $refs and converts to K8s structural schema.
// Used by webhooks (ExtractStructuralSchemas) and pipeline (BuildStructuralSchemas).
func OpenAPIV3ToStructural(rawSchema map[string]any) (*apiextschema.Structural, error)

// OpenAPIV3ToJSONSchema resolves $refs and converts to v1 JSONSchemaProps.
// Used by API services to return schema for UI/CLI consumption.
func OpenAPIV3ToJSONSchema(rawSchema map[string]any) (*extv1.JSONSchemaProps, error)

// OpenAPIV3ToStructuralAndJSONSchema returns both formats in one pass.
// Used by pipeline context (BuildStructuralSchemas) which needs both.
func OpenAPIV3ToStructuralAndJSONSchema(rawSchema map[string]any) (*apiextschema.Structural, *extv1.JSONSchemaProps, error)
```

**Conversion pipeline (shared internal logic):**
1. `ResolveRefs(rawSchema)` → fully-inlined `map[string]any`
2. `stripVendorExtensions(resolved)` — recursively remove all `x-` prefixed keys (K8s structural schema rejects them)
3. `json.Marshal(resolved)` → `json.Unmarshal` into `extv1.JSONSchemaProps` (leverages K8s JSON tags)
4. `extv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps()` → `apiext.JSONSchemaProps`
5. `apiextschema.NewStructural()` → `*apiextschema.Structural`
6. Return requested format(s)

**Helper:** `stripVendorExtensions(schema map[string]any) map[string]any` — walks tree, removes keys starting with `x-`. Must handle nested properties, items, additionalProperties, allOf/oneOf/anyOf.

**Key considerations:**
- Preserve `oc:` annotations as `x-oc-*` vendor extensions in the JSON Schema response (step 2 strips them from the structural path only, not the JSON schema path) — **OR** decide to strip `oc:` annotations entirely from openAPIV3Schema since they're an ocSchema concept. **Decision needed.**
- Ensure `type: "object"` is present on all object schemas (K8s structural requires this)
- `enum` values must match the declared type
- `default` values must be valid JSON (not YAML-only constructs)

**Test file:** `internal/schema/openapiv3_test.go`

**Test cases:**
| Case | Input | Validates |
|------|-------|-----------|
| Primitives | `{type: string, minLength: 1, default: "hello"}` | Type mapping + constraints + defaults |
| Object with properties | `{type: object, properties: {name: {type: string}}, required: [name]}` | Required field handling |
| Array with items | `{type: array, items: {type: integer}, minItems: 1}` | Array constraint mapping |
| Nested objects | 3-level deep object | Recursive conversion |
| With `$defs`/`$ref` | Schema using refs | End-to-end: resolve then convert |
| With `x-` extensions | `{type: string, x-ui-widget: "textarea"}` | Extensions stripped for structural, preserved for JSON schema |
| Enum | `{type: string, enum: [a, b, c]}` | Enum values preserved |
| Map (additionalProperties) | `{type: object, additionalProperties: {type: string}}` | Map type conversion |
| Invalid schema | Missing `type` on nested property | Clear error message |
| Empty schema | `{}` | Returns empty object schema |

---

#### 2.3 Update Core Resolution Functions in `definition.go`

**File:** `internal/schema/definition.go`

**What changes:** The `ResolveSectionToStructural` and `ResolveSectionToBundle` functions currently always go through `extractor.ExtractSchema()` (which parses ocSchema shorthand syntax). They need to branch: if the section is openAPIV3, use the new converter; otherwise use the existing extractor path.

**Current flow:**
```
ResolveSectionToStructural(section)
  → sectionRaw(section) → GetRaw() → picks whichever is set
  → unmarshalSection(raw) → yaml.Unmarshal → map[string]any
  → ToStructural(Definition{Schemas: [fields]})
    → extractor.ExtractSchema(fields) ← THIS ONLY WORKS FOR ocSchema
    → apiextschema.NewStructural()
```

**New flow:**
```
ResolveSectionToStructural(section)
  → if section.IsOpenAPIV3():
      → unmarshalSection(section.OpenAPIV3Schema)
      → OpenAPIV3ToStructural(fields)     ← NEW PATH
  → else:
      → unmarshalSection(section.OCSchema)
      → ToStructural(Definition{...})     ← EXISTING PATH
```

Same branching for `ResolveSectionToBundle`.

**Why this is the right place:** `ResolveSectionToStructural` and `ResolveSectionToBundle` are the single chokepoints where all consumers enter the schema system. By branching here, **no downstream code needs to change** — webhooks, pipeline, scaffold, and API services all call these functions and will automatically get correct behavior for both formats.

**Impact analysis — what does NOT need to change if we branch here:**
- `schemautil.ExtractStructuralSchemas` — already calls `ResolveSectionToStructural`, no change needed
- `BuildStructuralSchemas` in pipeline — already calls `ResolveSectionToBundle`, no change needed
- 4 webhook files — no change needed
- 3 pipeline callers — no change needed

**What still needs separate handling:**
- API service schema response (2.6) — needs to return raw JSON Schema for openAPIV3, not run through `ToJSONSchema()`
- Scaffold generator (2.5) — may need to parse properties differently

---

#### 2.4 Update API Services for Schema Response

**Problem:** API services currently call `schema.ToJSONSchema(Definition{...})` which runs the ocSchema extractor. For `openAPIV3Schema`, we should resolve `$ref`s and return the JSON Schema directly — it's already in the right format.

**Files to update:**
- `internal/openchoreo-api/services/componenttype/service.go` — `GetComponentTypeSchema()`
- `internal/openchoreo-api/services/clustercomponenttype/service.go` — `GetClusterComponentTypeSchema()`
- `internal/openchoreo-api/services/trait/service.go` — `GetTraitSchema()`
- `internal/openchoreo-api/services/clustertrait/service.go` — `GetClusterTraitSchema()`
- `internal/openchoreo-api/legacyservices/componenttype_service.go`
- `internal/openchoreo-api/legacyservices/clustercomponenttype_service.go`
- `internal/openchoreo-api/legacyservices/trait_service.go`
- `internal/openchoreo-api/legacyservices/clustertrait_service.go`
- Workflow services (Phase 3, but list here for completeness)

**Approach — add a helper function in `internal/schema/definition.go`:**
```go
// SectionToJSONSchema converts a SchemaSection to JSON Schema for API responses.
// Handles both ocSchema (via extractor) and openAPIV3Schema (via ref resolution).
func SectionToJSONSchema(section *v1alpha1.SchemaSection) (*extv1.JSONSchemaProps, error)
```

Then each API service calls this helper instead of manually unmarshaling + calling `ToJSONSchema()`. This reduces the ~12 call sites to a single pattern:
```go
jsonSchema, err := schema.SectionToJSONSchema(ct.Spec.Parameters)
```

---

#### 2.5 Update Scaffold Generator

**Files:** `internal/scaffold/component/generator.go`, `schema_defaults.go`

**Current behavior:** The scaffold generator takes a ComponentType schema, extracts field names/types/defaults, and generates a starter Component YAML with sensible defaults.

**What changes:**
- `extractAndConvertSchema()` currently parses the raw schema through the ocSchema extractor
- For `openAPIV3Schema`: the raw blob is already valid JSON Schema — resolve `$ref`s, then extract `properties` directly
- Use `ResolveSectionToBundle` (which already branches after 2.3) to get the JSON Schema, then extract properties from it

**This may require minimal changes** if the scaffold generator already uses `ResolveSectionToBundle` or similar. Check the actual code path before implementing.

---

#### 2.6 Add Webhook Schema Compilation Validation

**Files:** 4 webhook files (componenttype, clustercomponenttype, trait, clustertrait)

**Current state:** The kubebuilder XValidation rule on `SchemaSection` already enforces mutual exclusivity at the CRD level:
```go
// +kubebuilder:validation:XValidation:rule="!(has(self.ocSchema) && has(self.openAPIV3Schema))"
```

**Additional webhook validation needed:**
1. **Schema compilation check** — When `openAPIV3Schema` is provided, validate that it can be successfully compiled:
   - `$ref` resolution succeeds (no missing refs, no circular refs)
   - Result converts to a valid K8s structural schema
   - This catches errors at admission time with clear messages, rather than failing silently during rendering

**Implementation:** This is already covered by the existing `schemautil.ExtractStructuralSchemas()` call in each webhook, which calls `ResolveSectionToStructural()`. After 2.3, this will automatically exercise the new OpenAPIV3 path and surface resolution/conversion errors as admission errors.

**No code change needed in webhooks** — just verify with tests that:
- Invalid `$ref` in openAPIV3Schema → webhook rejects with clear error
- Circular `$ref` → webhook rejects
- Valid openAPIV3Schema → webhook accepts

---

#### 2.7 Tests

All tests should be written alongside their respective tasks (2.1–2.6). This section consolidates the test plan.

**Unit tests:**

| File | Covers | Key cases |
|------|--------|-----------|
| `internal/schema/ref_resolver_test.go` | 2.1 | See table in 2.1 |
| `internal/schema/openapiv3_test.go` | 2.2 | See table in 2.2 |
| `internal/schema/definition_test.go` | 2.3 | Add cases: `ResolveSectionToStructural` with openAPIV3Schema input, `ResolveSectionToBundle` with openAPIV3Schema input |

**Integration tests:**

| Test | Covers | Validates |
|------|--------|-----------|
| Webhook integration | 2.6 | ComponentType with openAPIV3Schema accepted; invalid schema rejected; both formats set → rejected by XValidation |
| Pipeline integration | 2.3 | End-to-end: openAPIV3Schema CT + Trait → `BuildComponentContext` → defaults applied → validation passes |
| API service integration | 2.4 | `GetComponentTypeSchema()` returns correct JSON Schema for both ocSchema and openAPIV3Schema inputs |
| Scaffold integration | 2.5 | Scaffold generates correct Component YAML from openAPIV3Schema-based ComponentType |

**Test data:** Create reusable test fixtures under `internal/schema/testdata/`:
- `simple_openapiv3.yaml` — basic types, constraints, defaults
- `with_refs_openapiv3.yaml` — uses `$defs`/`$ref`
- `nested_openapiv3.yaml` — deeply nested objects
- `invalid_circular_ref.yaml` — circular ref for error testing

---

#### 2.8 Update Samples and OpenAPI Spec

- Create openAPIV3Schema versions of `samples/getting-started/` component types and traits
- Add suffix openAPIV3Schema for new samples generated
- Keep existing ocSchema samples as-is (both formats must work)
- Update `openapi/openchoreo-api.yaml` if any API response shape changes (unlikely — JSON Schema output format is the same)

---

#### 2.9 End-to-End Cluster Validation

- `make generate manifests` — ensure CRD YAML reflects both schema options
- `make go.build` — clean compile
- `make test` — all unit + integration tests pass
- Deploy to k3d cluster:
  - Apply a ComponentType with `openAPIV3Schema` parameters
  - Apply a Component with values matching that schema
  - Verify defaults applied, validation works, rendering produces correct manifests
  - Apply a Trait with `openAPIV3Schema` and attach to component
  - Verify trait creates/patches render correctly
- Test error cases in cluster:
  - Submit ComponentType with invalid `$ref` → admission rejected
  - Submit Component with values violating openAPIV3Schema constraints → validation error

---

#### 2.10 Critical Review and Hardening

After implementation, review:
- Are error messages clear and actionable for platform engineers?
- Does `occ` CLI display openAPIV3Schema-based schemas correctly?
- Is there any performance concern with large schemas (many `$ref` resolutions)?
- Does the scaffold generator produce usable defaults from complex JSON Schema?
- Are there any ocSchema features that don't have a JSON Schema equivalent? (Document if so)
---

#### 2.11 Verify API Server Schema Endpoints Preserve OpenAPIV3Schema Fields

**Problem:** The OpenChoreo API Server schema endpoints (e.g., `GetComponentTypeSchema`, `GetTraitSchema`) must return the full JSON Schema when `openAPIV3Schema` is used, including fields that go beyond basic type/properties:
- `description` on properties
- `additionalProperties: false` (strict validation)
- Vendor extensions like `x-openchoreo-backstage-portal` (used by UI portals for custom widgets)

**Validation:** Verify that `SectionToJSONSchema()` (implemented in 2.4) preserves these fields in the API response. The `OpenAPIV3ToJSONSchema()` path resolves `$ref`s but does **not** strip vendor extensions, so `x-*` keys should be preserved in the JSON schema path. This is only applicable for API Responses. Not in the internal/controller/ or internal/schema/ or internal/pipeline/ paths.

**Test with this example schema:**
```yaml
parameters:
  openAPIV3Schema:
    type: object
    additionalProperties: false
    properties:
      repository:
        type: object
        additionalProperties: false
        properties:
          url:
            type: string
            description: Git repository URL
            x-openchoreo-backstage-portal:
              ui:field: RepoUrlPicker
              ui:options:
                allowedHosts:
                  - github.com
          secretRef:
            type: string
            description: Secret reference name for private repository Git credentials (optional for public repos)
```

**Expected behavior:**
- API response includes `description` fields on properties
- API response includes `additionalProperties: false` on object schemas
- API response includes `x-openchoreo-backstage-portal` vendor extension (for UI consumption)
- Structural schema path (used by webhooks/pipeline) strips `x-*` keys correctly — no admission errors

**Action items:**
1. Create a ComponentType YAML with the above schema
2. Apply to cluster and call the schema API endpoint via curl
3. Verify the JSON response contains all expected fields
4. If vendor extensions are dropped by `extv1.JSONSchemaProps` marshaling, investigate preserving them (may need to return raw resolved JSON instead of going through the K8s type)

---

#### 2.12 Create OpenAPIV3Schema Sample Files

**Goal:** Create openAPIV3Schema versions of existing samples so users have reference implementations for both formats.

**Files to create:**
- `samples/getting-started/` — duplicate key component types and traits with `-openapiv3` suffix
  - e.g., `deployment-ct-openapiv3.yaml`, `ingress-trait-openapiv3.yaml`
- Update any other relevant samples as well for traits and component types
- Keep existing ocSchema samples as-is (both formats must work side by side)
- Ensure samples use realistic patterns: `$defs`/`$ref` for type reuse, `description` fields, `default` values, `enum` constraints, `additionalProperties: false`

**Naming convention:** Append `-openapiv3` suffix to the resource name (not the file name) so both can coexist in the same namespace.

**Validation:** After creating samples, run `make samples-gen` to regenerate `all.yaml`.

---

#### 2.13 Test OpenChoreo API Server Endpoints

**Goal:** Manually verify the API server schema endpoints work correctly with openAPIV3Schema-based resources.

**Test plan:**
1. Apply an openAPIV3Schema ComponentType from 2.12
2. Call the schema endpoint and verify response:
   ```bash
   curl -s http://localhost:<port>/api/v1/namespaces/<ns>/componenttypes/<name>/schema | jq .
   ```
3. Verify response contains: `type`, `properties`, `required`, `default`, `description`, `additionalProperties`, vendor extensions
4. Apply a Component that references the ComponentType with valid parameter values — verify accepted
5. Apply a Component with invalid values (missing required, wrong type) — verify rejected with clear error messages
6. Repeat for Traits: apply openAPIV3Schema Trait, call trait schema endpoint, verify
7. Test environmentConfigs schema endpoint if applicable

---

#### 2.14 End-to-End Cluster Validation

**Goal:** Full deployment cycle validation with openAPIV3Schema resources.

**Steps:**
1. `make generate manifests` — ensure CRD YAML reflects both schema options
2. `make go.build` — clean compile
3. `make test` — all unit + integration tests pass
4. Build and load images into k3d:
   ```bash
   make k3d.build && make k3d.load
   ```
5. Install/upgrade via Helm:
   ```bash
   make k3d.install
   ```
6. Rollout restart deployments to pick up new images:
   ```bash
   kubectl rollout restart deployment -n openchoreo-system
   ```
7. Check controller and API server logs for errors:
   ```bash
   kubectl logs -n openchoreo-system deploy/openchoreo-controller-manager -f
   kubectl logs -n openchoreo-system deploy/openchoreo-api -f
   ```
8. Apply openAPIV3Schema samples from 2.12
9. Verify:
   - ComponentType accepted, shows in `kubectl get componenttypes`
   - Component with matching parameters accepted
   - Defaults applied correctly (check Component status or rendered release)
   - Validation works: submit Component with values violating constraints → rejected
   - Trait with openAPIV3Schema accepted and renders correctly
   - Schema API endpoints return correct JSON Schema (including `description`, `additionalProperties`, vendor extensions)
10. Test error cases:
    - ComponentType with invalid `$ref` → admission rejected with clear error
    - ComponentType with circular `$ref` → admission rejected
    - Component with unknown fields → validation error

---

### Phase 3: Add OpenAPI V3 Schema Support for Workflows

**Goal:** Same pattern as Phase 2 but for workflows (parameters only, no environmentConfig). Also fix remaining code paths that bypassed the unified schema resolution functions.

#### 3.1 Fix Workflow Pipeline

**File:** `internal/pipeline/workflow/pipeline.go`
- `buildStructuralSchema()` was bypassing `ResolveSectionToStructural()` and calling `schema.ToStructural()` directly (ocSchema-only)
- Fix: use `schema.ResolveSectionToStructural(wf.Spec.Parameters)` which handles both formats transparently

#### 3.2 Workflow API Services

- `internal/openchoreo-api/services/workflow/service.go` — already updated in Phase 2 (uses `SectionToRawJSONSchema`)
- `internal/openchoreo-api/legacyservices/workflow_service.go` — already updated in Phase 2 (uses `SectionToJSONSchema`)

#### 3.3 Tests + Samples

- Write pipeline tests for workflow openAPIV3Schema defaults (including `$defs`/`$ref` resolution)
- Convert all 4 workflow samples to openAPIV3Schema equivalents

#### 3.4 Fix ComponentRelease Webhook (found during critical review)

**File:** `internal/webhook/componentrelease/webhook.go`
- `validateComponentParameters()` and `validateTraitInstanceParameters()` used `schema.Definition{}` + `schema.ToJSONSchema()` directly — ocSchema-only path
- Fix: use `schema.SectionToJSONSchema(section)` which handles both formats

#### 3.5 Fix Legacy Component Service (found during critical review)

**File:** `internal/openchoreo-api/legacyservices/component_service.go`
- `validateWorkflowParameters()` used `schema.Definition{}` + `schema.ToStructural()` directly — ocSchema-only path
- Fix: use `schema.ResolveSectionToStructural(workflowSpec.Parameters)` which handles both formats

---

## Implementation Checklist

### Phase 1 (Restructure to top-level parameters/environmentConfigs with ocSchema)
- [x] 1.1 Define `SchemaSection` shared type and update `ComponentTypeSpec` (no separate `Types` field)
- [x] 1.1 Update `TraitSpec` with same pattern
- [x] 1.1 Update `WorkflowSpec` with same pattern (parameters only)
- [x] 1.2 Update `Source` interface (remove `GetTypes()`, rename `GetEnvOverrides()` → `GetEnvironmentConfigs()`)
- [x] 1.2 Update schema extractor to parse `$types` from within ocSchema blob
- [x] 1.3 Update pipeline context (3 call sites: component.go, trait.go, embedded_trait.go) — fully renamed to `environmentConfigs`
- [x] 1.3 Update pipeline types (`SchemaInput` — `EnvOverridesSchema` → `EnvironmentConfigsSchema`)
- [x] 1.3 Update `schemautil.ExtractStructuralSchemas` — field path `"environmentConfigs"`
- [x] 1.3 Update 5 webhook files — rename in progress (`EnvironmentConfig` → `EnvironmentConfigs`, `SimpleSource.EnvOverrides` → `SimpleSource.EnvironmentConfigs`)
- [x] 1.3 Update ~12 API service files — rename in progress
- [x] 1.3 Update workflow pipeline + services
- [x] 1.3 Update scaffold generator + tests
- [x] 1.3 Update CLI typed output files
- [x] 1.4 Regenerate deepcopy + CRD manifests (`make generate manifests`) — needs re-run after rename completes
- [x] 1.4 Update `openapi/openchoreo-api.yaml` — updated to `environmentConfigs`
- [x] 1.4 Update all sample YAML files — all converted to `environmentConfigs:`
- [x] 1.4 Update all test YAML testdata — converted to `environmentConfigs:`
- [x] 1.5 Verify clean build after full rename propagation
- [x] 1.6 Update remaining Go test files for struct field rename
- [x] 1.7 Update `openapi/openchoreo-api.yaml` — `environmentConfigs` property names
- [x] 1.8 Update CLI typed output (`typed/componenttype.go`, `typed/trait.go`) — code updated
- [x] 1.9 Full cluster validation: `make install`, build/load images, rollout restart, apply samples, verify rendering
- [x] 1.10 Rename `envOverrides` → `environmentConfigs` in remaining Go files (webhooks, services, controllers, validation, template, MCP, CLI constants)
- [x] 1.11 Rename `traitOverrides` → `traitEnvironmentConfigs` in ReleaseBinding (Go types, JSON tags, MCP tool schema/struct, OpenAPI spec, API models, all samples/testdata/docs, RCA agent, generated files regenerated)

### Phase 2 (OpenAPI V3 Schema for ComponentTypes + Traits)
- [x] 2.1 Implement `$ref`/`$defs` resolver (`internal/schema/ref_resolver.go`) + unit tests (15 tests: simple ref, nested chain, siblings, circular, missing, remote, items, allOf, oneOf, additionalProperties, depth limit, no refs, backward compat, nil, no mutation)
- [x] 2.2 Implement OpenAPIV3 → K8s type converter (`internal/schema/openapiv3.go`) + unit tests (18 tests: primitives, refs, JSON schema, vendor extensions preserved/stripped, both formats, nested, array, enum, map, empty, array of objects, boolean/number, pattern/format, nested defaults through refs, defaults applied)
- [x] 2.3 Branch `ResolveSectionToStructural` / `ResolveSectionToBundle` in `definition.go` on `IsOpenAPIV3()` — this is the chokepoint; webhooks + pipeline + scaffold all flow through here automatically
- [x] 2.4 Add `SectionToJSONSchema()` helper in `definition.go`; update ~14 API service files to use it (6 services, 5 legacy services, 2 component service files)
- [x] 2.5 Update scaffold generator to use `SectionToJSONSchema()` instead of `extractAndConvertSchema()` — removed unused function
- [x] 2.6 Verify webhook schema compilation catches invalid openAPIV3Schema at admission — no code change needed, webhooks flow through `ResolveSectionToStructural` automatically
- [x] 2.7 Write unit tests: definition.go integration tests for OpenAPIV3 path (22 tests total in `definition_test.go` — structural, refs, bundle, nil, JSON schema, OCSchema, defaults, testdata-based, validation, end-to-end pipeline)
- [x] 2.8 Create openAPIV3Schema test fixtures under `internal/schema/testdata/` (4 files: simple, with_refs, nested, invalid_circular_ref) + testdata-based tests in definition_test.go
- [ ] 2.9 End-to-end cluster validation: deploy CT/Trait/Component with openAPIV3Schema, verify defaults + validation + rendering
- [ ] 2.10 Critical review: error messages, CLI display, performance, feature parity gaps
- [x] 2.11 Verify API server schema endpoints preserve `description`, `additionalProperties`, and `x-*` vendor extensions — **Fixed:** `extv1.JSONSchemaProps` silently drops arbitrary `x-*` extensions. Added `SectionToRawJSONSchema()` returning `map[string]any` to preserve vendor extensions. Changed all 6 schema service interfaces (`componenttype`, `trait`, `clustercomponenttype`, `clustertrait`, `workflow`, `clusterworkflow`) + their authz wrappers + handlers to use `map[string]any` return type. Added 12 new tests (4 in openapiv3_test.go, 5 in definition_test.go for vendor extension preservation, $ref sibling merging, nested extensions, input immutability, and ocSchema fallback). CRUD operations (CREATE/GET/LIST/UPDATE) already preserve x-* via `runtime.RawExtension` raw bytes — no fix needed.
- [x] 2.12 Create openAPIV3Schema sample files in `samples/getting-started/` with `-openapiv3` suffix (service, webapp, worker, scheduled-task CTs + alert-rule trait) — service sample includes `x-openchoreo-*` vendor extensions
- [ ] 2.13 Test OpenChoreo API Server schema endpoints via curl — verify correct JSON Schema responses for both formats
- [ ] 2.14 Full cluster deployment validation: build, load into k3d, rollout restart, apply samples, check logs, verify rendering

### Phase 3 (OpenAPI V3 Schema for Workflows)
- [x] 3.1 Fix workflow pipeline `buildStructuralSchema()` — replaced `schema.ToStructural(def)` with `schema.ResolveSectionToStructural(wf.Spec.Parameters)` to handle both ocSchema and openAPIV3Schema
- [x] 3.2 Workflow API services — already updated in Phase 2 (workflow + clusterworkflow services use `SectionToRawJSONSchema`, legacy uses `SectionToJSONSchema`)
- [x] 3.3 Pipeline tests — `TestPipeline_Render_OpenAPIV3Schema_Defaults` with 5 subtests (openAPIV3 defaults, $defs/$ref defaults, ocSchema defaults, empty params, nil schema)
- [x] 3.4 Workflow sample files — created openAPIV3 equivalents for all 4 workflows (docker, react, ballerina-buildpack, google-cloud-buildpacks)
- [x] 3.5 Fix ComponentRelease webhook — `validateComponentParameters()` and `validateTraitInstanceParameters()` were using `schema.Definition{}` + `schema.ToJSONSchema()` (ocSchema-only). Fixed to use `schema.SectionToJSONSchema(section)`
- [x] 3.6 Fix legacy component service — `validateWorkflowParameters()` was using `schema.Definition{}` + `schema.ToStructural()` (ocSchema-only). Fixed to use `schema.ResolveSectionToStructural(workflowSpec.Parameters)`
- [ ] 3.7 Run `make samples-gen` to regenerate `all.yaml`
- [ ] 3.8 Run full test suite (`make test`)
- [ ] 3.9 E2E cluster validation
