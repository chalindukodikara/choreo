# Git Secret Management Improvement Proposal

## Executive Summary

This proposal addresses the complexity in configuring private repository access in OpenChoreo. Currently, Platform Engineers (PEs) must manually store secrets in the Key-Value store and update ComponentWorkflows, creating friction for developers. The proposed solution introduces streamlined secret management that maintains PE governance while simplifying the developer experience.

---

## Problem Statement

### Current Challenges

Private repository configuration currently requires a complex, multi-step process:
- Platform Engineers must manually manage secrets in the KV store
- ComponentWorkflows require manual updates for each secret
- Developers face a high barrier to entry for a fundamental feature
- First-time experience is degraded by unnecessary complexity

### Goal

Simplify private repository access while maintaining Platform Engineer governance and control over secret management policies.

---

## Current Architecture

<img width="1367" height="571" alt="Current Architecture Diagram" src="https://github.com/user-attachments/assets/3b3090fd-2d80-40ed-97b3-48cc64426155" />

### Workflow

1. **Secret Storage**: Platform Engineer manually adds git credentials to the KV store in the **build plane**
2. **Template Definition**: ComponentWorkflow defines an ExternalSecret resource pointing to the stored secret
3. **Runtime Creation**: On build trigger, ComponentWorkflowRun controller creates the ExternalSecret in the build plane
4. **Repository Access**: Clone step retrieves the token, constructs an authenticated URL, and clones the repository

### Limitations

- **High Friction**: Multi-step manual process creates barriers for developers
- **Poor Developer Experience**: Basic functionality requires complex setup
- **Governance Bottleneck**: PE involvement required for every new repository
- **First-Time User Barrier**: Onboarding developers is unnecessarily complicated

---

## Proposed Solution

### Overview

Integrate secret creation and selection directly into the Component Creation flow, enabling self-service while maintaining PE governance through templates and policies.

### Architecture

<img width="1213" height="716" alt="Proposed Architecture Diagram" src="" />

### Implementation Details

#### 1. Secret Creation Flow

**API Integration:**
- OpenChoreo API Server exposes a dedicated endpoint for secret creation
- Developers/PEs can create secrets through the UI/CLI during component setup

**Build Plane Secret Management:**
- Creates a Kubernetes Secret in the **build plane** namespace
- Creates a PushSecret resource to synchronize the secret to the KV store
- PushSecret automatically propagates the secret to the configured secret store
- These secrets will be directly managed in the UI

**Control Plane Reference:**
- Creates a SecretReference resource in the **control plane** namespace
- SecretReference points to the secret in the KV store

#### 2. ComponentWorkflow Integration

**System Parameters Enhancement:**
- New `secretRef` field added to ComponentWorkflow `systemParameters`
- Available in CEL templating context as `${secretRef}`

**Platform Engineer Control:**
- PEs define ExternalSecret templates in ComponentWorkflows
- Templates reference secrets using `${secretRef}`
- Governance maintained through ComponentWorkflow definitions

**Developer Experience:**
- Select existing secrets or create new ones during component creation
- No manual KV store interaction required

### Benefits

- **Simplified Developer Flow**: One-step secret creation during component setup
- **Maintained Governance**: PEs control secret templates and policies
- **Improved Onboarding**: Reduced friction for new users
- **Self-Service**: Developers can manage their own repository credentials
- **Consistent Pattern**: Aligns with OpenChoreo's Claim/Class architecture

---

## Custom Resource Definitions

### ComponentWorkflow

Platform Engineers define ComponentWorkflow templates that include git repository access patterns with secret references.

- `secretRef` field in `systemParameters.repository` for flexible secret management
- CEL expressions for dynamic secret reference field resolution

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ComponentWorkflow
metadata:
  name: google-cloud-buildpacks
  namespace: default
