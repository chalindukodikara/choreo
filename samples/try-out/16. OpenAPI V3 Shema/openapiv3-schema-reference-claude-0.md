# OpenAPI v3 Schema Reference for OpenChoreo

This document provides a comprehensive reference for all schema scenarios currently supported by OpenChoreo's simple schema, mapped to their OpenAPI v3 Schema equivalents.

## Table of Contents

- [Schema Structure](#schema-structure)
- [Primitive Types](#primitive-types)
- [String Constraints](#string-constraints)
- [Numeric Constraints](#numeric-constraints)
- [Boolean Type](#boolean-type)
- [Objects (Inline)](#objects-inline)
- [Nested Objects](#nested-objects)
- [Arrays](#arrays)
- [Maps](#maps)
- [Default Values](#default-values)
- [Required vs Optional Fields](#required-vs-optional-fields)
- [Custom Types (Reusable Definitions)](#custom-types-reusable-definitions)
- [Enum Values](#enum-values)
- [Documentation Markers](#documentation-markers)
- [Custom Annotations / Vendor Extensions](#custom-annotations--vendor-extensions)
- [Backstage Portal Extensions](#backstage-portal-extensions)
- [Complete Reference YAML](#complete-reference-yaml)

---

## Schema Structure

The top-level structure nests schemas under `schema.openAPIV3Schema`:

```yaml
spec:
  schema:
    openAPIV3Schema:
      parameters:
        type: object
        properties:
          # ... parameter definitions ...

      envOverrides:
        type: object
        properties:
          # ... environment override definitions ...
```

This mirrors the Kubernetes CRD pattern where `spec.versions[].schema.openAPIV3Schema` wraps the validation schema.

---

## Primitive Types

### Simple Schema
```yaml
name: string
age: integer
price: number
enabled: boolean
```

### OpenAPI v3 Schema
```yaml
properties:
  name:
    type: string
  age:
    type: integer
  price:
    type: number
  enabled:
    type: boolean
```

Supported types: `string`, `integer`, `number`, `boolean`, `object`, `array`

---

## String Constraints

### Simple Schema
```yaml
username: "string | minLength=3 maxLength=20 pattern=^[a-z][a-z0-9_]*$"
environment: "string | enum=development,staging,production"
apiKey: "string | title='API Key' description='Auth key' example=sk-abc123"
```

### OpenAPI v3 Schema
```yaml
properties:
  username:
    type: string
    minLength: 3
    maxLength: 20
    pattern: "^[a-z][a-z0-9_]*$"
  environment:
    type: string
    enum:
      - development
      - staging
      - production
  apiKey:
    type: string
    title: "API Key"
    description: "Auth key"
    example: "sk-abc123"
```

---

## Numeric Constraints

### Simple Schema
```yaml
age: "integer | minimum=0 maximum=150"
price: "number | minimum=0 exclusiveMinimum=true multipleOf=0.01"
temperature: "number | maximum=100 exclusiveMaximum=true"
statusCode: "integer | enum=200,201,204,400,404,500"
replicas: "integer | default=1 minimum=1 maximum=100"
```

### OpenAPI v3 Schema
```yaml
properties:
  age:
    type: integer
    minimum: 0
    maximum: 150
  price:
    type: number
    minimum: 0
    exclusiveMinimum: true
    multipleOf: 0.01
  temperature:
    type: number
    maximum: 100
    exclusiveMaximum: true
  statusCode:
    type: integer
    enum: [200, 201, 204, 400, 404, 500]
  replicas:
    type: integer
    default: 1
    minimum: 1
    maximum: 100
```

**Note:** In OpenAPI v3.0, `exclusiveMinimum` and `exclusiveMaximum` are booleans (modifying `minimum`/`maximum`). In JSON Schema Draft 2020-12, they are numeric values. The K8s CRD subset uses the boolean form.

---

## Boolean Type

### Simple Schema
```yaml
enabled: "boolean | default=false"
debug: boolean
```

### OpenAPI v3 Schema
```yaml
properties:
  enabled:
    type: boolean
    default: false
  debug:
    type: boolean
```

---

## Objects (Inline)

### Simple Schema
```yaml
database:
  host: string
  port: "integer | default=5432"
  username: string
```

### OpenAPI v3 Schema
```yaml
properties:
  database:
    type: object
    required:
      - host
      - username
    properties:
      host:
        type: string
      port:
        type: integer
        default: 5432
      username:
        type: string
```

---

## Nested Objects

### Simple Schema
```yaml
database:
  host: string
  port: "integer | default=5432"
  options:
    ssl: "boolean | default=true"
    timeout: "integer | default=30"
    pool:
      minSize: "integer | default=5"
      maxSize: "integer | default=20"
```

### OpenAPI v3 Schema
```yaml
properties:
  database:
    type: object
    required:
      - host
    properties:
      host:
        type: string
      port:
        type: integer
        default: 5432
      options:
        type: object
        default: {}
        properties:
          ssl:
            type: boolean
            default: true
          timeout:
            type: integer
            default: 30
          pool:
            type: object
            default: {}
            properties:
              minSize:
                type: integer
                default: 5
              maxSize:
                type: integer
                default: 20
```

---

## Arrays

### Simple Schema
```yaml
tags: "[]string"
ports: "[]integer | minItems=1 maxItems=10"
mounts: "[]MountConfig"
configs: "[]map<string>"
optionalTags: "[]string | default=[]"
```

### OpenAPI v3 Schema
```yaml
properties:
  tags:
    type: array
    items:
      type: string
  ports:
    type: array
    items:
      type: integer
    minItems: 1
    maxItems: 10
  mounts:
    type: array
    items:
      type: object
      required:
        - path
      properties:
        path:
          type: string
        subPath:
          type: string
          default: ""
        readOnly:
          type: boolean
          default: false
  configs:
    type: array
    items:
      type: object
      additionalProperties:
        type: string
  optionalTags:
    type: array
    default: []
    items:
      type: string
```

---

## Maps

### Simple Schema
```yaml
labels: "map<string>"
ports: "map<integer>"
settings: "map<boolean>"
metadata: "map<string> | minProperties=1 maxProperties=10"
optionalLabels: "map<string> | default={}"
```

### OpenAPI v3 Schema
```yaml
properties:
  labels:
    type: object
    additionalProperties:
      type: string
  ports:
    type: object
    additionalProperties:
      type: integer
  settings:
    type: object
    additionalProperties:
      type: boolean
  metadata:
    type: object
    additionalProperties:
      type: string
    minProperties: 1
    maxProperties: 10
  optionalLabels:
    type: object
    default: {}
    additionalProperties:
      type: string
```

---

## Default Values

### Primitive Defaults

```yaml
# Simple Schema
replicas: "integer | default=1"
name: "string | default=myapp"
enabled: "boolean | default=false"

# OpenAPI v3 Schema
properties:
  replicas:
    type: integer
    default: 1
  name:
    type: string
    default: myapp
  enabled:
    type: boolean
    default: false
```

### Object Defaults ($default equivalent)

#### Simple Schema
```yaml
monitoring:
  $default: {}
  enabled: "boolean | default=false"
  port: "integer | default=9090"

database:
  $default:
    host: "localhost"
  host: string
  port: "integer | default=5432"
```

#### OpenAPI v3 Schema
```yaml
properties:
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
  database:
    type: object
    default:
      host: "localhost"
    required:
      - host
    properties:
      host:
        type: string
      port:
        type: integer
        default: 5432
```

### Cascading Object Defaults

#### Simple Schema
```yaml
resources:
  $default: {}
  requests:
    $default: {}
    cpu: "string | default=100m"
    memory: "string | default=256Mi"
  limits:
    $default: {}
    cpu: "string | default=1000m"
    memory: "string | default=1Gi"
```

#### OpenAPI v3 Schema
```yaml
properties:
  resources:
    type: object
    default: {}
    properties:
      requests:
        type: object
        default: {}
        properties:
          cpu:
            type: string
            default: "100m"
          memory:
            type: string
            default: "256Mi"
      limits:
        type: object
        default: {}
        properties:
          cpu:
            type: string
            default: "1000m"
          memory:
            type: string
            default: "1Gi"
```

---

## Required vs Optional Fields

In the simple schema, all fields without `default` are required. In OpenAPI v3 Schema, required fields must be explicitly listed.

### Simple Schema (implicit required)
```yaml
parameters:
  name: string                    # required (no default)
  port: "integer | default=8080"  # optional (has default)
  host: string                    # required (no default)
```

### OpenAPI v3 Schema (explicit required)
```yaml
parameters:
  type: object
  required:
    - name
    - host
  properties:
    name:
      type: string
    port:
      type: integer
      default: 8080
    host:
      type: string
```

**Convention for OpenChoreo:** Properties without `default` that are not in `required` are treated as optional. To maintain the current "required by default" behavior, OpenChoreo could auto-populate `required` for properties without defaults. This should be a documented convention.

---

## Custom Types (Reusable Definitions)

The simple schema supports reusable types via `types:`. OpenAPI v3 Schema (K8s CRD subset) does not support `$ref`. Options:

### Option A: Inline (duplication)

```yaml
# Simple Schema with types
schema:
  types:
    ResourceQuantity:
      cpu: "string | default=100m"
      memory: "string | default=256Mi"
    ResourceRequirements:
      requests: "ResourceQuantity | default={}"
      limits: "ResourceQuantity | default={}"
  envOverrides:
    resources: "ResourceRequirements | default={}"

# OpenAPI v3 Schema - inlined
schema:
  openAPIV3Schema:
    envOverrides:
      type: object
      properties:
        resources:
          type: object
          default: {}
          properties:
            requests:
              type: object
              default: {}
              properties:
                cpu:
                  type: string
                  default: "100m"
                memory:
                  type: string
                  default: "256Mi"
            limits:
              type: object
              default: {}
              properties:
                cpu:
                  type: string
                  default: "100m"
                memory:
                  type: string
                  default: "256Mi"
```

### Option B: x-openchoreo-types extension (recommended)

```yaml
schema:
  openAPIV3Schema:
    x-openchoreo-types:
      ResourceQuantity:
        type: object
        default: {}
        properties:
          cpu:
            type: string
            default: "100m"
          memory:
            type: string
            default: "256Mi"
      ResourceRequirements:
        type: object
        default: {}
        properties:
          requests:
            x-openchoreo-type-ref: ResourceQuantity
          limits:
            x-openchoreo-type-ref: ResourceQuantity

    envOverrides:
      type: object
      properties:
        resources:
          x-openchoreo-type-ref: ResourceRequirements
```

OpenChoreo resolves `x-openchoreo-type-ref` at parse time, expanding type references inline before validation. This preserves reuse without depending on `$ref`.

---

## Enum Values

### Simple Schema
```yaml
imagePullPolicy: 'string | enum="Always, IfNotPresent, Never" default="IfNotPresent"'
environment: "string | enum=development,staging,production"
statusCode: "integer | enum=200,201,204,400,404,500"
```

### OpenAPI v3 Schema
```yaml
properties:
  imagePullPolicy:
    type: string
    default: IfNotPresent
    enum:
      - Always
      - IfNotPresent
      - Never
  environment:
    type: string
    enum:
      - development
      - staging
      - production
  statusCode:
    type: integer
    enum: [200, 201, 204, 400, 404, 500]
```

---

## Documentation Markers

### Simple Schema
```yaml
apiKey: "string | title='API Key' description='Authentication key' example=sk-abc123"
timeout: "integer | description='Request timeout in seconds' default=30"
```

### OpenAPI v3 Schema
```yaml
properties:
  apiKey:
    type: string
    title: "API Key"
    description: "Authentication key"
    example: "sk-abc123"
  timeout:
    type: integer
    description: "Request timeout in seconds"
    default: 30
```

---

## Custom Annotations / Vendor Extensions

### Simple Schema (oc: prefix)
```yaml
commitHash: "string | oc:build:inject=git.sha oc:ui:hidden=true"
advancedTimeout: "string | default='30s' oc:scaffolding=omit"
```

### OpenAPI v3 Schema (x- prefix)
```yaml
properties:
  commitHash:
    type: string
    x-openchoreo-build-inject: git.sha
    x-openchoreo-ui-hidden: true
  advancedTimeout:
    type: string
    default: "30s"
    x-openchoreo-scaffolding: omit
```

The `x-` extension mechanism is a first-class feature of OpenAPI. Extensions are preserved through validation and can be read by any tool that processes the schema.

---

## Backstage Portal Extensions

OpenAPI vendor extensions provide a clean way to attach UI metadata for Backstage template generation.

```yaml
properties:
  repository:
    type: object
    properties:
      url:
        type: string
        description: "Git repository URL"
        x-openchoreo-backstage-portal:
          ui:field: RepoUrlPicker
          ui:options:
            allowedHosts:
              - github.com
      secretRef:
        type: string
        description: "Secret reference name for private repository Git credentials"
        x-openchoreo-backstage-portal:
          ui:field: SecretPicker
  environment:
    type: string
    enum:
      - development
      - staging
      - production
    x-openchoreo-backstage-portal:
      ui:widget: radio
```

---

## Complete Reference YAML

Below is a single comprehensive YAML file demonstrating all supported scenarios using openAPIV3Schema.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ComponentType
metadata:
  name: openapiv3-reference-example
  namespace: default
spec:
  workloadType: deployment

  schema:
    openAPIV3Schema:

      # ============================================================
      # PARAMETERS - Static configuration provided at component creation
      # ============================================================
      parameters:
        type: object
        required:
          - name
          - repository
        properties:

          # --- Primitive Types ---
          name:
            type: string
            description: "Application name"
            title: "Name"
            minLength: 1
            maxLength: 63
            pattern: "^[a-z][a-z0-9-]*$"

          port:
            type: integer
            default: 8080
            minimum: 1
            maximum: 65535
            description: "Application port"

          cpuWeight:
            type: number
            default: 1.0
            minimum: 0.1
            maximum: 10.0
            multipleOf: 0.1
            description: "CPU scheduling weight"

          enabled:
            type: boolean
            default: true
            description: "Whether the application is enabled"

          # --- Enum ---
          tier:
            type: string
            default: standard
            enum:
              - basic
              - standard
              - premium
            description: "Service tier"

          statusCode:
            type: integer
            enum: [200, 201, 204, 400, 404, 500]
            default: 200

          # --- Nested Object (inline) ---
          repository:
            type: object
            required:
              - url
            properties:
              url:
                type: string
                description: "Git repository URL"
                x-openchoreo-backstage-portal:
                  ui:field: RepoUrlPicker
                  ui:options:
                    allowedHosts:
                      - github.com
              secretRef:
                type: string
                description: "Secret reference for Git credentials"
              revision:
                type: object
                default: {}
                properties:
                  branch:
                    type: string
                    default: main
                    description: "Git branch"
                  commit:
                    type: string
                    description: "Git commit SHA"
              appPath:
                type: string
                default: "."
                description: "Path to application directory"

          # --- Deeply Nested Object ---
          autoscaling:
            type: object
            default: {}
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
                maximum: 100

          # --- Array of Strings ---
          tags:
            type: array
            default: []
            items:
              type: string
            minItems: 0
            maxItems: 10
            description: "Tags for the application"

          # --- Array of Integers ---
          exposedPorts:
            type: array
            items:
              type: integer
              minimum: 1
              maximum: 65535
            minItems: 1

          # --- Array of Objects ---
          volumes:
            type: array
            default: []
            items:
              type: object
              required:
                - name
                - mountPath
              properties:
                name:
                  type: string
                mountPath:
                  type: string
                readOnly:
                  type: boolean
                  default: false
                subPath:
                  type: string
                  default: ""

          # --- Map with String Values ---
          labels:
            type: object
            default: {}
            additionalProperties:
              type: string
            description: "Custom labels"
            minProperties: 0
            maxProperties: 20

          # --- Map with Integer Values ---
          portMapping:
            type: object
            default: {}
            additionalProperties:
              type: integer

          # --- Map with Boolean Values ---
          featureFlags:
            type: object
            default: {}
            additionalProperties:
              type: boolean

          # --- Custom Annotations (x- extensions) ---
          buildConfig:
            type: object
            default: {}
            properties:
              commitHash:
                type: string
                x-openchoreo-build-inject: git.sha
                x-openchoreo-ui-hidden: true
              dockerfilePath:
                type: string
                default: "./Dockerfile"
                x-openchoreo-scaffolding: omit

      # ============================================================
      # ENVIRONMENT OVERRIDES - Per-environment configuration
      # ============================================================
      envOverrides:
        type: object
        default: {}
        properties:

          # --- Simple overrides ---
          replicas:
            type: integer
            default: 1
            minimum: 1
            maximum: 100

          imagePullPolicy:
            type: string
            default: IfNotPresent
            enum:
              - Always
              - IfNotPresent
              - Never

          # --- Nested object override (resource requirements) ---
          resources:
            type: object
            default: {}
            description: "Container resource requirements"
            properties:
              requests:
                type: object
                default: {}
                properties:
                  cpu:
                    type: string
                    default: "100m"
                  memory:
                    type: string
                    default: "256Mi"
              limits:
                type: object
                default: {}
                properties:
                  cpu:
                    type: string
                    default: "1000m"
                  memory:
                    type: string
                    default: "1Gi"

          # --- Autoscaling env-specific overrides ---
          autoscaling:
            type: object
            default: {}
            properties:
              minReplicas:
                type: integer
                minimum: 1
              maxReplicas:
                type: integer
                minimum: 1

          # --- Map override ---
          annotations:
            type: object
            default: {}
            additionalProperties:
              type: string

  # ============================================================
  # Resources (templates) - unchanged by schema format choice
  # ============================================================
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${metadata.name}
          namespace: ${metadata.namespace}
          labels: ${metadata.labels}
        spec:
          replicas: ${envOverrides.replicas}
          selector:
            matchLabels: ${metadata.podSelectors}
          template:
            metadata:
              labels: ${metadata.podSelectors}
            spec:
              containers:
                - name: main
                  image: ${workload.container.image}
                  imagePullPolicy: ${envOverrides.imagePullPolicy}
                  resources:
                    requests:
                      cpu: ${envOverrides.resources.requests.cpu}
                      memory: ${envOverrides.resources.requests.memory}
                    limits:
                      cpu: ${envOverrides.resources.limits.cpu}
                      memory: ${envOverrides.resources.limits.memory}
```

---

## Mapping Summary: Simple Schema to OpenAPI v3 Schema

| Simple Schema Feature | OpenAPI v3 Schema Equivalent |
|----------------------|----------------------------|
| `string` | `type: string` |
| `integer` | `type: integer` |
| `number` | `type: number` |
| `boolean` | `type: boolean` |
| `[]string` | `type: array` + `items: {type: string}` |
| `[]CustomType` | `type: array` + `items: {type: object, properties: ...}` |
| `map<string>` | `type: object` + `additionalProperties: {type: string}` |
| `default=value` | `default: value` |
| `minimum=N` | `minimum: N` |
| `maximum=N` | `maximum: N` |
| `minLength=N` | `minLength: N` |
| `maxLength=N` | `maxLength: N` |
| `pattern=regex` | `pattern: "regex"` |
| `enum=a,b,c` | `enum: [a, b, c]` |
| `minItems=N` | `minItems: N` |
| `maxItems=N` | `maxItems: N` |
| `minProperties=N` | `minProperties: N` |
| `maxProperties=N` | `maxProperties: N` |
| `exclusiveMinimum=true` | `exclusiveMinimum: true` |
| `exclusiveMaximum=true` | `exclusiveMaximum: true` |
| `multipleOf=N` | `multipleOf: N` |
| `title='...'` | `title: "..."` |
| `description='...'` | `description: "..."` |
| `example=...` | `example: "..."` |
| `$default: {}` | `default: {}` |
| `types:` section | Inline or `x-openchoreo-types` extension |
| `oc:key=value` | `x-openchoreo-key: value` |
| Implicit required (no default) | Explicit `required: [field1, field2]` |
| Nested object | `type: object` + `properties:` |
