# Research: OpenAPI V3 Schema Applicability for OpenChoreo

## 1. Is `openAPIV3Schema` the Same as JSON Schema?

**No, but they are closely related.** The relationship depends on the OpenAPI version:

### OpenAPI 3.0 (Current Kubernetes CRD Standard)

OpenAPI 3.0 uses a **modified subset of JSON Schema Draft-07**, often called the "subset-superset" problem:

- **Subset**: It removes some JSON Schema features (e.g., `$ref` to external files, `if/then/else`, `$defs`)
- **Superset**: It adds OpenAPI-specific keywords (e.g., `discriminator`, `readOnly`, `writeOnly`, `xml`, `externalDocs`, `example`, `deprecated`, `nullable`)

Key differences from JSON Schema Draft-07:
| Feature | JSON Schema Draft-07 | OpenAPI 3.0 Schema |
|---------|----------------------|-------------------|
| `type` as array | `type: ["string", "null"]` | Not allowed; use `nullable: true` instead |
| `$ref` | Allowed alongside other keywords | `$ref` replaces the entire schema object (no sibling keywords) |
| `if/then/else` | Supported | Not supported |
| `$defs` / `definitions` | Supported | Not supported (use `components/schemas` instead) |
| `examples` (array) | Supported | Use singular `example` keyword |
| `contentEncoding` | Supported | Not supported |
| `const` | Supported | Not supported |
| `exclusiveMinimum/Maximum` | Numeric value | Boolean value (modifies `minimum`/`maximum`) |

### OpenAPI 3.1 (Latest)

OpenAPI 3.1 achieves **full alignment with JSON Schema Draft 2020-12**:
- `type` can be an array (e.g., `type: ["string", "null"]`)
- `nullable` keyword removed (use type arrays instead)
- `$ref` can have sibling keywords
- `examples` array supported
- `const` supported
- `exclusiveMinimum/Maximum` are numeric values (not booleans)
- `$defs` supported natively
- `contentEncoding` and `contentMediaType` supported
- `if/then/else` supported

**Important**: Kubernetes currently uses OpenAPI 3.0-based schemas, NOT 3.1. This means OpenChoreo's `openAPIV3Schema` operates under the OpenAPI 3.0 subset rules.

### References
- JSON Schema Specification: https://json-schema.org/
- OpenAPI 3.0 Specification: https://swagger.io/specification/v3/
- OpenAPI 3.1 Specification: https://spec.openapis.org/oas/v3.1.0
- Migrating from OpenAPI 3.0 to 3.1: https://www.openapis.org/blog/2021/02/16/migrating-from-openapi-3-0-to-3-1-0
- OpenAPI v3.1 and JSON Schema: https://apisyouwonthate.com/blog/openapi-v3-1-and-json-schema/

---

## 2. How Does Kubernetes Use `openAPIV3Schema` in CRD Validation?

Kubernetes uses `openAPIV3Schema` as part of Custom Resource Definition (CRD) validation to enforce the shape and constraints of custom resources.

### Where It Lives

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: mycrs.example.com
spec:
  group: example.com
  versions:
    - name: v1
      schema:
        openAPIV3Schema:          # <-- Here
          type: object
          properties:
            spec:
              type: object
              properties:
                replicas:
                  type: integer
                  minimum: 1
                  maximum: 100
