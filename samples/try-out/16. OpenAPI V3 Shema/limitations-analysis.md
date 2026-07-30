# Limitations Analysis: OpenAPI V3 Schema Adoption for OpenChoreo

## 1. Custom Types (`schema.types`) and Type Reuse

### The Problem

Simple Schema supports reusable named types:
```yaml
types:
  ResourceRequirements:
    requests: "ResourceQuantity | default={}"
    limits: "ResourceQuantity | default={}"
  ResourceQuantity:
    cpu: "string | default=100m"
    memory: "string | default=256Mi"

parameters:
  resources: "ResourceRequirements | default={}"
```

JSON Schema has `$defs`/`$ref` for this purpose:
```yaml
$defs:
  ResourceQuantity:
    type: object
    properties:
      cpu: { type: string, default: "100m" }
      memory: { type: string, default: "256Mi" }
  ResourceRequirements:
    type: object
    properties:
      requests: { $ref: "#/$defs/ResourceQuantity", default: {} }
      limits: { $ref: "#/$defs/ResourceQuantity", default: {} }
```

**However, Kubernetes CRD validation does NOT support `$ref` or `$defs`.** This is a long-standing limitation (see [kubernetes/kubernetes#62872](https://github.com/kubernetes/kubernetes/issues/62872) and [#91669](https://github.com/kubernetes/kubernetes/issues/91669)).

### Options for OpenChoreo

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **A. Inline everything** | Duplicate type definitions at every usage site | K8s-compatible; no resolution step | Extremely verbose; error-prone; hard to maintain |
| **B. Support `$defs`/`$ref` and resolve internally** | Allow authors to use `$defs`/`$ref` in `openAPIV3Schema`; OpenChoreo resolves (inlines) them before validation/use | Familiar JSON Schema DX; type reuse preserved; clean authoring experience | Requires a resolution step in the controller; resolved schema differs from authored schema |
| **C. Keep `types` as a separate OpenChoreo concept** | Support a `types` section alongside `openAPIV3Schema` that gets merged in | Backward-compatible with Simple Schema mental model | Non-standard; confusing for users expecting pure JSON Schema |

### Recommendation

**Option B** is the best path forward. It aligns with industry practice (Crossplane and Helm both resolve `$ref` before use) and gives platform engineers the reuse they need while remaining JSON Schema-compliant at the authoring level.

Implementation note: The `$ref` resolution must handle:
- Circular references (error or depth limit)
- Default merging (a `default` adjacent to a `$ref` should override the referenced schema's default)
- Nested `$ref` chains (type A references type B references type C)

---

## 2. Verbosity

### The Problem

The same schema is significantly more verbose in OpenAPI V3 format:

**Simple Schema (5 lines):**
```yaml
parameters:
  autoscaling:
    enabled: "boolean | default=false"
    minReplicas: "integer | default=2 | minimum=1"
    maxReplicas: "integer | default=10 | minimum=1"
```

**OpenAPI V3 Schema (18 lines):**
```yaml
parameters:
  type: object
  properties:
    autoscaling:
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
```

This is a **~3.5x increase** in line count. For complex ComponentTypes with many fields, types, and both parameters and envOverrides, this can make schemas unwieldy.

### Impact

- **Platform Engineer DX**: PEs author schemas. Increased verbosity means more typing, more scrolling, and more room for error.
- **Code review**: Larger diffs are harder to review.
- **File size**: ComponentType YAML files could become quite large.

### Mitigation

- **Keep Simple Schema as the "fast" option**: Since both formats will be supported, PEs can use Simple Schema for rapid development and OpenAPI V3 when they need advanced features.
- **IDE support**: OpenAPI V3 Schema has excellent IDE support (autocompletion, validation), which partially offsets verbosity.
- **`$defs`/`$ref` for type reuse**: Reduces duplication significantly in schemas with repeated structures.
- **Tooling**: Consider providing a CLI tool that converts Simple Schema to OpenAPI V3 Schema (and vice versa) for migration and inspection.

---

## 3. Default Handling

### The Problem

Simple Schema has specific `$default` semantics for objects:

```yaml
monitoring:
  $default: {}
  enabled: "boolean | default=false"
  port: "integer | default=9090"
```

This means: "If the `monitoring` object is not provided at all, use `{}` as the default, then apply field-level defaults to produce `{enabled: false, port: 9090}`."

### JSON Schema `default` Behavior

JSON Schema's `default` keyword works similarly:
```yaml
monitoring:
  type: object
  default: {}
  properties:
    enabled:
      type: boolean
      default: false
    port:
      type: integer
      default: 9090
```

When `monitoring` is omitted:
1. The object `default: {}` is applied, creating an empty `monitoring` object
2. Kubernetes server-side defaulting then applies field-level defaults within the object

### Behavioral Differences

| Aspect | Simple Schema | OpenAPI V3 Schema |
|--------|---------------|-------------------|
| Object default syntax | `$default: {}` or `default={}` on type reference | `default: {}` on the schema node |
| Default cascading | Explicit: `$default` + field defaults compose | Same: object default + field defaults compose (Kubernetes applies defaults recursively) |
| Type-level defaults | `$default` in type definition makes all references optional | `default` in `$defs` definition; but `$ref` override semantics vary by implementation |
| Non-empty object default | `$default: {host: "localhost"}` | `default: {host: "localhost"}` |

### Key Consideration

Kubernetes applies defaults **during admission** (on CREATE/UPDATE). This means:
- The stored resource will always have defaults filled in
- This matches Simple Schema's behavior where defaults are applied at admission time
- **Nested defaulting works**: If an object's default is `{}`, Kubernetes will then apply defaults for the object's properties

### Conclusion

The mapping is clean. `$default: {}` maps directly to `default: {}`. The only nuance is that Simple Schema's `$default` is a special key that doesn't appear as a field, while in JSON Schema `default` is a standard keyword. No behavioral mismatch.

---

## 4. `additionalProperties` and Schema Evolution

### The Problem

Simple Schema explicitly allows additional properties for schema evolution:
> "OpenChoreo schemas allow additional properties beyond what's defined (no `additionalProperties: false`)"

In Kubernetes CRD structural schemas:
- **Pruning is enabled by default**: Unknown fields are automatically stripped unless `x-kubernetes-preserve-unknown-fields: true` is set
- If `additionalProperties` is not specified on an object, Kubernetes defaults to pruning unknown fields
- Setting `additionalProperties: true` disables pruning for that object

### Impact on OpenChoreo

Since OpenChoreo schemas are NOT directly Kubernetes CRD schemas (they are embedded within OpenChoreo's own CRDs as opaque data), this is nuanced:

1. **If schemas are stored as opaque strings/objects**: OpenChoreo validates using its own controller logic. The controller can choose to not prune, matching Simple Schema behavior.
2. **If schemas are part of a CRD's openAPIV3Schema**: Kubernetes would enforce pruning. You'd need `x-kubernetes-preserve-unknown-fields: true` or `additionalProperties: true` to preserve Simple Schema's behavior.

### Recommendation

For consistency with Simple Schema's evolution-friendly behavior:
- When converting, explicitly set `additionalProperties: true` on objects that should allow unknown fields
- OR document that OpenChoreo's validation layer does not prune unknown fields (matching current behavior)
- Consider providing a per-schema option: `x-openchoreo-strict-mode: true` to opt into strict validation (no extra properties)

---

## 5. Custom Annotations Mapping

### The Problem

Simple Schema uses `oc:` prefix for custom metadata:
```yaml
commitHash: "string | oc:build:inject=git.sha oc:ui:hidden=true"
```

OpenAPI V3 uses `x-` prefixed vendor extensions:
```yaml
commitHash:
  type: string
  x-openchoreo-build-inject: git.sha
  x-openchoreo-ui-hidden: true
```

### Is This a Clean Mapping?

**Mostly yes**, but with considerations:

| Aspect | `oc:` annotations | `x-openchoreo-*` extensions |
|--------|-------------------|----------------------------|
| Format | Key-value in constraint string | Standard YAML keys with any JSON value |
| Nesting | Flat: `oc:ui:hidden=true` | Can be nested: `x-openchoreo-ui: {hidden: true}` |
| Value types | Strings only (from constraint parser) | Any JSON type (strings, booleans, objects, arrays) |
| Validation | Ignored by schema validation | Preserved by OpenAPI tooling; ignored by JSON Schema validation |
| Tooling support | OpenChoreo-specific | Standard OpenAPI extension pattern; tools know to preserve `x-*` keys |

### Advantages of `x-openchoreo-*`

1. **Richer values**: Can use boolean, numeric, object, and array values instead of just strings
2. **Grouping**: Can group related annotations under a single extension key:
   ```yaml
   x-openchoreo-backstage-portal:
     ui:field: RepoUrlPicker
     ui:options:
       allowedHosts: ["github.com"]
   ```
3. **Tooling**: OpenAPI validators and generators preserve `x-*` keys automatically
4. **Interoperability**: Backstage, ArgoCD, and other tools already use `x-*` extensions

### Migration Path

The mapping is straightforward:
- `oc:build:inject=git.sha` -> `x-openchoreo-build-inject: git.sha`
- `oc:ui:hidden=true` -> `x-openchoreo-ui-hidden: true`
- `oc:scaffolding=omit` -> `x-openchoreo-scaffolding: omit`

Consider also supporting grouped forms:
- `x-openchoreo: {build: {inject: git.sha}, ui: {hidden: true}}`

---

## 6. Validation at the API Server Level

### The Problem

If OpenAPI V3 schemas are embedded in OpenChoreo's CRDs, does Kubernetes validate the schema itself? Or does OpenChoreo need its own validation?

### How It Works

OpenChoreo's CRDs (ComponentType, Trait, Workflow) store schemas as **data within their spec**. There are two layers:

1. **CRD-level validation**: Kubernetes validates that the ComponentType/Trait/Workflow resource conforms to its CRD schema. The `spec.schema.openAPIV3Schema` field would be typed as an opaque object (e.g., `type: object, x-kubernetes-preserve-unknown-fields: true`) in the CRD definition.

2. **OpenChoreo controller validation**: When a Component or WorkflowRun is created, OpenChoreo's controller must:
   - Parse the schema from the referenced ComponentType/Trait/Workflow
   - Validate the developer-provided `parameters` and `envOverrides` against that schema
   - Apply defaults

### What OpenChoreo Needs to Build

| Validation Step | Who Does It |
|-----------------|-------------|
| Is the ComponentType YAML valid? | Kubernetes CRD validation |
| Is `spec.schema.openAPIV3Schema` a valid OpenAPI V3 schema? | **OpenChoreo controller** (Kubernetes treats it as opaque) |
| Do Component `parameters` match the schema? | **OpenChoreo controller** |
| Do ReleaseBinding `envOverrides` match the schema? | **OpenChoreo controller** |
| Are defaults applied correctly? | **OpenChoreo controller** |

### Implementation

OpenChoreo should use a JSON Schema validation library (e.g., Go's `santhosh-tekuri/jsonschema` or `xeipuv/gojsonschema`) to:
1. Validate that the schema itself is well-formed
2. Validate user-provided values against the schema
3. Apply defaults (using Kubernetes-style defaulting or custom logic)

This is the same approach used by Crossplane (validates Composition schemas in-controller) and Backstage (validates template parameters client-side).

---

## 7. CEL Templating Interaction

### The Problem

OpenChoreo schemas feed into CEL templates:
```yaml
# In ComponentType template:
replicas: ${envOverrides.replicas}
minReplicas: ${parameters.autoscaling.minReplicas}
```

Does the schema format (Simple Schema vs OpenAPI V3) affect template resolution?

### Analysis

**No, the schema format does not affect CEL template resolution.** Here's why:

1. **Both formats compile to the same data structure**: Whether defined via Simple Schema or OpenAPI V3 Schema, the user-provided values are the same JSON/YAML structure:
   ```yaml
   parameters:
     autoscaling:
       enabled: true
       minReplicas: 2
       maxReplicas: 10
   ```

2. **CEL operates on values, not schemas**: CEL expressions access the resolved parameter/envOverride values. The schema only determines:
   - What values are valid (validation)
   - What defaults are applied (defaulting)
   - The schema format doesn't change the value structure

3. **Type consistency**: Both schema formats enforce the same types (integer, string, boolean, etc.), so CEL expressions see the same Go types regardless of schema format.

### One Subtle Consideration

If OpenChoreo resolves defaults differently between Simple Schema and OpenAPI V3 Schema, CEL templates could see different values for the same input. For example:

- Simple Schema applies `$default` cascading with specific precedence rules
- OpenAPI V3 Schema's `default` keyword has standard JSON Schema semantics

To avoid issues:
- Ensure the defaulting logic is identical for both schema formats
- Write integration tests that verify the same user input produces the same resolved values regardless of schema format

### Conclusion

The schema format is transparent to CEL templates. As long as defaulting behavior is consistent between formats, switching schemas is invisible to the template layer.

---

## Summary of Challenges and Recommendations

| Challenge | Severity | Recommendation |
|-----------|----------|----------------|
| No `$ref`/`$defs` in K8s CRDs | **High** | Support `$ref`/`$defs` at authoring time; resolve internally before use |
| Verbosity (~3.5x more lines) | **Medium** | Keep Simple Schema as an option; provide conversion tooling |
| Default handling differences | **Low** | Direct mapping exists; ensure defaulting logic is consistent |
| `additionalProperties` behavior | **Medium** | Explicitly document OpenChoreo's behavior; consider `additionalProperties: true` by default |
| Custom annotation migration | **Low** | Clean mapping from `oc:*` to `x-openchoreo-*`; richer value types as a bonus |
| Schema validation in controller | **Medium** | Implement JSON Schema validation in OpenChoreo controller (required regardless) |
| CEL template interaction | **Low** | No impact; transparent to templates if defaulting is consistent |
