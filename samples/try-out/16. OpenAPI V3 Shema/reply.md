If we go with this format, the overall structure would look like:

```yaml
schema:
  openAPIV3Schema:
    parameters:
      ...
    envOverrides:
      ...
  ocSchema:
    types:
      ...
    parameters:
      ...
    envOverrides:
      ...
```

### Key design question: One schema document or two?

Should we treat the entire `openAPIV3Schema` as a single JSON Schema document, or treat `parameters` and `envOverrides` as two separate JSON Schemas?

#### Option A: Single schema document

If we treat `openAPIV3Schema` as one JSON Schema document, we can use `$defs` + `$ref` for reusable type definitions (similar to `types` in ocSchema). But it forces us into a structure where `parameters` and `envOverrides` become `properties` of a root object — which is valid JSON Schema but looks awkward:

```yaml
openAPIV3Schema:
  type: object

  $defs:
    ResourceQuantity:
      type: object
      properties:
        cpu:
          type: string
          default: "100m"
        memory:
          type: string
          default: "128Mi"

    ResourceRequirements:
      type: object
      properties:
        requests:
          $ref: "#/$defs/ResourceQuantity"
          default: {}
        limits:
          $ref: "#/$defs/ResourceQuantity"
          default: {}

  properties:
    parameters:
      type: object
      additionalProperties: false
      properties:
        ....

    envOverrides:
      type: object
      additionalProperties: false
      properties:
        ....
```

This is **valid JSON Schema** (verified with `santhosh-tekuri/jsonschema/v6` using Draft 2020-12 — `$defs`, `$ref`, and `$ref` with sibling keywords like `default` all work correctly). However, it breaks the clean visual structure — `parameters` and `envOverrides` get buried inside `properties:`, and there's an extra `type: object` at the root that feels like unnecessary noise.

**Important note on `$ref` with siblings:** In JSON Schema **Draft 4–7** and **OpenAPI 3.0**, sibling keywords next to `$ref` (like `default: {}`) are **silently ignored**. This was fixed in **Draft 2019-09+**, where `$ref` is just another applicator and siblings are preserved. So if we go this route, we must use a Draft 2020-12 parser — not a Draft 4/7 or OpenAPI 3.0 parser.

#### Option B: Two separate schemas (recommended)

Treat `parameters` and `envOverrides` as independent JSON Schema documents:

```yaml
openAPIV3Schema:
  parameters:
    type: object
    additionalProperties: false
    $defs:
      ...
    properties:
      resources:
        $ref: "#/$defs/ResourceQuantity"
        default: {}
      ....

  envOverrides:
    type: object
    additionalProperties: false
    $defs:
      ...
    properties:
      resources:
        $ref: "#/$defs/ResourceQuantity"
        default: {}
      ....
```

This is cleaner and more intuitive — each section stands on its own. Users can still use `$defs` + `$ref` **within** each individual schema for type reuse. The only limitation compared to Option A is that `$defs` **cannot be shared across** `parameters` and `envOverrides` — if both need the same type (e.g., `ResourceQuantity`), the definition must be duplicated in each section. In practice this is a minor inconvenience since `parameters` and `envOverrides` rarely share the same types.

### Recommendation

Go with **Option B** (separate schemas). The cleaner structure is worth the trade-off. `$defs` + `$ref` still works within each schema for local type reuse, and cross-schema sharing is rarely needed.

If cross-schema type reuse becomes a strong requirement later, we can revisit by either:
- Treating `openAPIV3Schema` as a single document (Option A)
- Adding a pre-processing step that resolves `$defs`/`$ref` before passing to the validator