```

### What Kubernetes Does With It

1. **Server-side validation**: The API server validates every CREATE and UPDATE request against the schema. Invalid resources are rejected with descriptive error messages.
2. **Pruning (unknown field removal)**: Fields not defined in the schema are automatically stripped (unless `x-kubernetes-preserve-unknown-fields: true` is set).
3. **Defaulting**: `default` values in the schema are applied server-side when fields are omitted.
4. **Server-Side Apply**: The schema enables conflict detection for field ownership tracking.
5. **OpenAPI publishing**: CRD schemas are published to the cluster's OpenAPI endpoint (`/openapi/v2`), enabling `kubectl explain`, IDE autocompletion, and client-side validation.

### Structural Schema Requirement

Since Kubernetes 1.15 (required in apiextensions.k8s.io/v1), all CRD schemas must be **structural**. A structural schema must:

1. Specify a non-empty `type` at the root and for every object field and array item
2. Not use `$ref`, `$defs`, `definitions`, `dependencies`, or `$id`
3. For any field specified within `allOf`, `anyOf`, `oneOf`, or `not`, also specify that field at the top level of the schema (outside the logical junctor)
4. Not set `additionalProperties` at the root or on any sub-schema that also defines `properties`
5. Not use `description`, `type`, `default`, `additionalProperties`, `nullable` inside logical junctors

### References
- Kubernetes CRD Documentation: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/
- Future of CRDs: Structural Schemas: https://kubernetes.io/blog/2019/06/20/crd-structural-schema/
- Kubernetes Enhancement Proposal for Structural Schemas: https://github.com/kubernetes/enhancements/issues/2335

---

## 3. JSON Schema Feature Availability

### Available in OpenAPI 3.0 / Kubernetes CRDs

| Feature | Supported | Notes |
|---------|-----------|-------|
| `type` | Yes | Must be non-empty for structural schemas |
| `properties` | Yes | Defines object fields |
| `required` | Yes | Array of required field names |
| `additionalProperties` | Yes | Boolean or schema; used for map types |
| `items` | Yes | Schema for array items |
| `enum` | Yes | Allowed values |
| `default` | Yes | Default values (applied server-side) |
| `minimum` / `maximum` | Yes | Numeric constraints |
| `exclusiveMinimum` / `exclusiveMaximum` | Yes | Boolean in OpenAPI 3.0 (modifies min/max) |
| `minLength` / `maxLength` | Yes | String length constraints |
| `pattern` | Yes | Regex pattern for strings |
| `minItems` / `maxItems` | Yes | Array length constraints |
| `minProperties` / `maxProperties` | Yes | Object property count constraints |
| `multipleOf` | Yes | Numeric multiple constraint |
| `allOf` | Yes | Must satisfy all schemas (with structural schema rules) |
| `oneOf` | Yes | Must satisfy exactly one schema (with structural schema rules) |
| `anyOf` | Yes | Must satisfy at least one schema (with structural schema rules) |
| `not` | Yes | Must NOT satisfy the schema (with structural schema rules) |
| `title` | Yes | Human-readable title |
| `description` | Yes | Human-readable description |
| `format` | Yes | Hints for string formats (e.g., `date-time`, `email`) |
| `nullable` | Yes | OpenAPI 3.0 extension to allow null values |
| `x-kubernetes-*` | Yes | Kubernetes-specific extensions (see below) |

### Kubernetes-Specific Extensions

| Extension | Purpose |
|-----------|---------|
| `x-kubernetes-preserve-unknown-fields` | Opt out of pruning; allow arbitrary fields |
| `x-kubernetes-int-or-string` | Field can be either integer or string |
| `x-kubernetes-embedded-resource` | Field is an embedded Kubernetes resource (has apiVersion, kind, metadata) |
| `x-kubernetes-validations` | CEL-based validation rules |
| `x-kubernetes-list-type` | List semantics: `atomic`, `set`, or `map` |
| `x-kubernetes-list-map-keys` | Keys for map-type lists |
| `x-kubernetes-map-type` | Map semantics: `atomic` or `granular` |

### NOT Supported in Kubernetes CRDs

| Feature | Status | Reason |
|---------|--------|--------|
| `$ref` | **Not supported** | Structural schema requirement; must inline everything |
| `$defs` / `definitions` | **Not supported** | No local reference resolution |
| `$id` | **Not supported** | No schema identification |
| `$schema` | **Not supported** | Fixed to OpenAPI 3.0 dialect |
| `dependencies` | **Not supported** | Not part of structural schemas |
| `if / then / else` | **Not supported** | Not available in OpenAPI 3.0 |
| `const` | **Not supported** | Not available in OpenAPI 3.0 (use `enum` with single value) |
| `contentEncoding` | **Not supported** | Not available in OpenAPI 3.0 |
| `contentMediaType` | **Not supported** | Not available in OpenAPI 3.0 |
| `patternProperties` | **Not supported** | Not available in OpenAPI 3.0 CRDs |
| `propertyNames` | **Not supported** | Not available in structural schemas |

### Why These Features Are Excluded

Kubernetes chose to restrict JSON Schema features for several reasons:

1. **Performance**: `$ref` resolution and complex schema composition add overhead to API server validation
2. **Simplicity**: Structural schemas ensure every field path can be statically analyzed, enabling pruning, defaulting, and server-side apply
3. **Security**: Features like `$ref` with external URIs could introduce security risks
4. **Determinism**: Restrictions ensure validation behavior is predictable and consistent across all API server instances

### References
- Kubernetes CRD Validation: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation
- $ref not supported: https://github.com/kubernetes/kubernetes/issues/62872
- $ref feature request: https://github.com/kubernetes/kubernetes/issues/91669
- Structural schema blog post: https://kubernetes.io/blog/2019/06/20/crd-structural-schema/

---

## 4. Implications for OpenChoreo

### Key Takeaway

OpenChoreo's `openAPIV3Schema` should follow the **Kubernetes structural schema subset** of OpenAPI 3.0, since:

1. The schemas will ultimately be used within a Kubernetes context
2. This ensures compatibility if schemas need to be embedded in CRDs
3. Kubernetes tooling (kubectl, IDE plugins) can work with these schemas

### What This Means in Practice

- **No `$ref`/`$defs`**: Reusable types must be resolved (inlined) before the schema is used by Kubernetes. OpenChoreo can support `$ref`/`$defs` at the authoring level and resolve them internally.
- **Use `x-` extensions**: Custom annotations map cleanly to `x-openchoreo-*` vendor extensions
- **Type must always be specified**: Every field needs an explicit `type`
- **`allOf`/`oneOf`/`anyOf` with care**: Available but fields must be duplicated outside the logical junctor
- **`additionalProperties` explicitly**: To match Simple Schema's evolution-friendly behavior, set `additionalProperties: true` or omit it (Kubernetes defaults to pruning unknown fields)

### Comparison: How Other Projects Handle This

| Project | Schema Approach |
|---------|----------------|
| **Crossplane** | Uses OpenAPI V3 Schema in Compositions; resolves `$ref` during compilation |
| **Helm** | Uses JSON Schema for `values.schema.json`; full JSON Schema support (not Kubernetes-validated) |
| **Backstage** | Uses JSON Schema for template parameters with `ui:*` extensions |
| **Kro** | Custom "Simple Schema" that compiles to JSON Schema internally |
