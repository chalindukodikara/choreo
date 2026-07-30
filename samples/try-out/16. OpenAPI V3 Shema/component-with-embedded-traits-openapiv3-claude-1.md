# Converted Sample: Component with Embedded Traits (OpenAPI V3 Schema)

Source file: `samples/component-types/component-with-embedded-traits/component-with-embedded-traits.yaml`

This file converts the schemas from three resources:
1. **Trait**: `horizontal-pod-autoscaler`
2. **Trait**: `persistent-volume`
3. **ComponentType**: `service-with-autoscaling` (with custom types)

---

## 1. Trait: horizontal-pod-autoscaler

### Original (Simple Schema)

```yaml
spec:
  schema:
    parameters:
      enabled: "boolean | default=false"
      minReplicas: "integer | default=2 | minimum=1"
      maxReplicas: "integer | default=10 | minimum=1"
      targetCPUUtilizationPercentage: "integer | default=80 | minimum=1 | maximum=100"

    envOverrides:
      minReplicas: "integer | minimum=1"
      maxReplicas: "integer | minimum=1"
```

### Converted (OpenAPI V3 Schema)

```yaml
spec:
  schema:
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
        # No 'required' array needed: all fields have defaults, so all are optional

      envOverrides:
        type: object
        properties:
          minReplicas:
            type: integer
            minimum: 1
          maxReplicas:
            type: integer
            minimum: 1
        # minReplicas and maxReplicas have no defaults, so they are required
        # when envOverrides is provided. However, envOverrides itself is optional
        # (only supplied in ReleaseBinding when overriding).
        required:
          - minReplicas
          - maxReplicas
```

### Notes
- In Simple Schema, all fields without `default=` are required. In OpenAPI V3, this maps to the `required` array.
- `envOverrides.minReplicas` and `envOverrides.maxReplicas` have no defaults, so they are listed in `required`.
- All `parameters` fields have defaults, so no `required` array is needed for `parameters`.

---

## 2. Trait: persistent-volume

### Original (Simple Schema)

```yaml
spec:
  schema:
    parameters:
      volumeName: "string"
      mountPath: "string"
      containerName: "string | default=main"

    envOverrides:
      size: "string | default=10Gi"
      storageClass: "string | default=local-path"
```

### Converted (OpenAPI V3 Schema)

```yaml
spec:
  schema:
    openAPIV3Schema:
      parameters:
        type: object
        required:
          - volumeName      # No default -> required
          - mountPath        # No default -> required
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
        # All fields have defaults, so no 'required' array needed
```

### Notes
- `volumeName` and `mountPath` are required because they have no defaults in the Simple Schema.
- `containerName` has `default=main`, making it optional.
- All `envOverrides` fields have defaults, making the entire section optional.

---

## 3. ComponentType: service-with-autoscaling

### Original (Simple Schema)

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
      autoscaling:
        minReplicas: "integer | minimum=1"
        maxReplicas: "integer | minimum=1"
```

### Converted (OpenAPI V3 Schema) - With `$defs`/`$ref` (Authoring Format)

This is the cleanest authoring experience, using `$defs` and `$ref` for type reuse. OpenChoreo would resolve these internally before use.

```yaml
spec:
  schema:
    openAPIV3Schema:
      # Reusable type definitions (equivalent of Simple Schema 'types')
      # $defs/$ref are stored as-is in etcd (opaque data under x-kubernetes-preserve-unknown-fields).
      # Kubernetes does NOT parse or validate these — only OpenChoreo's controller does.
      # The controller resolves $ref in-memory when validating developer-provided parameters.
      $defs:
        ResourceQuantity:
          type: object
          properties:
            cpu:
              type: string
              default: "100m"
            memory:
              type: string
              default: "256Mi"

        ResourceRequirements:
          type: object
          properties:
            requests:
              $ref: "#/$defs/ResourceQuantity"
              default: {}
            limits:
              $ref: "#/$defs/ResourceQuantity"
              default: {}

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
            # All fields have defaults -> no 'required' array

      envOverrides:
        type: object
        properties:
          replicas:
            type: integer
            default: 1
          resources:
            $ref: "#/$defs/ResourceRequirements"
            default: {}
          imagePullPolicy:
            type: string
            default: IfNotPresent
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
```

### Converted (OpenAPI V3 Schema) - Fully Inlined (Kubernetes-Compatible)

This is the resolved version with all `$ref` expanded inline. This is what Kubernetes (and validation libraries) would actually process.

```yaml
spec:
  schema:
    openAPIV3Schema:
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

      envOverrides:
        type: object
        properties:
          replicas:
            type: integer
            default: 1
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
          imagePullPolicy:
            type: string
            default: IfNotPresent
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
```

### Notes

- **Type reuse**: `ResourceQuantity` is defined once in `$defs` and referenced twice (in `requests` and `limits`). In the inlined version, the definition is duplicated at each usage site.
- **Default cascading**: `resources: "ResourceRequirements | default={}"` maps to `resources: {$ref: ..., default: {}}`. When resolved, the object gets `default: {}` and its children (`requests`, `limits`) each get `default: {}`, and their children (`cpu`, `memory`) get their own defaults.
- **Required fields in envOverrides.autoscaling**: `minReplicas` and `maxReplicas` have no defaults in the Simple Schema, so they are listed in `required`. This means if a PE provides `envOverrides.autoscaling`, they must provide both fields.
- **Verbosity comparison**: The Simple Schema version is ~15 lines for the schema section. The inlined OpenAPI V3 version is ~45 lines (~3x increase). The `$defs`/`$ref` version is ~40 lines but cleaner for maintenance.

---

## Full Converted File (Inlined Version)

For reference, here is the complete converted file with all three resources using `openAPIV3Schema`:

```yaml
# Copyright 2025 The OpenChoreo Authors
# SPDX-License-Identifier: Apache-2.0

