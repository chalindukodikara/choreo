# OpenAPI V3 Schema for OpenChoreo — Research Findings

## Summary

This document consolidates all research, analysis, and decisions made around adopting OpenAPI V3 Schema (JSON Schema) as a first-class schema format in OpenChoreo alongside the existing Simple Schema (Kro-based).

---

## 1. Background

OpenChoreo uses schemas in three CRD types — **ComponentType**, **Trait**, and **Workflow** — to define validation rules for user-provided configuration (`parameters` and `envOverrides`). The current schema format is Kro's Simple Schema (v0.8.5), which is concise but unstable, not widely adopted, and lacks extensibility.

**Decision:** OpenChoreo 1.0 will support both formats:
```yaml
spec:
  schema:
    openAPIV3Schema:     # OpenAPI V3 / JSON Schema (new, default for 1.0)
      parameters: ...
      envOverrides: ...
    ocSchema:            # Current Simple Schema (name TBD, still available)
      types: ...
      parameters: ...
      envOverrides: ...
```

---

## 2. What is OpenAPI V3 Schema?

OpenAPI V3 Schema is based on JSON Schema with some modifications:

| Aspect | OpenAPI 3.0 | OpenAPI 3.1 | Kubernetes CRDs |
|--------|-------------|-------------|-----------------|
| Base | JSON Schema Draft-07 (modified subset) | JSON Schema Draft 2020-12 (full alignment) | OpenAPI 3.0 subset (structural schemas) |
| `$ref`/`$defs` | `$ref` replaces entire schema, no `$defs` | Full support | **Not supported** |
| `if/then/else` | Not supported | Supported | Not supported |
| `const` | Not supported | Supported | Not supported (use single-value `enum`) |
| `exclusiveMinimum/Maximum` | Boolean (modifies min/max) | Numeric value | Boolean |
| `nullable` | `nullable: true` | Use `type: ["string", "null"]` | `nullable: true` |
| `$ref` with siblings | Siblings ignored | Siblings preserved | N/A |

**Key insight:** Kubernetes CRDs use a restricted "structural schema" subset. But OpenChoreo's schemas are stored as **opaque data inside CRD instances** (not as CRD definitions themselves), so we can support features like `$defs`/`$ref` and resolve them internally.

**References:**
- JSON Schema: https://json-schema.org/
- OpenAPI 3.0: https://swagger.io/specification/v3/
- OpenAPI 3.1: https://spec.openapis.org/oas/v3.1.0
- K8s CRD Structural Schemas: https://kubernetes.io/blog/2019/06/20/crd-structural-schema/

---

## 3. Schema Structure Decision

### Question: One JSON Schema document or two?

**Option A — Single document:** `openAPIV3Schema` is one JSON Schema root with `parameters` and `envOverrides` as properties. `$defs` lives at the root and is shared.

```yaml
openAPIV3Schema:
  type: object
  $defs:
    ResourceQuantity: { ... }
  properties:
    parameters: { ... }
    envOverrides: { ... }
```

- Pro: Literally a valid JSON Schema document. Any validator compiles it directly.
- Con: Feels awkward — `parameters`/`envOverrides` are buried under `properties:`.

**Option B — Bundle (recommended):** `parameters` and `envOverrides` are independent schema roots. `$defs` at the bundle root is shared across both.

```yaml
openAPIV3Schema:
  parameters:
    type: object
    properties: { ... }
  envOverrides:
    type: object
    properties: { ... }
```

- Pro: Cleaner structure, intuitive separation.
- Con: `$defs` is outside each schema root — controller must inject/wrap `$defs` into each schema before validation.

**Decision: Option B.** The controller assembles valid schema roots by merging `$defs` into each schema at compile-time. This can be done by either:
1. Wrapping into Option A internally (single root with `properties`)
2. Copying `$defs` into each schema root before compilation

**Important:** We must use a JSON Schema Draft 2020-12 parser (not Draft 4/7 or OpenAPI 3.0) because `$ref` with sibling keywords (like `default: {}`) is only supported from Draft 2019-09+.

---

## 4. Limitations and Challenges

