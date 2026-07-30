# OpenAPI V3 Schema Reference for OpenChoreo

This document demonstrates **every schema feature** OpenChoreo currently supports, expressed in `openAPIV3Schema` format. It serves as a comprehensive reference for platform engineers writing schemas.

---

## Complete Reference YAML

The following single YAML document covers all supported scenarios:

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ComponentType
metadata:
  name: comprehensive-reference
  namespace: default
spec:
  workloadType: deployment
  schema:
    openAPIV3Schema:

      # ============================================================
      # Reusable type definitions (equivalent of Simple Schema 'types')
      # ============================================================
      # NOTE: $defs/$ref are NOT supported in Kubernetes CRD *definitions*
      # (apiextensions.k8s.io CustomResourceDefinition's openAPIV3Schema).
      # However, they ARE valid here because this schema lives as opaque data
      # inside a ComponentType *instance* — K8s stores it in etcd as-is without
      # interpreting it. OpenChoreo's controller reads this schema, resolves
      # $ref references in-memory, and uses the resolved schema to validate
      # developer-provided parameters/envOverrides.
      $defs:

        # Simple reusable type
        ResourceQuantity:
          type: object
          properties:
            cpu:
              type: string
              default: "100m"
            memory:
              type: string
              default: "256Mi"

        # Reusable type that references another reusable type
        ResourceRequirements:
          type: object
          properties:
            requests:
              $ref: "#/$defs/ResourceQuantity"
              default: {}
            limits:
              $ref: "#/$defs/ResourceQuantity"
              default: {}

        # Reusable type with its own default (equivalent of $default: {} in Simple Schema)
        # All references to this type are automatically optional
        Probe:
          type: object
          default: {}
          properties:
            path:
              type: string
              default: /healthz
            port:
              type: integer
              default: 8080
            initialDelaySeconds:
              type: integer
              default: 0
            periodSeconds:
              type: integer
              default: 10

        # Reusable type with required fields
        DatabaseConfig:
          type: object
          required:
            - host
            - database
            - username
            - password
          properties:
            host:
              type: string
            database:
              type: string
            username:
              type: string
            password:
              type: string
            port:
              type: integer
              default: 5432
              minimum: 1
              maximum: 65535

      # ============================================================
      # PARAMETERS section
      # ============================================================
      parameters:
        type: object
        required:
          - appName             # Required: no default
          - age                 # Required: no default
          - price               # Required: no default
          - tags                # Required: no default
          - metadata            # Required: no default
          - database            # Required: no default
        properties:

          # ----------------------------------------------------------
          # 1. PRIMITIVE TYPES
          # ----------------------------------------------------------

          # String - required (no default)
          appName:
            type: string
            title: "Application Name"
            description: "The name of the application to deploy"
            # 'example' is an OpenAPI 3.0 keyword (singular, not 'examples')
            example: "my-web-app"

          # String with constraints
          username:
            type: string
            minLength: 3
            maxLength: 20
            pattern: "^[a-z][a-z0-9_]*$"
            default: "admin"
            title: "Username"
            description: "Login username (lowercase alphanumeric with underscores)"
            example: "john_doe"

          # String with enum
          environment:
            type: string
            enum:
              - development
              - staging
              - production
            default: development
            description: "Target deployment environment"

          # Integer - required (no default)
          age:
            type: integer
            minimum: 0
            maximum: 150
            description: "User age in years"

          # Integer with all numeric constraints
          statusCode:
            type: integer
            enum:
              - 200
              - 201
              - 204
              - 400
              - 404
              - 500
            default: 200
            description: "Expected HTTP status code"

          # Number (float) - required (no default)
          price:
            type: number
            minimum: 0
            # In OpenAPI 3.0, exclusiveMinimum is a boolean that modifies 'minimum'
            # In OpenAPI 3.1 / JSON Schema draft 2020-12, it's a numeric value
            # For Kubernetes CRD compatibility, use the OpenAPI 3.0 boolean form:
            exclusiveMinimum: true
            multipleOf: 0.01
            description: "Product price (must be positive, in 1-cent increments)"

          # Number with maximum constraints
          temperature:
            type: number
            maximum: 100
            exclusiveMaximum: true
            default: 20.0
            description: "Temperature reading (must be below 100)"

          # Boolean with default
          enabled:
            type: boolean
            default: false
            description: "Enable or disable the feature"

          # ----------------------------------------------------------
          # 2. ARRAYS
          # ----------------------------------------------------------

          # Array of strings - required (no default)
          tags:
            type: array
            items:
              type: string
            minItems: 1
            maxItems: 10
            description: "Tags for the application (1-10 tags required)"

          # Array of strings with default (optional)
          labels:
            type: array
            items:
              type: string
            default: []
            description: "Optional labels"

          # Array of integers
          ports:
            type: array
            items:
              type: integer
              minimum: 1
              maximum: 65535
            default:
              - 8080
            description: "Ports to expose"

          # Array of objects (inline object definition)
          endpoints:
            type: array
            items:
              type: object
              required:
                - name
                - port
              properties:
                name:
                  type: string
                port:
                  type: integer
                  minimum: 1
                  maximum: 65535
                protocol:
                  type: string
                  enum:
                    - HTTP
                    - HTTPS
                    - gRPC
                  default: HTTP
            default: []
            description: "Application endpoints to configure"

          # Array of custom type (using $ref)
          healthProbes:
            type: array
            items:
              $ref: "#/$defs/Probe"
            default: []
            description: "Health check probes"

          # ----------------------------------------------------------
          # 3. MAPS
          # ----------------------------------------------------------

          # Map with string values - required (no default)
          metadata:
            type: object
            additionalProperties:
              type: string
            minProperties: 1
            maxProperties: 10
            description: "Key-value metadata (1-10 entries)"

          # Map with integer values (optional)
          portMappings:
            type: object
            additionalProperties:
              type: integer
            default: {}
            description: "Named port mappings"

          # Map with boolean values (optional)
          featureFlags:
            type: object
            additionalProperties:
              type: boolean
            default: {}
            description: "Feature flag toggles"

          # ----------------------------------------------------------
          # 4. NESTED OBJECTS (inline)
          # ----------------------------------------------------------

          # Nested object with required fields
          database:
            $ref: "#/$defs/DatabaseConfig"
            # Override the type-level default with a field-level default
            # This provides required fields that DatabaseConfig demands
            default:
              host: "localhost"
              database: "mydb"
              username: "root"
              password: "changeme"

          # Nested object with all fields having defaults (optional via default: {})
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
              path:
                type: string
                default: /metrics

          # Deeply nested object
          logging:
            type: object
            default: {}
            properties:
              level:
                type: string
                enum:
                  - debug
                  - info
                  - warn
                  - error
                default: info
              output:
                type: object
                default: {}
                properties:
                  console:
                    type: boolean
                    default: true
                  file:
                    type: object
                    default: {}
                    properties:
                      enabled:
                        type: boolean
                        default: false
                      path:
                        type: string
                        default: /var/log/app.log
                      maxSizeMB:
                        type: integer
                        default: 100
                        minimum: 1

          # ----------------------------------------------------------
          # 5. REUSABLE TYPES (via $ref)
          # ----------------------------------------------------------

          # Reference a reusable type
          resources:
            $ref: "#/$defs/ResourceRequirements"
            default: {}

          # Reference with override default
          livenessProbe:
            $ref: "#/$defs/Probe"
            # Override the type-level default with custom values
            default:
              path: /health
              port: 8080
              initialDelaySeconds: 15
              periodSeconds: 20

          readinessProbe:
            $ref: "#/$defs/Probe"
            # Uses the type-level default: {}

          # ----------------------------------------------------------
          # 6. CUSTOM ANNOTATIONS (x-openchoreo-* extensions)
          # ----------------------------------------------------------

          # Equivalent of: commitHash: "string | oc:build:inject=git.sha oc:ui:hidden=true"
          commitHash:
            type: string
            default: ""
            x-openchoreo-build-inject: git.sha
            x-openchoreo-ui-hidden: true

          # Equivalent of: advancedTimeout: "string | default='30s' oc:scaffolding=omit"
          advancedTimeout:
            type: string
            default: "30s"
            x-openchoreo-scaffolding: omit

          # ----------------------------------------------------------
          # 7. BACKSTAGE UI INTEGRATION (vendor extensions)
          # ----------------------------------------------------------

          # Backstage RepoUrlPicker integration
          repoUrl:
            type: string
            description: "Repository URL for the project"
            default: ""
            x-openchoreo-backstage-portal:
              ui:field: RepoUrlPicker
              ui:options:
                allowedHosts:
                  - github.com
                  - gitlab.com
                allowedOwners:
                  - my-org

          # Backstage multi-select
          cloudRegions:
            type: array
            items:
              type: string
              enum:
                - us-east-1
                - us-west-2
                - eu-west-1
                - ap-southeast-1
            default: ["us-east-1"]
            x-openchoreo-backstage-portal:
              ui:widget: checkboxes
              ui:options:
                inline: true

          # Backstage textarea
          description:
            type: string
            default: ""
            x-openchoreo-backstage-portal:
              ui:widget: textarea
              ui:options:
                rows: 5

          # ----------------------------------------------------------
          # 8. SCHEMA EVOLUTION (additionalProperties)
          # ----------------------------------------------------------

          # Object that allows additional unknown properties
          # This matches Simple Schema's default behavior (no additionalProperties: false)
          extensibleConfig:
            type: object
            additionalProperties: true
            default: {}
            properties:
              knownField:
                type: string
                default: "value"
            description: >
              This object allows additional properties beyond 'knownField'.
              New fields can be added without updating the schema, supporting
              gradual schema evolution.

      # ============================================================
      # ENVOVERRIDES section
      # ============================================================
      envOverrides:
        type: object
        properties:

          # Simple overridable fields
          replicas:
            type: integer
            default: 1
            minimum: 0
            description: "Number of replicas for this environment"

          # Reference reusable type for environment-specific resources
          resources:
            $ref: "#/$defs/ResourceRequirements"
            default: {}

          imagePullPolicy:
            type: string
            enum:
              - Always
              - IfNotPresent
              - Never
            default: IfNotPresent

          # Nested object in envOverrides
          autoscaling:
            type: object
            required:
              - minReplicas
              - maxReplicas
            properties:
              minReplicas:
                type: integer
                minimum: 1
              maxReplicas:
                type: integer
                minimum: 1

          # Environment-specific feature toggles
          featureFlags:
            type: object
            additionalProperties:
              type: boolean
            default: {}

          # Environment-specific logging level
          logLevel:
            type: string
            enum:
              - debug
              - info
              - warn
              - error
            default: info
