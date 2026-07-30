## Problem Statement

Currently, OpenChoreo has two separate workflow CRD pairs:
- **ComponentWorkflow/ComponentWorkflowRun**: Component-specific workflows with structured system parameters (repository.url, repository.revision.branch, secretRef)
- **Workflow/WorkflowRun**: Generic workflows for arbitrary automation tasks

**Key Questions:**
1. Do we need separate CRs for ComponentWorkflows and Workflows? The implementation is nearly identical except for a few CR fields. ComponentWorkflow has specific required parameters: secretRef, url, branch, commit, appPath.
4. Is maintaining two separate CRD pairs worth the added complexity?

Previously, it was agreed that having separate CRDs would provide clearer separation of concerns and allow for more tailored validation and user experience. However, this has led to some confusion and duplication in the API surface. There is a significant overlap in the underlying implementation.
Refer: https://github.com/openchoreo/openchoreo/discussions/943

## Proposed Solution: Annotation-Based Workflows

### Overview

Unify ComponentWorkflow and Workflow into a single Workflow CRD. Use annotations to indicate that a Workflow is component-aware and to specify where component-specific parameters are located in the schema.

### Workflow and WorkflowRun CRs

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Workflow
metadata:
  name: google-cloud-buildpacks
  namespace: default
  annotations:
    # Recommended (UX): YAML mapping in a multi-line string for readability/editability.
    # The controller can parse this string as YAML into a map[string]string.
    openchoreo.dev/component-workflow-parameters: |
      repoUrl: parameters.repository.url
      branch: parameters.repository.revision.branch
      commit: parameters.repository.revision.commit
      appPath: parameters.repository.appPath
      secretRef: parameters.repository.secretRef
      projectName: parameters.scope.projectName
      componentName: parameters.scope.componentName
spec:
  schema:
    parameters:
        scope:
          projectName: string | description="Name of the project"
          componentName: string | description="Name of the component"
        repository:
          url: string
          revision:
            branch: string | default=main
            commit: string | default=HEAD
          appPath: string | default=.
          secretRef: string | enum=["reading-list-repo-credentials-dev","payments-repo-credentials-dev"]
        version: integer | default=1
        testMode: string | enum=["unit", "integration", "none"] | default=unit
        resources:
          cpuCores: integer | default=1
          memoryGb: integer | default=2
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
            value: ${parameters.componentName}
          - name: project-name
            value: ${parameters.projectName}
          - name: git-repo
            value: ${parameters.repository.url}
          - name: branch
            value: ${parameters.repository.revision.branch}
          - name: commit
            value: ${parameters.repository.revision.commit}
          - name: app-path
            value: ${parameters.repository.appPath}
          - name: image-name
            value: ${metadata.namespaceName}-${parameters.projectName}-${parameters.componentName}
          - name: image-tag
            value: v1
          - name: git-secret
            value: ${metadata.workflowRunName}-git-secret
          - name: registry-push-secret
            value: ${metadata.workflowRunName}-registry-push-secret
      serviceAccountName: workflow-sa
      workflowTemplateRef:
        clusterScope: true
        name: google-cloud-buildpacks
  resources:
    - id: git-secret
      template:
        apiVersion: external-secrets.io/v1
        kind: ExternalSecret
        metadata:
          name: ${metadata.workflowRunName}-git-secret
          namespace: ${metadata.namespace}
        spec:
          refreshInterval: 15s
          secretStoreRef:
            kind: ClusterSecretStore
            name: openbao
          target:
            name: ${metadata.workflowRunName}-git-secret
            creationPolicy: Owner
            template:
              type: ${secretRef.type}
          data:
            - secretKey: ${secretRef.key}
              remoteRef:
                key: ${secretRef.remoteKey}
                property: ${secretRef.property}
    - id: registry-push-secret
      template:
        apiVersion: external-secrets.io/v1
        kind: ExternalSecret
        metadata:
          name: ${metadata.workflowRunName}-registry-push-secret
          namespace: ${metadata.namespace}
        spec:
          data:
            - remoteRef:
                key: registry-push-secret
              secretKey: registrysecret
          refreshInterval: 15s
          secretStoreRef:
            kind: ClusterSecretStore
            name: openbao
          target:
            creationPolicy: Owner
            name: ${metadata.workflowRunName}-registry-push-secret
            template:
              data:
                .dockerconfigjson: ""
              type: kubernetes.io/dockerconfigjson
```

For filtering purposes, WorkflowRun will include project name and component name labels.
```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: foo-component-google-cloud-buildpacks-run-1
  namespace: default
  annotations:
    openchoreo.dev/description: "Google Cloud Buildpacks workflow for containerized builds"
  labels:
    openchoreo.dev/project-name: "" # Used for filtering
    openchoreo.dev/component-name: ""
spec:
```