### 4.1 Custom Types / Type Reuse

| Approach | Description | Verdict |
|----------|-------------|---------|
| Inline everything | Duplicate type definitions at every usage site | K8s-compatible but verbose and error-prone |
| **`$defs`/`$ref` with internal resolution** | Allow `$defs`/`$ref` in authoring; OpenChoreo resolves before validation | **Recommended** — matches Crossplane/Helm approach |
| Keep `types` as separate concept | Non-standard, confusing for JSON Schema users | Not recommended |

Implementation must handle: circular references (error or depth limit), default merging (`default` adjacent to `$ref` overrides the referenced schema's default), and nested `$ref` chains.

### 4.2 Verbosity

OpenAPI V3 Schema is ~3.5x more verbose than Simple Schema:

- Simple Schema: 5 lines for 3 fields with constraints
- OpenAPI V3 Schema: 18 lines for the same

**Mitigation:** Keep Simple Schema as the "fast" option. Provide conversion tooling. `$defs`/`$ref` reduces duplication in complex schemas. IDE autocompletion offsets verbosity.

### 4.3 Default Handling

Clean mapping: Simple Schema's `$default: {}` maps directly to JSON Schema's `default: {}`. Kubernetes applies defaults recursively during admission. No behavioral mismatch.

| Simple Schema | OpenAPI V3 Schema |
|---------------|-------------------|
| `$default: {}` on object | `default: {}` on schema node |
| `default=value` on field | `default: value` on field |
| No default = required | Field name in parent's `required` array |

### 4.4 `additionalProperties` and Schema Evolution

Simple Schema allows additional properties by default (no `additionalProperties: false`). In Kubernetes CRDs, unknown fields are pruned by default.

Since OpenChoreo schemas are opaque data (not CRD definitions), OpenChoreo's controller controls pruning behavior. Recommendation: default to `additionalProperties: true` to match Simple Schema's evolution-friendly behavior. Optionally support `x-openchoreo-strict-mode: true` for strict validation.

### 4.5 Custom Annotations

Clean mapping from `oc:*` to `x-openchoreo-*` vendor extensions:

| Simple Schema | OpenAPI V3 |
|---------------|------------|
| `oc:build:inject=git.sha` | `x-openchoreo-build-inject: git.sha` |
| `oc:ui:hidden=true` | `x-openchoreo-ui-hidden: true` |
| `oc:scaffolding=omit` | `x-openchoreo-scaffolding: omit` |

**Advantage:** `x-openchoreo-*` supports rich values (objects, arrays, booleans), not just strings.

### 4.6 Backstage Integration

OpenAPI vendor extensions enable clean Backstage UI metadata:

```yaml
repoUrl:
  type: string
  x-openchoreo-backstage-portal:
    ui:field: RepoUrlPicker
    ui:options:
      allowedHosts: ["github.com"]
```

This eliminates the need for out-of-band UI metadata that Simple Schema cannot express.

### 4.7 Validation Responsibilities

| What | Who |
|------|-----|
| Is the ComponentType YAML valid? | Kubernetes CRD validation |
| Is `openAPIV3Schema` a valid JSON Schema? | **OpenChoreo controller** (K8s treats it as opaque) |
| Do Component `parameters` match the schema? | **OpenChoreo controller** |
| Do ReleaseBinding `envOverrides` match the schema? | **OpenChoreo controller** |
| Are defaults applied correctly? | **OpenChoreo controller** |

Use a Go JSON Schema library (e.g., `santhosh-tekuri/jsonschema/v6` with Draft 2020-12).

### 4.8 CEL Templating Interaction

**No impact.** CEL templates operate on resolved values, not schemas. Both formats produce the same value structure. The only risk is if defaulting logic differs between formats — ensure consistency with integration tests.

---

## 5. Challenge Severity Summary

| Challenge | Severity | Recommendation |
|-----------|----------|----------------|
| No `$ref`/`$defs` in K8s CRDs | **High** | Support at authoring time; resolve internally |
| Verbosity (~3.5x) | **Medium** | Keep Simple Schema as option; provide tooling |
| Default handling | **Low** | Direct 1:1 mapping |
| `additionalProperties` behavior | **Medium** | Default to `true`; document clearly |
| Custom annotation migration | **Low** | Clean `oc:*` -> `x-openchoreo-*` mapping |
| Schema validation in controller | **Medium** | Required regardless; use `santhosh-tekuri/jsonschema/v6` |
| CEL template interaction | **Low** | Transparent if defaulting is consistent |

---

## 6. Converted Samples

### 6.1 Trait: horizontal-pod-autoscaler

**Simple Schema:**
```yaml
parameters:
  enabled: "boolean | default=false"
  minReplicas: "integer | default=2 | minimum=1"
  maxReplicas: "integer | default=10 | minimum=1"
  targetCPUUtilizationPercentage: "integer | default=80 | minimum=1 | maximum=100"
envOverrides:
  minReplicas: "integer | minimum=1"
  maxReplicas: "integer | minimum=1"
```

**OpenAPI V3 Schema:**
```yaml
openAPIV3Schema:
  parameters:
    type: object
    properties:
      enabled:
        type: boolean
        default: false
      minReplicas:
        type: integer
        default: 2
        minimum: 1
      maxReplicas:
        type: integer
        default: 10
        minimum: 1
      targetCPUUtilizationPercentage:
        type: integer
        default: 80
        minimum: 1
        maximum: 100
  envOverrides:
    type: object
    required: [minReplicas, maxReplicas]
    properties:
      minReplicas:
        type: integer
        minimum: 1
      maxReplicas:
        type: integer
        minimum: 1
```

### 6.2 Trait: persistent-volume

**Simple Schema:**
```yaml
parameters:
  volumeName: "string"
  mountPath: "string"
  containerName: "string | default=main"
envOverrides:
  size: "string | default=10Gi"
  storageClass: "string | default=local-path"
```

**OpenAPI V3 Schema:**
```yaml
openAPIV3Schema:
  parameters:
    type: object
    required: [volumeName, mountPath]
    properties:
      volumeName:
        type: string
      mountPath:
        type: string
      containerName:
        type: string
        default: main
  envOverrides:
    type: object
    properties:
      size:
        type: string
        default: "10Gi"
      storageClass:
        type: string
        default: local-path
```

### 6.3 ComponentType: service-with-autoscaling (with `$defs`/`$ref`)

**Simple Schema:**
```yaml
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
  autoscaling:
    minReplicas: "integer | minimum=1"
    maxReplicas: "integer | minimum=1"
```

**OpenAPI V3 Schema (authoring format with `$defs`):**
```yaml
openAPIV3Schema:
  parameters:
    type: object
    properties:
      autoscaling:
        type: object
        properties:
          enabled: { type: boolean, default: false }
          minReplicas: { type: integer, default: 2, minimum: 1 }
          maxReplicas: { type: integer, default: 10, minimum: 1 }

  envOverrides:
    type: object
    properties:
      replicas: { type: integer, default: 1 }
      imagePullPolicy: { type: string, default: IfNotPresent }
      autoscaling:
        type: object
        required: [minReplicas, maxReplicas]
        properties:
          minReplicas: { type: integer, minimum: 1 }
          maxReplicas: { type: integer, minimum: 1 }
```

### 6.4 Workflow: docker

**Simple Schema:**
```yaml
parameters:
  repository:
    url: string | description="Git repository URL"
    secretRef: string | description="Secret reference name..."
    revision:
      branch: string | default=main description="Git branch to checkout"
      commit: string | description="Git commit SHA or reference..."
    appPath: string | default=. description="Path to the application directory..."
  docker:
    context: string | default=. description="Docker build context path..."
    filePath: string | default=./Dockerfile description="Path to the Dockerfile..."
```

**OpenAPI V3 Schema:**
```yaml
openAPIV3Schema:
  parameters:
    type: object
    required: [repository]
    properties:
      repository:
        type: object
        required: [url, secretRef, revision]
        properties:
          url:
            type: string
            description: "Git repository URL"
          secretRef:
            type: string
            description: "Secret reference name for private repository Git credentials"
          revision:
            type: object
            required: [commit]
            properties:
              branch:
                type: string
                default: main
                description: "Git branch to checkout"
              commit:
                type: string
                description: "Git commit SHA or reference"
          appPath:
            type: string
            default: "."
            description: "Path to the application directory within the repository"
      docker:
        type: object
        properties:
          context:
            type: string
            default: "."
            description: "Docker build context path relative to the repository root"
          filePath:
            type: string
            default: "./Dockerfile"
            description: "Path to the Dockerfile relative to the repository root"
```

---

## 7. Quick Conversion Cheat Sheet

| Simple Schema | OpenAPI V3 Schema |
|---------------|-------------------|
| `field: string` | `field: {type: string}` |
| `field: "string \| default=foo"` | `field: {type: string, default: "foo"}` |
| `field: "integer \| minimum=1 maximum=10"` | `field: {type: integer, minimum: 1, maximum: 10}` |
| `field: "boolean \| default=false"` | `field: {type: boolean, default: false}` |
| `field: "[]string"` | `field: {type: array, items: {type: string}}` |
| `field: "[]string \| default=[]"` | `field: {type: array, items: {type: string}, default: []}` |
| `field: "map<string>"` | `field: {type: object, additionalProperties: {type: string}}` |
| `field: "map<string> \| default={}"` | `field: {type: object, additionalProperties: {type: string}, default: {}}` |
| `field: "string \| enum=a,b,c"` | `field: {type: string, enum: [a, b, c]}` |
| `field: "string \| pattern=^[a-z]+$"` | `field: {type: string, pattern: "^[a-z]+$"}` |
| `field: "string \| title=T description=D"` | `field: {type: string, title: "T", description: "D"}` |
| `field: "string \| oc:ui:hidden=true"` | `field: {type: string, x-openchoreo-ui-hidden: true}` |
| `field: CustomType` | `field: {$ref: "#/$defs/CustomType"}` |
| `field: "CustomType \| default={}"` | `field: {$ref: "#/$defs/CustomType", default: {}}` |
| `types: { MyType: ... }` | `$defs: { MyType: ... }` |
| `$default: {}` | `default: {}` |
| No default on field | Field name in parent's `required` array |

---

## 8. Supported Features Reference

All features currently supported by Simple Schema, expressed in OpenAPI V3 format:

| Feature | OpenAPI V3 Keyword | Example |
|---------|-------------------|---------|
| String type | `type: string` | `name: {type: string}` |
| Integer type | `type: integer` | `count: {type: integer}` |
| Number type | `type: number` | `price: {type: number}` |
| Boolean type | `type: boolean` | `enabled: {type: boolean}` |
| Min/max length | `minLength`, `maxLength` | `{type: string, minLength: 3, maxLength: 20}` |
| Pattern | `pattern` | `{type: string, pattern: "^[a-z]+$"}` |
| Enum | `enum` | `{type: string, enum: [a, b, c]}` |
| Min/max (numeric) | `minimum`, `maximum` | `{type: integer, minimum: 1, maximum: 100}` |
| Exclusive min/max | `exclusiveMinimum`, `exclusiveMaximum` | Boolean in OpenAPI 3.0 |
| Multiple of | `multipleOf` | `{type: number, multipleOf: 0.01}` |
| Default | `default` | `{type: string, default: "foo"}` |
| Arrays | `type: array`, `items` | `{type: array, items: {type: string}}` |
| Array constraints | `minItems`, `maxItems` | `{type: array, minItems: 1, maxItems: 10}` |
| Maps | `additionalProperties` | `{type: object, additionalProperties: {type: string}}` |
| Map constraints | `minProperties`, `maxProperties` | `{type: object, minProperties: 1}` |
| Nested objects | `type: object`, `properties` | Inline YAML |
| Required fields | `required` array | `{type: object, required: [name]}` |
| Reusable types | `$defs` + `$ref` | `{$ref: "#/$defs/MyType"}` |
| Title | `title` | `{title: "App Name"}` |
| Description | `description` | `{description: "Details..."}` |
| Example | `example` | `{example: "my-app"}` |
| Custom annotations | `x-openchoreo-*` | `{x-openchoreo-ui-hidden: true}` |
| Backstage UI | `x-openchoreo-backstage-portal` | `{x-openchoreo-backstage-portal: {ui:field: RepoUrlPicker}}` |
| Schema evolution | `additionalProperties: true` | Allows unknown fields |

---

## 9. How Other Projects Handle This

| Project | Schema Approach |
|---------|----------------|
| **Kubernetes CRDs** | OpenAPI V3 structural schemas (restricted subset, no `$ref`) |
| **Crossplane** | OpenAPI V3 in Compositions; resolves `$ref` during compilation |
| **Helm** | JSON Schema for `values.schema.json`; full JSON Schema support |
| **Backstage** | JSON Schema for template parameters with `ui:*` extensions |
| **Kro** | Custom Simple Schema that compiles to JSON Schema internally |

---

## 10. Implementation Approach: `$ref`/`$defs` Resolution

### Decision: No External JSON Schema Library

After analyzing the codebase and Kubernetes types, we determined that **no new JSON Schema library is needed**. The `santhosh-tekuri/jsonschema/v6` library mentioned in earlier research is unnecessary for the current scope.

**Why:** Both schema formats (ocSchema and openAPIV3Schema) converge to the same Kubernetes type (`apiext.JSONSchemaProps`). The only new capability needed is `$defs`/`$ref` resolution, which is a ~100-150 line tree walk on `map[string]any`.

### Why `map[string]any` Resolution (Not K8s Type-Level)

The Kubernetes `extv1.JSONSchemaProps` type has:
- `Ref *string` (JSON tag: `$ref`) — exists but rejected by `NewStructural()`
- `Definitions JSONSchemaDefinitions` (JSON tag: `definitions`) — Draft 4/7 keyword, also rejected by `NewStructural()`
- **No `$defs` field** — that's JSON Schema 2020-12, not in OpenAPI 3.0

Since users write `$defs` (2020-12 style) and K8s types don't have a `$defs` field, resolution must happen at the `map[string]any` level before unmarshaling into K8s types.

### Processing Pipeline

```
OpenAPIV3Schema raw YAML
  → yaml.Unmarshal → map[string]any
  → ResolveRefs():
      1. Extract $defs (or definitions) from top-level
      2. Walk tree, replace $ref nodes with referenced definitions
      3. Merge $ref siblings (default, description, etc.) — siblings win
      4. Detect circular references → error
      5. Remove $defs/definitions from output
  → Strip x-* vendor extensions (K8s types don't support them)
  → json.Marshal → json.Unmarshal → extv1.JSONSchemaProps
  → Convert_v1_To_apiextensions → apiext.JSONSchemaProps
  → apiextschema.NewStructural() → Structural
  → [same downstream as ocSchema: Prune → Defaults → Validate → CEL]
```

### What a Library Would Add (Future)

If we later need:
- **Meta-schema validation** (validate that user's schema IS valid JSON Schema) → `santhosh-tekuri/jsonschema/v6`
- **Advanced features** (`if/then/else`, `$dynamicRef`, `$anchor`) → full library needed
- **Remote `$ref` resolution** (references to external URLs) → library or custom HTTP fetcher

For now, local `$ref`/`$defs` resolution + K8s structural validation is sufficient.

---

## 11. Deliverables Produced

| File | Content |
|------|---------|
| `research-references-claude-1.md` | Full research on JSON Schema vs OpenAPI V3, Kubernetes CRD usage, feature availability |
| `limitations-analysis.md` | Detailed analysis of 7 challenges with recommendations |
| `component-with-embedded-traits-openapiv3-claude-1.md` | Converted ComponentType + Traits sample (both `$defs`/`$ref` and fully-inlined versions) |
| `workflow-docker-openapiv3-claude-1.md` | Converted Docker Workflow sample with field-by-field mapping |
| `openapi-v3-schema-reference-claude-1.md` | Comprehensive reference covering all supported scenarios |
| `FINDINGS.md` | This consolidated summary |