```

---

## Feature-by-Feature Reference

### 1. Primitive Types

| Simple Schema | OpenAPI V3 Schema |
|---------------|-------------------|
| `name: string` | `name: {type: string}` |
| `age: "integer \| minimum=0"` | `age: {type: integer, minimum: 0}` |
| `price: "number \| minimum=0.01"` | `price: {type: number, minimum: 0.01}` |
| `enabled: "boolean \| default=false"` | `enabled: {type: boolean, default: false}` |

### 2. String Constraints

```yaml
# All string constraints demonstrated:
field:
  type: string
  minLength: 3          # Minimum length
  maxLength: 100        # Maximum length
  pattern: "^[a-z]+$"  # Regex pattern
  enum:                 # Allowed values
    - a
    - b
    - c
  default: "a"         # Default value
```

### 3. Numeric Constraints

```yaml
# All numeric constraints demonstrated:
field:
  type: integer    # or 'number' for floats
  minimum: 0
  maximum: 100
  exclusiveMinimum: true   # OpenAPI 3.0: boolean (makes minimum exclusive)
  exclusiveMaximum: true   # OpenAPI 3.0: boolean (makes maximum exclusive)
  multipleOf: 5
  enum: [10, 20, 30, 40, 50]
  default: 10
```

**Note on `exclusiveMinimum`/`exclusiveMaximum`:**
- In **OpenAPI 3.0** (Kubernetes CRDs): These are **booleans** that modify the meaning of `minimum`/`maximum`
- In **OpenAPI 3.1** / **JSON Schema 2020-12**: These are **numeric values** (standalone, not modifiers)
- For Kubernetes compatibility, use the boolean form

### 4. Boolean with Default

```yaml
enabled:
  type: boolean
  default: false
