## Meeting Notes: 09 Mar 2026

**Participants:** @Mirage20 @ChathurangaKCD @chalindukodikara

- The current structure doesn't feel natural — `openAPIV3Schema` wraps two independent JSON Schema documents (`parameters` and `environmentConfig`). The format key acts as a container rather than a schema property.
- Decision: Move `parameters` and `environmentConfig` to the top level (under `spec`), each independently choosing between `openAPIV3Schema` and `ocSchema`.
---

## Current Design

```yaml
spec:
  schema:
    openAPIV3Schema:
      parameters:
        type: object
        $defs:
          ResourceQuantity: ...
        properties:
          resources:
            $ref: "#/$defs/ResourceQuantity"
            default: {}

      environmentConfig:
        type: object
        $defs:
          ResourceQuantity: ...    # duplicated
        properties:
          resources:
            $ref: "#/$defs/ResourceQuantity"
            default: {}

    # --- OR (mutually exclusive) ---
    ocSchema:
      parameters: ...
      environmentConfig: ...
```

---

## New Design

`parameters` and `environmentConfig` become top-level fields. Each independently declares its schema format:

```yaml
spec:
  parameters:
    openAPIV3Schema:
      type: object
      properties: ...
      $defs: ...
    # --- OR ---
    ocSchema:
      $types:
        Resources:
          cpu: "string | default=100m"
          memory: "string | default=256Mi"
      replicas: "integer | default=1"
      resources: "Resources | default={}"

  environmentConfig:
    openAPIV3Schema:
      type: object
      properties:
        resources:
          $ref: "#/$defs/ResourceQuantity"
          default: {}
      $defs:
        ResourceQuantity: ...
    # --- OR ---
    ocSchema:
      resources: "Resources | default={}"
```
