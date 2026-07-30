# Proposal: External CR References in Workflow

## Problem Statement

Previously, **ComponentWorkflow** had a hardcoded mechanism for resolving Git `SecretReference` CR. The controller fetches the SecretReference by name from `systemParameters.repository.secretRef`, extracts its data, and injects it into the CEL context as `secretRef`.

**Workflow** has no such mechanism. Its CEL context only contains `metadata` and `parameters`.

As we work toward merging ComponentWorkflow and Workflow into a unified Workflow CRD, we need a generic way for Workflows to reference external CRs (starting with `SecretReference`) and make their data available in CEL context.

## Proposed Solution: Inline `externalRef` on Resources

Extend the existing `resources` array in the Workflow spec so that each resource entry can declare an `externalRef` instead of a `template`. An `externalRef` entry resolves an external CR and injects its spec into the CEL context under the resource's `id`, making it available to subsequent template resources.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Workflow
metadata:
  name: google-cloud-buildpacks
  namespace: default
  annotations:
    backstage.io/buildParameters: "
      repository: parameters.repository.url,
      branch: parameters.repository.revision.branch,
      secretRef: parameters.repository.secretRef"
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
      includeWhen: has(parameters.repository.secretRef) && parameters.repository.secretRef != ""
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

1. Resources are processed in order. For each entry with `externalRef`:
   - The `name` field is evaluated (can contain CEL expressions like `${parameters.repository.secretRef}`)
   - The referenced CR is fetched from the cluster
   - The CR's spec is injected into the CEL context using the resource's `id` as the variable name
2. For each entry with `template`:
   - The template is rendered with the full CEL context (including any previously resolved external refs)
   - The rendered resource is created in the cluster
3. Templates can access resolved CR fields, e.g., `${git-secret-reference.spec.type}` or `${git-secret-reference.spec.data}`

### Key Design Points

- Each resource entry has **either** `externalRef` **or** `template` — never both
- `externalRef` entries don't create resources; they only populate the CEL context
- `includeWhen` works on both `externalRef` and `template` entries for conditional resolution
- The `id` serves dual purpose: CEL context variable name and unique identifier

### Scope

For now, `externalRef` will be limited to the `SecretReference` kind. We can extend it to support other CR types in the future if needed.

## Options

### Option 1: Inline `externalRef` on Resources (Recommended)

Embed the external reference directly within the `resources` array. Instead of a `template`, a resource entry can declare an `externalRef` that resolves an external CR and injects its spec into the CEL context under the resource's `id`.

```yaml
spec:
  resources:
    - id: git-secret-reference
      externalRef:
        apiVersion: openchoreo.dev/v1alpha1
        kind: SecretReference
        name: ${parameters.repository.secretRef}

    - id: push-secret-reference
      externalRef:
        apiVersion: openchoreo.dev/v1alpha1
        kind: SecretReference
        name: ${parameters.registry.secretRef}

    - id: git-secret
      includeWhen: has(parameters.repository.secretRef) && parameters.repository.secretRef != ""
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

**How It Works:**

1. Each resource entry has **either** `template` (creates a Kubernetes resource) **or** `externalRef` (resolves an external CR).
2. For `externalRef` entries, the controller:
   - Evaluates the `name` field (which can contain CEL expressions like `${parameters.repository.secretRef}`)
   - Fetches the referenced CR from the cluster
   - Injects the CR's spec into the CEL context using the resource's `id` as the variable name
3. Subsequent `template` resources can reference the resolved CR's fields, e.g., `${git-secret-reference.spec.type}`
4. Resources are processed in order — `externalRef` entries are resolved first, making their data available to later `template` entries

**Advantages:**
- No new top-level field required — reuses the existing `resources` array
- Clear co-location: the external reference and the resources that consume it live in the same list
- The `id` already serves as the CEL context variable name for resources
- Extensible to any CR type via `apiVersion` and `kind`
- Processing order is explicit and intuitive (declare refs before templates that use them)
- `includeWhen` can be used on both `externalRef` and `template` entries for conditional resolution

**Disadvantages:**
- Overloads the `resources` array with two different resource types (external refs vs templates)
- Slightly less obvious that `externalRef` entries don't create resources

### Option 2: Top-level `externalRefs`

Declare external references as a separate top-level field in the Workflow spec.

```yaml
spec:
  externalRefs:
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

**Advantages:**
- Clear separation between external references and resource templates
- Explicit about which CR types are being referenced

**Disadvantages:**
- Introduces a new top-level field to the Workflow spec
- External refs and the resources that consume them are separated, making it harder to see the relationship
- The `id` namespace is shared across `externalRefs` and `resources`, which could cause confusion

### Option 3: SecretReference-specific `secretRefs`

```yaml
spec:
  secretRefs:
    - id: git-secret-reference
      name: ${parameters.repository.secretRef}

    - id: push-secret-reference
      name: ${parameters.registry.secretRef}
```

**Advantages:**
- Simpler and less verbose

**Disadvantages:**
- Limited to `SecretReference` only — would require new top-level fields for other CR types
- Not extensible without schema changes

---

**Recommendation:** Option 1, since it reuses the existing `resources` array, keeps external references co-located with the templates that consume them, and is extensible to any CR type without introducing new top-level fields.

We encountered a problem with the current approach where there is no way to inject a SecretReference into the CEL context and use it when defining ExternalSecret resources. To solve this, I have proposed this design: https://github.com/openchoreo/openchoreo/discussions/2035

## Open Questions

1. Should we support `externalRef` in ComponentType resource templates as well?
2. Should `externalRef` resolution order be enforced (all refs resolved before templates), or should it follow list order strictly?
3. Should we validate that `externalRef` entries are declared before the `template` entries that reference them?