```

### 5. Arrays

```yaml
# Array of primitives
tags:
  type: array
  items:
    type: string
  minItems: 1
  maxItems: 10
  default: []

# Array of objects
endpoints:
  type: array
  items:
    type: object
    required: [name, port]
    properties:
      name: {type: string}
      port: {type: integer}
  default: []

# Array of custom type
probes:
  type: array
  items:
    $ref: "#/$defs/Probe"
  default: []
```

### 6. Maps

```yaml
# Map with string values (equivalent of map<string>)
labels:
  type: object
  additionalProperties:
    type: string
  minProperties: 1
  maxProperties: 10
  default: {}

# Map with integer values (equivalent of map<integer>)
portMappings:
  type: object
  additionalProperties:
    type: integer
  default: {}
```

**Key insight**: In OpenAPI V3, maps are represented as objects with `additionalProperties` defining the value type. The key type is always string (same as Simple Schema).

### 7. Nested Objects

```yaml
# Inline nested object
database:
  type: object
  required: [host]
  properties:
    host: {type: string}
    port: {type: integer, default: 5432}

# Nested object with default (optional)
monitoring:
  type: object
  default: {}          # Equivalent of $default: {}
  properties:
    enabled: {type: boolean, default: false}
    port: {type: integer, default: 9090}