spec:
  schema:
    parameters: {}
    systemParameters:
      repository:
        url: string | description="Git repository URL"
        secretRef: string | description="Secret reference name for Git credentials"
        revision:
          branch: string | default=main description="Git branch to checkout"
          commit: string | description="Git commit SHA or reference (optional, defaults to latest)"
        appPath: string | default=. description="Path to the application directory within the repository"

  # ExternalSecret resource for fetching git credentials from KV store
  resources:
    - id: git-secret
      template:
        apiVersion: external-secrets.io/v1
        kind: ExternalSecret
        metadata:
          name: ${metadata.componentWorkflowRunName}-git-secret
          namespace: openchoreo-ci-${metadata.namespaceName}
        spec:
          refreshInterval: 15s
          secretStoreRef:
            kind: ClusterSecretStore
            name: default
          target:
            name: ${metadata.componentWorkflowRunName}-git-secret
            creationPolicy: Owner
            template:
              type: kubernetes.io/basic-auth
          data:
            - secretKey: ${secretRef.key}
              remoteRef:
                key: ${secretRef.remoteKey}
                property: ${secretRef.property}

  # Argo Workflow template for building container images
  runTemplate:
    apiVersion: argoproj.io/v1alpha1
    kind: Workflow
    metadata:
      name: ${metadata.workflowRunName}
      namespace: ${metadata.namespace}
    spec:
      arguments:
        parameters:
          - name: component-name
            value: ${metadata.componentName}
          - name: project-name
            value: ${metadata.projectName}
          - name: git-repo
            value: ${systemParameters.repository.url}
          - name: branch
            value: ${systemParameters.repository.revision.branch}
          - name: commit
            value: ${systemParameters.repository.revision.commit}
          - name: app-path
            value: ${systemParameters.repository.appPath}
          - name: image-name
            value: ${metadata.projectName}-${metadata.componentName}-image
          - name: image-tag
            value: v1
          - name: git-secret
            value: ${metadata.componentWorkflowRunName}-git-secret
      serviceAccountName: workflow-sa
      workflowTemplateRef:
        clusterScope: true
        name: google-cloud-buildpacks
```

### SecretReference

Control plane resource that provides an abstraction for secrets stored in the KV store. Created by the OpenChoreo API Server when developers create secrets.

**Purpose:**
- Decouples secret references from actual secret storage
- Enables cross-plane secret reference
- Provides a consistent interface for secret access

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: SecretReference
metadata:
  name: <secretName>
  annotations:
    kubernetes.io/secret-type: "basic-auth"
    openchoreo.dev/secret-type: "git-credentials"
  namespace: default
spec:
  plane: build-plane
  template:
    type: kubernetes.io/basic-auth
    metadata:
      labels:
        app: example-component
  data:
    - secretKey: password
      remoteRef:
        key: <secretName>
        property: password
        version: ""
  refreshInterval: 15m
```

### PushSecret

Build plane resource that synchronizes Kubernetes secrets to the external KV store (e.g., Vault). Part of the External Secrets Operator.

**Purpose:**
- Automates secret propagation to KV store
- Maintains secret synchronization
- Enables secret lifecycle management

```yaml
apiVersion: external-secrets.io/v1alpha1
kind: PushSecret
metadata:
  name: <secretName>-credentials
  namespace: openchoreo-system
spec:
  refreshInterval: 1m
  updatePolicy: Replace              # Replace | IfNotExists
  deletionPolicy: Delete             # Delete KV store secret when PushSecret is deleted
  secretStoreRefs:
    - kind: ClusterSecretStore
      name: vault
  selector:
    secret:
      name: <secretName>-credentials    # Source Kubernetes secret
  data:
    - match:
        secretKey: password              # Key in the Kubernetes secret
        remoteRef:
          remoteKey: <secretName>-credentials  # Vault path
          property: password             # Property name in Vault
```

## Conclusion

This proposal streamlines private repository configuration while maintaining the governance and security controls that Platform Engineers require. By integrating secret management into the component creation flow, we reduce friction for developers without compromising on security or operational best practices.
