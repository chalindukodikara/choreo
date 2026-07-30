# Implementation: Add WorkflowPlane Reference to Git Secret API

## Summary

Replaced auto-discovery of workflow planes with explicit user-specified references, added ClusterWorkflowPlane support, and moved metadata from annotations to labels for filtering support.

## Files Modified

### 1. OpenAPI Spec — `openapi/openchoreo-api.yaml`

**CreateGitSecretRequest** schema:
- Added `workflowPlaneKind` (required, enum: `WorkflowPlane` | `ClusterWorkflowPlane`)
- Added `workflowPlaneName` (required, string)

**GitSecretResponse** schema:
- Added `workflowPlaneKind` (optional, string)
- Added `workflowPlaneName` (optional, string)

After these changes, `make openapi-codegen` was run to regenerate:
- `internal/openchoreo-api/api/gen/models.gen.go`
- `internal/openchoreo-api/api/gen/server.gen.go`
- `internal/openchoreo-api/api/gen/client.gen.go`

### 2. Service Interface — `internal/openchoreo-api/services/gitsecret/interface.go`

Added `WorkflowPlaneKind` and `WorkflowPlaneName` fields to both:
- `GitSecretInfo` — returned from List and Create operations
- `CreateGitSecretParams` — input to Create operation

### 3. Service Implementation — `internal/openchoreo-api/services/gitsecret/service.go`

This is the core of the change. Key modifications:

#### New Constants
```go
workflowPlaneKindLabel   = "openchoreo.dev/workflow-plane-kind"
workflowPlaneNameLabel   = "openchoreo.dev/workflow-plane-name"

workflowPlaneKindWorkflowPlane        = "WorkflowPlane"
workflowPlaneKindClusterWorkflowPlane = "ClusterWorkflowPlane"
```

#### Annotations → Labels Migration
- `gitSecretTypeAnnotation` → `gitSecretTypeLabel` (`openchoreo.dev/secret-type`)
- `gitSecretAuthTypeAnnotation` → `gitSecretAuthTypeLabel` (`kubernetes.io/secret-type`)

Both were previously stored as annotations on the SecretReference CR. They are now stored as labels, enabling server-side filtering via label selectors.

#### Removed: `getWorkflowPlane()`
The old method listed all `WorkflowPlane` resources in a namespace and picked the first one. This was the root cause of the three problems described in the task.

#### Added: `resolveWorkflowPlane()` + helpers
Replaced with a dispatch method that accepts `kind` and `name`:

```go
func (s *gitSecretService) resolveWorkflowPlane(ctx context.Context, namespaceName, kind, name string) (*workflowPlaneInfo, error)
```

Dispatches to:
- `resolveNamespacedWorkflowPlane()` — fetches `WorkflowPlane` by name in namespace, validates SecretStoreRef, returns client via `kubernetesClient.GetK8sClientFromWorkflowPlane()`
- `resolveClusterWorkflowPlane()` — fetches `ClusterWorkflowPlane` by name (cluster-scoped), validates SecretStoreRef, returns client via `kubernetesClient.GetK8sClientFromClusterWorkflowPlane()`

Both return a `workflowPlaneInfo` struct:
```go
type workflowPlaneInfo struct {
    client          client.Client
    secretStoreName string
}
```

#### Updated: `CreateGitSecret()`
- Uses `resolveWorkflowPlane()` with kind/name from request params instead of auto-discovery
- Uses `wpInfo.client` and `wpInfo.secretStoreName` throughout
- Passes `WorkflowPlaneKind` and `WorkflowPlaneName` to `buildSecretReference()`
- Returns workflow plane info in `GitSecretInfo` response

#### Updated: `DeleteGitSecret()`
- Reads `workflowPlaneKindLabel` and `workflowPlaneNameLabel` from SecretReference labels
- Returns error if labels are missing (guards against legacy SecretReferences without labels)
- Uses `resolveWorkflowPlane()` with the stored kind/name instead of auto-discovery

#### Updated: `buildSecretReference()`
- Accepts additional `wpKind` and `wpName` parameters
- All metadata now stored as labels (no annotations):
  - `openchoreo.dev/secret-type: git-credentials`
  - `kubernetes.io/secret-type: <basic-auth|ssh-auth>`
  - `openchoreo.dev/workflow-plane-kind: <WorkflowPlane|ClusterWorkflowPlane>`
  - `openchoreo.dev/workflow-plane-name: <name>`