```

### 8. Reusable Types

```yaml
# Definition (in $defs)
$defs:
  MyType:
    type: object
    properties:
      field1: {type: string, default: "value"}

# Usage (via $ref)
myField:
  $ref: "#/$defs/MyType"
  default: {}        # Override default from type definition
```

**Kubernetes limitation**: `$defs`/`$ref` are NOT supported in CRD `openAPIV3Schema`. OpenChoreo must resolve these internally (inline expansion) before the schema reaches Kubernetes.

### 9. Documentation

```yaml
field:
  type: string
  title: "Human-Readable Title"
  description: "Detailed description of the field's purpose"
  example: "example-value"    # OpenAPI 3.0: singular 'example'
```

### 10. Custom Annotations (`x-openchoreo-*`)

| Simple Schema (`oc:*`) | OpenAPI V3 (`x-openchoreo-*`) |
|-------------------------|-------------------------------|
| `oc:build:inject=git.sha` | `x-openchoreo-build-inject: git.sha` |
| `oc:ui:hidden=true` | `x-openchoreo-ui-hidden: true` |
| `oc:scaffolding=omit` | `x-openchoreo-scaffolding: omit` |

**Advantage**: `x-openchoreo-*` supports rich values (objects, arrays, booleans), not just strings:

```yaml
field:
  type: string
  x-openchoreo-ui:
    hidden: true
    order: 5
    group: "Advanced"
```

### 11. Backstage Integration

```yaml
field:
  type: string
  x-openchoreo-backstage-portal:
    ui:field: RepoUrlPicker
    ui:options:
      allowedHosts: ["github.com"]
    ui:widget: textarea
    ui:help: "Select your repository"
```

### 12. Schema Evolution (`additionalProperties`)

```yaml
# Allow unknown properties (matches Simple Schema default behavior)
config:
  type: object
  additionalProperties: true    # Explicit: allow unknown fields
  properties:
    knownField: {type: string}

# Strict mode (reject unknown properties)
strictConfig:
  type: object
  additionalProperties: false   # Reject any field not in 'properties'
  required: [name]
  properties:
    name: {type: string}
```

**Default behavior in different contexts:**
| Context | Default `additionalProperties` |
|---------|-------------------------------|
| JSON Schema | `true` (allow unknown fields) |
| Kubernetes CRD | Pruned (unknown fields removed) unless `x-kubernetes-preserve-unknown-fields: true` |
| OpenChoreo Simple Schema | `true` (allow unknown fields for schema evolution) |
| OpenChoreo OpenAPI V3 | Should default to `true` to match Simple Schema behavior |

### 13. Parameters vs EnvOverrides

Both `parameters` and `envOverrides` (or `environmentConfig`) are top-level keys under `openAPIV3Schema`:

```yaml
spec:
  schema:
    openAPIV3Schema:
      parameters:
        type: object
        properties:
          # Fields set once at component creation time
          appName: {type: string}

      envOverrides:
        type: object
        properties:
          # Fields that can be tuned per environment
          replicas: {type: integer, default: 1}
```

**Semantics:**
- `parameters`: Set by developers when creating a Component. Immutable after creation (unless the Component is updated).
- `envOverrides` / `environmentConfig`: Set by platform teams per environment in ReleaseBindings. Can differ across dev/staging/production.

---

## Quick Conversion Cheat Sheet

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
| `field: "string \| title=Title description=Desc"` | `field: {type: string, title: "Title", description: "Desc"}` |
| `field: "string \| oc:ui:hidden=true"` | `field: {type: string, x-openchoreo-ui-hidden: true}` |
| `field: CustomType` | `field: {$ref: "#/$defs/CustomType"}` |
| `field: "CustomType \| default={}"` | `field: {$ref: "#/$defs/CustomType", default: {}}` |
| `types: { MyType: ... }` | `$defs: { MyType: ... }` |
| `$default: {}` (in object def) | `default: {}` (on the schema node) |
| No default on field | Field name in parent's `required` array |
