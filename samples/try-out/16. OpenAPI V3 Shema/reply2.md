If we support **both** schema formats, the CRD can expose something like:

```yaml
spec:
  schema:
    # New format (JSON Schema / OpenAPI-style)
    openAPIV3Schema:
      $defs: {}         # shared reusable definitions (JSON Schema $defs)
      parameters: {}    # JSON Schema root for "parameters"
      envOverrides: {}  # JSON Schema root for "envOverrides"

    # Current format (OpenChoreo / Kro Simple Schema)
    ocSchema:
      types: {}
      parameters: {}
      envOverrides: {}
```

The key question is what `openAPIV3Schema` **means**:

- Is it **one** JSON Schema document?
- Or is it a **bundle** that contains **two** JSON Schema documents (`parameters` and `envOverrides`) plus shared `$defs`?

## Option A — `openAPIV3Schema` is a single JSON Schema root

Make `openAPIV3Schema` a *proper* JSON Schema root (`type: object`) and model `parameters` + `envOverrides` as *properties*:

```yaml
openAPIV3Schema:
  type: object
  $defs:
    ResourceQuantity:
      type: object
      properties:
        cpu: {type: string, default: "100m"}
        memory: {type: string, default: "128Mi"}

  properties:
    parameters:
      type: object
      properties: {}
    envOverrides:
      type: object
      properties:
        resources:
          $ref: "#/$defs/ResourceQuantity"
          default: {}
```

Pros:
- The YAML is **literally a valid JSON Schema document** as-is.
- Any off-the-shelf JSON Schema validator can compile it directly.

Cons:
- You now validate a wrapper instance like `{parameters: {...}, envOverrides: {...}}`, even though OpenChoreo handles these as **two separate value bags**.

## Option B — `openAPIV3Schema` is a bundle (recommended for OpenChoreo)

Treat `openAPIV3Schema.parameters` and `openAPIV3Schema.envOverrides` as **two independent schema roots**, and keep `$defs` at the bundle root as a shared “types” mechanism:

```yaml
openAPIV3Schema:
  $defs:
    ResourceQuantity:
      type: object
      properties:
        cpu: {type: string, default: "100m"}
        memory: {type: string, default: "128Mi"}

  parameters:
    type: object
    properties: {}

  envOverrides:
    type: object
    properties:
      resources:
        $ref: "#/$defs/ResourceQuantity"
        default: {}
```

Important nuance (this answers the “`$defs` outside?” concern):

- **If you take `envOverrides` alone** and try to compile it as a standalone JSON Schema root, then yes — `#/$defs/...` won’t resolve because `$defs` isn’t inside that schema root.
- But if OpenChoreo defines `openAPIV3Schema` as a **bundle**, the controller can make it work in one of these ways:
  1. **Wrap at compile-time**: build a real JSON Schema root with `type: object`, put `$defs` at the root, and attach `parameters` + `envOverrides` under `properties` (like Option A) purely for compilation/validation.
  2. **Inject at compile-time**: copy/merge the bundle’s `$defs` into the root of the `parameters` schema and into the root of the `envOverrides` schema before compiling each one.

So `$defs` is not “outside the JSON Schema” in practice — it’s outside the *entrypoint subschema*, and the implementation must decide how to assemble a valid schema root before running a standard JSON Schema validator.
