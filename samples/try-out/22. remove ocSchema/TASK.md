# Remove `ocSchema` from OpenChoreo

## Context

`ocSchema` is a shorthand schema format (e.g., `replicas: "integer | default=1"`) that gets converted to standard OpenAPI V3 JSON Schema internally. It lived in `SchemaSection` alongside `openAPIV3Schema` as a mutually exclusive alternative.

We are removing it because it's a non-standard format that adds complexity. The public API surface (CRD fields + REST API) must stop accepting it before the upcoming release to avoid post-release breaking changes.

---

## Phase 1: Remove `ocSchema` from the public API surface (pre-release) — DONE

**Goal:** Users can no longer submit or receive `ocSchema` through any API. Only `openAPIV3Schema` is accepted.

See [IMPLEMENTATION.md](IMPLEMENTATION.md) for full details of what was done.

**Summary of changes:**
- [x] 1.1 — Removed `OCSchema` field from `SchemaSection` struct, simplified `GetRaw()`
- [x] 1.2 — Removed `ocSchema` from OpenAPI spec `SchemaSection` definition
- [x] 1.3 — Regenerated all code (`make generate manifests openapi-codegen helm-generate samples-gen`)
- [x] 1.4 — Updated comments in 6 legacy service files
- [x] 1.5 — Updated webhook test fixtures (`OCSchema:` → `OpenAPIV3Schema:`)
- [x] 1.6 — Removed `validateSchemaFormatConsistency()` from validation utilities
- [x] 1.7 — Updated all controller/CLI test fixtures
- [x] 1.8 — Renamed `ocSchema:` → `openAPIV3Schema:` in all sample YAML files
- [x] 1.9 — Deleted `docs/templating/openchoreo-schema.md`, updated `openapiv3-schema.md` and `validations.md`
- [x] 1.10 — Verified: `make test` passes, `make lint-fix` clean, `make go.build` succeeds

---

## Phase 2: Refactor internal schema library to remove shorthand support

**Goal:** Clean up all internal code that converts shorthand schema syntax to JSON Schema. This is safe to do after release since it's purely internal.

**Key context:** In Phase 1, we introduced `isOpenAPIV3Content()` in `internal/schema/definition.go` — a content-based detection function that routes shorthand content to the extractor even though the YAML key is now `openAPIV3Schema:`. This was necessary because many internal test fixtures still use shorthand values. Phase 2 converts those values to proper OpenAPI V3 and removes the detection + extractor.

### 2.1 — Convert all test data from shorthand to proper OpenAPI V3

Convert inline YAML and testdata files that still use shorthand syntax (e.g., `replicas: "integer | default=1"`) under `openAPIV3Schema:` to proper JSON Schema format (e.g., `type: object, properties: {replicas: {type: integer, default: 1}}`).

**Files with shorthand content under `openAPIV3Schema:`:**
- `internal/pipeline/component/pipeline_test.go` (~15 inline YAML blocks)
- `internal/pipeline/component/benchmark_test.go` (2 inline YAML blocks)
- `internal/pipeline/component/context/builder_test.go` (7 inline YAML blocks)
- `internal/pipeline/component/testdata/component-with-traits.yaml`
- `internal/pipeline/component/testdata/configurations-and-secrets/snapshot.yaml`
- `internal/pipeline/component/testdata/configurations-and-secrets/snapshot-with-config-helpers.yaml`
- `internal/scaffold/component/testdata/*.yaml` (11 files)
- `samples/ocSchema/` directory (all files still use shorthand values)

### 2.2 — Remove `isOpenAPIV3Content()` and shorthand routing

**File:** `internal/schema/definition.go`

- Remove `isOpenAPIV3Content()` function
- Remove shorthand fallback branches in:
  - `ResolveSectionToStructural()`
  - `ResolveSectionToBundle()`
  - `SectionToJSONSchema()`
  - `SectionToRawJSONSchema()`
- All content is now proper OpenAPI V3 — go directly to `OpenAPIV3To*()` functions

### 2.3 — Remove schema extractor

**Directory:** `internal/schema/extractor/`

- Remove `ExtractSchema()` and all shorthand parsing logic in `schema.go`
- Remove corresponding tests in `schema_test.go`
- Remove `ToJSONSchema()` and `ToStructural()` from `definition.go` (shorthand entry points)
- Remove `ToStructuralAndJSONSchema()` if unused after cleanup
- Update `definition_test.go`

### 2.4 — Remove `IsOpenAPIV3()` method

**File:** `api/v1alpha1/componenttype_types.go`

- Remove `IsOpenAPIV3()` — always true now, no callers after 2.2
- Audit remaining callers of `GetRaw()` — simplify or inline if appropriate

### 2.5 — Verification

```bash
make test       # all unit tests pass
make lint       # clean
make go.build   # builds successfully
```