#### Updated: `ListGitSecrets()`
- Filters by `ref.Labels[gitSecretTypeLabel]` instead of `ref.Annotations[...]`
- Populates `WorkflowPlaneKind` and `WorkflowPlaneName` from labels on each SecretReference

### 4. Handler — `internal/openchoreo-api/api/handlers/git_secrets.go`

#### `CreateGitSecret` handler:
- Maps `request.Body.WorkflowPlaneKind` and `request.Body.WorkflowPlaneName` to `CreateGitSecretParams`
- Maps `result.WorkflowPlaneKind` and `result.WorkflowPlaneName` to `GitSecretResponse`

#### `ListGitSecrets` handler:
- Maps `item.WorkflowPlaneKind` and `item.WorkflowPlaneName` to `GitSecretResponse`
- Only includes workflow plane fields in response if non-empty (backwards compatibility with old SecretReferences)

### 5. No Changes Required

- **`service_authz.go`** — Pure pass-through decorator, no logic changes needed
- **`errors.go`** — No new error types needed (reuses existing `ErrWorkflowPlaneNotFound`, `ErrSecretStoreNotConfigured`, and `services.ValidationError`)
- **`models/request.go`** — Legacy model only used by legacy service path, not the current handler

## Data Flow

### Create Flow (after changes)
```
POST /api/v1alpha1/namespaces/{ns}/gitsecrets
  { secretName, secretType, workflowPlaneKind, workflowPlaneName, ... }
    ↓
Handler: maps request fields → CreateGitSecretParams
    ↓
AuthZ wrapper: checks secretreference:create permission
    ↓
Service.CreateGitSecret():
  1. validateCredentials()
  2. Check SecretReference doesn't already exist
  3. resolveWorkflowPlane(kind, name) → workflowPlaneInfo{client, secretStoreName}
  4. ensureNamespaceExists(workflows-{ns})
  5. Apply K8s Secret in workflow plane (via wpInfo.client)
  6. Apply PushSecret in workflow plane (via wpInfo.client)
  7. Create SecretReference in control plane (with workflow plane labels)
    ↓
Handler: maps GitSecretInfo → GitSecretResponse (includes workflowPlaneKind/Name)
```

### Delete Flow (after changes)
```
DELETE /api/v1alpha1/namespaces/{ns}/gitsecrets/{name}
    ↓
Handler: passes namespace + name
    ↓
AuthZ wrapper: checks secretreference:delete permission
    ↓
Service.DeleteGitSecret():
  1. Get SecretReference from control plane
  2. Verify it's a git-credentials type (via label)
  3. Read workflowPlaneKind/Name from SecretReference labels
  4. resolveWorkflowPlane(kind, name) → workflowPlaneInfo{client, ...}
  5. Delete PushSecret from workflow plane (via wpInfo.client)
  6. Delete K8s Secret from workflow plane (via wpInfo.client)
  7. Delete SecretReference from control plane
```

### List Flow (after changes)
```
GET /api/v1alpha1/namespaces/{ns}/gitsecrets
    ↓
Service.ListGitSecrets():
  1. List all SecretReferences in namespace
  2. Filter by label openchoreo.dev/secret-type=git-credentials
  3. Read workflowPlaneKind/Name from labels on each
    ↓
Handler: maps each GitSecretInfo → GitSecretResponse (includes workflowPlane fields if present)
```

## Backwards Compatibility

- **Old SecretReferences** (created before this change) will not have the workflow plane labels. The List handler handles this gracefully by only including workflow plane fields in the response when non-empty.
- **Old SecretReferences** that used annotations for `openchoreo.dev/secret-type` will NOT be found by the new label-based filtering in List/Delete. If migration of existing secrets is needed, a one-time script to copy annotations to labels would be required.
- The Delete flow will return an error for old SecretReferences that lack workflow plane labels, since it can no longer auto-discover the workflow plane.

## Verification

All checks passed:
```
make openapi-codegen   ✅
make test              ✅
make lint-fix          ✅ (0 issues)
make go.build          ✅
```