---
# HorizontalPodAutoscaler Trait - OpenAPI V3 Schema version
apiVersion: openchoreo.dev/v1alpha1
kind: Trait
metadata:
  name: horizontal-pod-autoscaler
  namespace: default
spec:
  schema:
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

  creates:
    - includeWhen: ${parameters.enabled}
      template:
        apiVersion: autoscaling/v2
        kind: HorizontalPodAutoscaler
        metadata:
          name: ${metadata.name}
          namespace: ${metadata.namespace}
          labels: ${metadata.labels}
        spec:
          scaleTargetRef:
            apiVersion: apps/v1
            kind: Deployment
            name: ${metadata.name}
          minReplicas: |
            ${has(envOverrides.minReplicas) ? envOverrides.minReplicas : parameters.minReplicas}
          maxReplicas: |
            ${has(envOverrides.maxReplicas) ? envOverrides.maxReplicas : parameters.maxReplicas}
          metrics:
            - type: Resource
              resource:
                name: cpu
                target:
                  type: Utilization
                  averageUtilization: ${parameters.targetCPUUtilizationPercentage}

---
# Persistent Volume Trait - OpenAPI V3 Schema version
apiVersion: openchoreo.dev/v1alpha1
kind: Trait
metadata:
  name: persistent-volume
  namespace: default
spec:
  schema:
    openAPIV3Schema:
      parameters:
        type: object
        required:
          - volumeName
          - mountPath
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

  creates:
    - template:
        apiVersion: v1
        kind: PersistentVolumeClaim
        metadata:
          name: ${metadata.name}-${trait.instanceName}
          namespace: ${metadata.namespace}
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: ${envOverrides.size}
          storageClassName: ${envOverrides.storageClass}

  patches:
    - target:
        group: apps
        version: v1
        kind: Deployment
      operations:
        - op: add
          path: /spec/template/spec/volumes/-
          value:
            name: ${parameters.volumeName}
            persistentVolumeClaim:
              claimName: ${metadata.name}-${trait.instanceName}
    - target:
        group: apps
        version: v1
        kind: Deployment
      operations:
        - op: add
          path: /spec/template/spec/containers/[?(@.name=='${parameters.containerName}')]/volumeMounts/-
          value:
            name: ${parameters.volumeName}
            mountPath: ${parameters.mountPath}

---
# ComponentType: service-with-autoscaling - OpenAPI V3 Schema version (fully inlined)
apiVersion: openchoreo.dev/v1alpha1
kind: ComponentType
metadata:
  name: service-with-autoscaling
  namespace: default
spec:
  workloadType: deployment

  allowedWorkflows:
    - kind: Workflow
      name: google-cloud-buildpacks
    - kind: Workflow
      name: ballerina-buildpack
    - kind: Workflow
      name: docker

  allowedTraits:
    - name: api-configuration
    - name: observability-alert-rule
    - name: persistent-volume

  validations:
    - rule: "${size(workload.endpoints) > 0}"
      message: "Service components must have at least one endpoint."

  schema:
    openAPIV3Schema:
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

      envOverrides:
        type: object
        properties:
          replicas:
            type: integer
            default: 1
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
          imagePullPolicy:
            type: string
            default: IfNotPresent
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

  traits:
    - name: horizontal-pod-autoscaler
      instanceName: autoscaler
      parameters:
        enabled: ${parameters.autoscaling.enabled}
        minReplicas: ${parameters.autoscaling.minReplicas}
        maxReplicas: ${parameters.autoscaling.maxReplicas}
        targetCPUUtilizationPercentage: 75
      envOverrides:
        minReplicas: ${envOverrides.autoscaling.minReplicas}
        maxReplicas: ${envOverrides.autoscaling.maxReplicas}

  resources:
    # ... (resource templates unchanged - they reference parameters/envOverrides the same way)
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
