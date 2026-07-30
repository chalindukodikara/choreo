# Converted Sample: Docker Workflow (OpenAPI V3 Schema)

Source file: `samples/workflows/ci/docker.yaml`

This file converts the Workflow schema for the Docker build workflow.

---

## Original (Simple Schema)

```yaml
spec:
  schema:
    parameters:
      repository:
        url: string | description="Git repository URL"
        secretRef: string | description="Secret reference name for private repository Git credentials (optional for public repos)"
        revision:
          branch: string | default=main description="Git branch to checkout"
          commit: string | description="Git commit SHA or reference (optional, defaults to latest commit on branch)"
        appPath: string | default=. description="Path to the application directory within the repository"
      docker:
        context: string | default=. description="Docker build context path relative to the repository root"
        filePath: string | default=./Dockerfile description="Path to the Dockerfile relative to the repository root"
```

---

## Converted (OpenAPI V3 Schema)

```yaml
spec:
  schema:
    openAPIV3Schema:
      parameters:
        type: object
        required:
          - repository    # Has required sub-fields (url, secretRef, commit)
          - docker        # Although all docker fields have defaults, the object itself has no default
        properties:
          repository:
            type: object
            required:
              - url           # No default -> required
              - secretRef     # No default -> required
            properties:
              url:
                type: string
                description: "Git repository URL"
              secretRef:
                type: string
                description: "Secret reference name for private repository Git credentials (optional for public repos)"
              revision:
                type: object
                required:
                  - commit    # No default -> required
                properties:
                  branch:
                    type: string
                    default: main
                    description: "Git branch to checkout"
                  commit:
                    type: string
                    description: "Git commit SHA or reference (optional, defaults to latest commit on branch)"
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

## Field-by-Field Mapping

| Simple Schema Field | Type | Default | Required | OpenAPI V3 Mapping |
|---------------------|------|---------|----------|-------------------|
| `repository.url` | `string` | (none) | Yes | `type: string` in `required` array |
| `repository.secretRef` | `string` | (none) | Yes | `type: string` in `required` array |
| `repository.revision.branch` | `string` | `main` | No | `type: string, default: main` |
| `repository.revision.commit` | `string` | (none) | Yes | `type: string` in `required` array |
| `repository.appPath` | `string` | `.` | No | `type: string, default: "."` |
| `docker.context` | `string` | `.` | No | `type: string, default: "."` |
| `docker.filePath` | `string` | `./Dockerfile` | No | `type: string, default: "./Dockerfile"` |

---

## Notes

### Required vs Optional

In Simple Schema, the rule is simple: **no default = required**. In OpenAPI V3 Schema, this maps to the `required` array on the parent object:

- `repository.url` has no default -> listed in `repository`'s `required` array
- `repository.revision.branch` has `default=main` -> NOT in `required` array
- `docker.context` has `default=.` -> NOT in `required` array

### Nested Object Handling

The `repository` and `docker` objects are nested inline objects. In Simple Schema, nested YAML keys implicitly create objects. In OpenAPI V3, each level needs an explicit `type: object` and `properties` declaration.

The `revision` sub-object has no `$default`/`default` in the Simple Schema, which means it's required. In OpenAPI V3, `revision` is implicitly required because it contains required fields (`commit`) and has no default. However, we don't need to list `revision` in `repository`'s required array separately since `commit` being required already enforces that the `revision` object must be provided.

**Note on `revision` requiredness**: Looking more carefully, `revision` as an object has no `$default` in Simple Schema, so it IS required. We should add it to the required array:

```yaml
required:
  - url
  - secretRef
  - revision    # Object with no default -> required
```

### Description Mapping

`description=` constraint markers map directly to the `description` keyword in OpenAPI V3 Schema. This is a clean 1:1 mapping.

### Verbosity Comparison

- Simple Schema: ~10 lines
- OpenAPI V3 Schema: ~35 lines

This is roughly a 3.5x increase, consistent with the general verbosity overhead observed in the ComponentType conversion.

---

## Full Converted Workflow File

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Workflow
metadata:
  name: docker
  namespace: default
  annotations:
    openchoreo.dev/description: "Docker build workflow for containerized applications using Dockerfile"
    openchoreo.dev/workflow-scope: "component"
    openchoreo.dev/component-workflow-parameters: |
      repoUrl: parameters.repository.url
      branch: parameters.repository.revision.branch
      commit: parameters.repository.revision.commit
      appPath: parameters.repository.appPath
      secretRef: parameters.repository.secretRef
spec:
  buildPlaneRef:
    kind: BuildPlane
    name: default

  ttlAfterCompletion: "1d"

  schema:
    openAPIV3Schema:
      parameters:
        type: object
        required:
          - repository
        properties:
          repository:
            type: object
            required:
              - url
              - secretRef
              - revision
            properties:
              url:
                type: string
                description: "Git repository URL"
              secretRef:
                type: string
                description: "Secret reference name for private repository Git credentials (optional for public repos)"
              revision:
                type: object
                required:
                  - commit
                properties:
                  branch:
                    type: string
                    default: main
                    description: "Git branch to checkout"
                  commit:
                    type: string
                    description: "Git commit SHA or reference (optional, defaults to latest commit on branch)"
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

  # runTemplate, externalRefs, resources sections unchanged
  # (they reference parameters the same way regardless of schema format)
  runTemplate:
    apiVersion: argoproj.io/v1alpha1
    kind: Workflow
    metadata:
      name: ${metadata.workflowRunName}
      namespace: openchoreo-ci-${metadata.namespaceName}
    spec:
      arguments:
        parameters:
          - name: component-name
            value: ${metadata.labels['openchoreo.dev/component']}
          - name: project-name
            value: ${metadata.labels['openchoreo.dev/project']}
          - name: namespace-name
            value: ${metadata.namespaceName}
          - name: git-repo
            value: ${parameters.repository.url}
          - name: branch
            value: ${parameters.repository.revision.branch}
          - name: commit
            value: ${parameters.repository.revision.commit}
          - name: app-path
            value: ${parameters.repository.appPath}
          - name: docker-context
            value: ${parameters.docker.context}
          - name: dockerfile-path
            value: ${parameters.docker.filePath}
          - name: registry-url
            value: gcr.io/openchoreo-dev/images
          - name: build-timeout
            value: "30m"
          - name: image-name
            value: ${metadata.labels['openchoreo.dev/project']}-${metadata.labels['openchoreo.dev/component']}-image
          - name: image-tag
            value: v1
          - name: git-secret
            value: ${metadata.workflowRunName}-git-secret
          - name: registry-push-secret
            value: ${metadata.workflowRunName}-registry-push-secret
      serviceAccountName: workflow-sa
      workflowTemplateRef:
        clusterScope: true
        name: docker
```
