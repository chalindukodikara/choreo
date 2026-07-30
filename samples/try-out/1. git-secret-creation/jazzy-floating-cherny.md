# Implementation Plan: Git Secret Creation API

## Overview

Implement a new API endpoint to create git secrets through the OpenChoreo API server. This endpoint will:
1. Accept a secret name and PAT token
2. Create a Kubernetes secret in the build plane's `openchoreo-system` namespace
3. Create a PushSecret to push the secret to the key vault
4. Create a SecretReference in the control plane namespace

## API Specification

```
POST /api/v1/namespaces/{namespaceName}/git-secrets
Body: { "secretName": "string", "token": "string" }
```

## Current Understanding

### Architecture Flow
1. **OpenChoreo API** receives the request
2. **BuildPlaneService** retrieves the BuildPlane CR from the namespace
3. **Cluster Gateway Client** is used to create resources in the build plane cluster
4. Three resources are created:
   - K8s Secret (basic-auth type) in `openchoreo-system` namespace on build plane
   - PushSecret to push to key vault in `openchoreo-system` namespace on build plane
   - SecretReference in the control plane namespace

### Key Findings

#### 1. Cluster Gateway URL Configuration (MISSING)
The openchoreo-api currently does NOT have cluster gateway configuration:
- Controller manager has it via `--cluster-gateway-url` flag
- BuildPlaneService has `GetBuildPlaneClient(ctx, namespace, gatewayURL)` but no source for gatewayURL
- Need to add `ClusterGateway` config to openchoreo-api config structure

**Files to modify:**
- `internal/openchoreo-api/config/config.go` - Add ClusterGatewayConfig
- `internal/openchoreo-api/config/` - Create new cluster_gateway.go
- `cmd/openchoreo-api/main.go` - Pass config to services
- `internal/openchoreo-api/services/services.go` - Accept gateway config
- `internal/openchoreo-api/services/buildplane_service.go` - Store gatewayURL
- `install/helm/openchoreo-control-plane/templates/openchoreo-api/configmap.yaml` - Add config
- `install/helm/openchoreo-control-plane/templates/openchoreo-api/deployment.yaml` - Mount CA cert

#### 2. PushSecret Type (MISSING)
The codebase has ExternalSecret types but NOT PushSecret:
- `internal/dataplane/kubernetes/types/externalsecrets/v1/` has ExternalSecret
- Need to add PushSecret types (external-secrets.io/v1alpha1)
- Need to register with scheme in `internal/clients/kubernetes/client.go`

#### 3. SecretReference Service (PARTIAL)
SecretReference service exists but only for listing:
- `ListSecretReferences()` exists
- Need to add `CreateSecretReference()` method
- Follow same pattern as ProjectService.CreateProject()

### Sample Resources

**K8s Secret (in build plane openchoreo-system):**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {secretName}-github-credentials
  namespace: openchoreo-system
type: kubernetes.io/basic-auth
data:
  password: <base64-encoded-token>
```

**PushSecret (in build plane openchoreo-system):**
```yaml
apiVersion: external-secrets.io/v1alpha1
kind: PushSecret
metadata:
  name: {secretName}-github-credentials
  namespace: openchoreo-system
spec:
  refreshInterval: 1m
  updatePolicy: Replace
  deletionPolicy: Delete
  secretStoreRefs:
    - kind: ClusterSecretStore
      name: vault  # From BuildPlane.Spec.SecretStoreRef
  selector:
    secret:
      name: {secretName}-github-credentials
  data:
    - match:
        secretKey: password
        remoteRef:
          remoteKey: {secretName}-github-credentials
          property: password
```

**SecretReference (in control plane namespace):**
```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: SecretReference
metadata:
  name: {secretName}-github-credentials
  namespace: {namespaceName}
spec:
  template:
    type: kubernetes.io/basic-auth
  data:
    - secretKey: password
      remoteRef:
        key: {secretName}-github-credentials
        property: password
  refreshInterval: 15m
