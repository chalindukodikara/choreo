## Problem Statement

Previously, **ComponentWorkflow** had a hardcoded mechanism for resolving Git `SecretReference` CR. The controller fetches the SecretReference by name from `systemParameters.repository.secretRef`, extracts its data, and injects it into the CEL context as `secretRef`.

**Workflow** has no such mechanism. Its CEL context only contains `metadata` and `parameters`.

As we work toward merging ComponentWorkflow and Workflow into a unified Workflow CRD, we need a generic way for Workflows to reference external CRs (starting with `SecretReference`) and make their data available in CEL context.

## Proposed Solution: `externalRefs` Field in Workflow

Add an `externalRefs` field to the Workflow spec that declares references to external CRs. Each reference has an `id` that becomes available as a variable in CEL context.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Workflow
metadata:
name: google-cloud-buildpacks
namespace: default
annotations:
backstage.io/buildParameters: "repository: parameters.repository.url, branch: parameters.repository.revision.branch, secretRef: parameters.repository.secretRef, projectName: parameters.projectName, componentName: parameters.componentName"
spec:
schema:
parameters:
repository:
# ...

runTemplate:
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
name: ${metadata.workflowRunName}
namespace: openchoreo-ci-${metadata.namespaceName}
spec:
# ...

resources:
# External ref: resolves SecretReference CR and injects into CEL context as "git-secret-reference"
- id: git-secret-reference
externalRef:
apiVersion: openchoreo.dev/v1alpha1
kind: SecretReference
name: ${parameters.repository.secretRef}

# External ref: resolves SecretReference CR and injects into CEL context as "push-secret-reference"
- id: push-secret-reference
externalRef:
apiVersion: openchoreo.dev/v1alpha1
kind: SecretReference
name: ${parameters.registry.secretRef}

# Template: creates an ExternalSecret using the resolved SecretReference data
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
type: ${git-secret-reference.spec.type}
data: |
${git-secret-reference.spec.data.map(secret, {
"secretKey": secret.secretKey,
"remoteRef": {
"key": secret.remoteRef.key,
"property": has(secret.remoteRef.property) && secret.remoteRef.property != "" ? secret.remoteRef.property : oc_omit()
}
})}
```

### How It Works

1. The WorkflowRun controller resolves each `externalRefs` entry:
- Evaluates the `name` field (which can contain CEL expressions like `${parameters.repository.secretRef}`)
- Fetches the referenced CR from the cluster
2. The resolved CR's spec is injected into the CEL context using the `id` as the variable name
3. Templates can then access the CR's fields, e.g., `${git-secret-reference.spec.type}` or `${git-secret-reference.spec.data}`

### Scope

For now, this will be limited to the `SecretReference` kind. We can extend it to support other CR types in the future if needed.

### Option 1: Inline `externalRef` on Resources (Recommended)

Embed the external reference directly within the `resources` array. Instead of a `template`, a resource entry can declare an `externalRef` that resolves an external CR and injects its spec into the CEL context under the resource's `id`.

```yaml
spec:
resources:
- id: git-secret-reference
  contextRef:
    apiVersion: openchoreo.dev/v1alpha1
    kind: SecretReference
    name: ${parameters.repository.secretRef}

- id: push-secret-reference
  contextRef:
    apiVersion: openchoreo.dev/v1alpha1
    kind: SecretReference
    name: ${parameters.registry.secretRef}
```

**How It Works:**

1. Each resource entry has **either** `template` (creates a Kubernetes resource) **or** `contextRef` (resolves an external CR).
2. For `contextRef` entries, the controller:
- Evaluates the `name` field (which can contain CEL expressions like `${parameters.repository.secretRef}`)
- Fetches the referenced CR from the cluster
- Injects the CR's spec into the CEL context using the resource's `id` as the variable name
3. Subsequent `template` resources can reference the resolved CR's fields, e.g., `${git-secret-reference.spec.type}`
4. Resources are processed in order, `contextRef` entries are resolved first, making their data available to later `template` entries


### Option 2: Top-level `contextRefs`

Declare external references as a separate top-level field in the Workflow spec.

```yaml
spec:
 contextRefs:
- id: git-secret-reference
  apiVersion: openchoreo.dev/v1alpha1
  kind: SecretReference
  name: ${parameters.repository.secretRef}

- id: push-secret-reference
  apiVersion: openchoreo.dev/v1alpha1
  kind: SecretReference
  name: ${parameters.registry.secretRef}

resources:
- id: git-secret
  template:
# ... uses ${git-secret-reference.spec.type}
```

Let's go with Option 2 as it is cleaner and separates concerns. The `contextRefs` field clearly indicates that these are references to external CRs, while `resources` is focused on defining the Kubernetes resources to create. This separation also allows for better organization and readability of the Workflow spec.

References:
- component types
api/v1alpha1/componenttype_types.go
samples/getting-started/component-types/service.yaml

- workflows
api/v1alpha1/workflow_types.go
samples/getting-started/workflows/docker.yaml

- cel templating
docs/templating/*

Tasks
We have already implemented this option 2 with `contextRefs` field. I want to evaluate what is the best field. Find pros and cons of each option.

1. First evaluate refs vs references

2. contextRefs vs contextReferences vs resourceReferences vs externalRefs vs externalReferences vs references. Evaluate each option considering pros and cons and find the better field name to go with which indicate we are reading the CR and allow you to access those.