```

## Implementation Steps

### Phase 1: Add Cluster Gateway Configuration

1. **Create `internal/openchoreo-api/config/cluster_gateway.go`**
   - Define `ClusterGatewayConfig` struct with URL, Enabled, TLS settings
   - Add validation and defaults

2. **Update `internal/openchoreo-api/config/config.go`**
   - Add `ClusterGateway ClusterGatewayConfig` field
   - Update `Defaults()` and `Validate()`

3. **Update `internal/openchoreo-api/services/services.go`**
   - Add gateway config parameter to `NewServices()`
   - Pass to BuildPlaneService

4. **Update `internal/openchoreo-api/services/buildplane_service.go`**
   - Add `gatewayURL` field to service struct
   - Update constructor to accept gatewayURL
   - Update `GetBuildPlaneClient()` to use stored URL

5. **Update `cmd/openchoreo-api/main.go`**
   - Pass `cfg.ClusterGateway` to `NewServices()`

6. **Update Helm templates**
   - `templates/openchoreo-api/configmap.yaml` - Add cluster_gateway section
   - `templates/openchoreo-api/deployment.yaml` - Mount CA certificate volume

### Phase 2: Add PushSecret Types

1. **Create `internal/dataplane/kubernetes/types/externalsecrets/v1alpha1/`**
   - `doc.go` - Package documentation
   - `pushsecret_types.go` - PushSecret CRD types
   - `register.go` - Scheme registration

2. **Update `internal/clients/kubernetes/client.go`**
   - Import and register v1alpha1 types with scheme

### Phase 3: Add Git Secret Service

1. **Create `internal/openchoreo-api/services/gitsecret_service.go`**
   - `GitSecretService` struct with k8sClient, buildPlaneService, logger
   - `CreateGitSecret(ctx, namespaceName, secretName, token)` method
   - Steps:
     a. Get BuildPlane from namespace
     b. Get build plane client via gateway
     c. Ensure `openchoreo-system` namespace exists in build plane
     d. Create K8s Secret in build plane
     e. Create PushSecret in build plane (using BuildPlane.Spec.SecretStoreRef)
     f. Create SecretReference in control plane namespace

2. **Create `internal/openchoreo-api/models/gitsecret.go`**
   - `CreateGitSecretRequest` struct
   - `GitSecretResponse` struct

3. **Update `internal/openchoreo-api/services/services.go`**
   - Add `GitSecretService` field
   - Initialize in `NewServices()`

### Phase 4: Add Handler and Route

1. **Create `internal/openchoreo-api/handlers/git_secrets.go`**
   - `CreateGitSecret(w, r)` handler
   - Extract namespaceName, parse body, call service, handle errors

2. **Update `internal/openchoreo-api/handlers/handlers.go`**
   - Add route: `api.HandleFunc("POST "+v1+"/namespaces/{namespaceName}/git-secrets", h.CreateGitSecret)`

### Phase 5: Add Authorization Constants

1. **Update `internal/openchoreo-api/services/constants.go`**
   - Add `SystemActionCreateGitSecret`
   - Add `ResourceTypeGitSecret`

2. **Update `internal/openchoreo-api/audit/definitions.go`**
   - Add audit action definition for git secret creation

## Critical Files

### To Create
- `internal/openchoreo-api/config/cluster_gateway.go`
- `internal/dataplane/kubernetes/types/externalsecrets/v1alpha1/doc.go`
- `internal/dataplane/kubernetes/types/externalsecrets/v1alpha1/pushsecret_types.go`
- `internal/dataplane/kubernetes/types/externalsecrets/v1alpha1/register.go`
- `internal/openchoreo-api/services/gitsecret_service.go`
- `internal/openchoreo-api/models/gitsecret.go`
- `internal/openchoreo-api/handlers/git_secrets.go`

### To Modify
- `internal/openchoreo-api/config/config.go`
- `internal/openchoreo-api/services/services.go`
- `internal/openchoreo-api/services/buildplane_service.go`
- `internal/openchoreo-api/handlers/handlers.go`
- `internal/clients/kubernetes/client.go`
- `cmd/openchoreo-api/main.go`
- `install/helm/openchoreo-control-plane/templates/openchoreo-api/configmap.yaml`
- `install/helm/openchoreo-control-plane/templates/openchoreo-api/deployment.yaml`
- `install/helm/openchoreo-control-plane/values.yaml` (add openchoreoApi.config.cluster_gateway)

## Verification Plan

1. **Unit Tests**
   - Test GitSecretService.CreateGitSecret() with mocked clients
   - Test request validation

2. **Integration Testing**
   - Deploy to local k3d cluster
   - Call API endpoint with valid token
   - Verify Secret created in build plane openchoreo-system
   - Verify PushSecret created in build plane openchoreo-system
   - Verify SecretReference created in control plane namespace

3. **Manual Testing**
   ```bash
   # Call the API
   curl -X POST "http://localhost:8080/api/v1/namespaces/default/git-secrets" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"secretName": "my-repo", "token": "ghp_xxx"}'

   # Verify resources in build plane
   kubectl get secret my-repo-github-credentials -n openchoreo-system
   kubectl get pushsecret my-repo-github-credentials -n openchoreo-system

   # Verify SecretReference in control plane
   kubectl get secretreference my-repo-github-credentials -n default
   ```

## Open Questions

1. Should the API validate the token format (e.g., GitHub PAT pattern)?
2. Should there be a delete endpoint to clean up all three resources?
3. How should errors be handled if partial creation fails (rollback)?
4. Should the secret store name come from BuildPlane.Spec.SecretStoreRef or be configurable?
