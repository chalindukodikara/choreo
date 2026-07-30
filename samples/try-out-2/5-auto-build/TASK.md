# Auto Build: Per-Namespace Webhook Secrets

## Problem

The current auto-build webhook endpoint uses a single global Kubernetes Secret
(`git-webhook-secrets` in `openchoreo-control-plane`) for all namespaces. This means:

- No tenant isolation — all namespaces share one secret per git provider
- A single webhook can trigger builds in any namespace
- Rotating the secret for one org affects everyone
- Component scan is cluster-wide (`client.List` with no namespace filter)

## Design

Add an optional `webhookSecretRef` query parameter to the autobuild endpoint:

```
POST /api/v1alpha1/autobuild?webhookSecretRef=<namespace>/<secretName>
```

- When `webhookSecretRef` is provided: look up the named Secret in the given namespace,
  validate the webhook signature against it, and scope the component scan to that
  namespace only.
- When `webhookSecretRef` is omitted: fall back to the current global-secret behavior
  (backward compatible).

### Why per-namespace (not per-component or per-project)

- Namespaces are the org/tenant trust boundary in OpenChoreo
- Git providers typically configure webhook secrets at the org level
- Per-component would require a separate webhook URL per component on the git side
- Per-namespace matches the Flux CD Receiver model and GitLab group-webhook model

## Files to Change

### 1. OpenAPI Spec — add `webhookSecretRef` query parameter

**File:** `openapi/openchoreo-api.yaml` (lines ~3718-3736)

Add a new optional query parameter to the `handleAutoBuild` operation:

```yaml
- in: query
  name: webhookSecretRef
  required: false
  schema:
    type: string
    pattern: '^[a-z0-9-]+/[a-z0-9-]+$'
  description: |
    Namespace-scoped secret reference in the format `<namespace>/<secretName>`.
    When provided, the webhook signature is validated against this specific
    Kubernetes Secret instead of the global default. The component scan is
    also scoped to the given namespace.
```

After editing, run `make openapi-codegen` to regenerate:
- `internal/openchoreo-api/api/gen/models.gen.go` — adds `WebhookSecretRef *string` to `HandleAutoBuildParams`
- `internal/openchoreo-api/api/gen/server.gen.go` — adds query param extraction

### 2. Webhook Handler — parse and forward `webhookSecretRef`

**File:** `internal/openchoreo-api/api/handlers/webhook_handler.go`

In `HandleAutoBuild()` (~line 86):
- Read `request.Params.WebhookSecretRef` (generated from OpenAPI)
- If present, parse into namespace + secretName (split on `/`, validate exactly 2 parts)
- Return 400 if the format is invalid
- Pass the parsed values into `ProcessWebhookParams`

### 3. Autobuild Service Interface — extend `ProcessWebhookParams`

**File:** `internal/openchoreo-api/services/autobuild/interface.go`

Add two optional fields to `ProcessWebhookParams`:

```go
type ProcessWebhookParams struct {
    ProviderType    git.ProviderType
    SignatureHeader string
    Signature       string
    SecretKey       string
    Payload         []byte
    // Per-namespace webhook secret override (both empty = use global default)
    WebhookSecretNamespace string
    WebhookSecretName      string
}
```

### 4. Autobuild Service — namespace-scoped secret lookup

**File:** `internal/openchoreo-api/services/autobuild/service.go`

Modify `ProcessWebhook()` (~line 49):
- If `params.WebhookSecretNamespace != ""` and `params.WebhookSecretName != ""`:
  - Call `getWebhookSecret` with the provided namespace/name instead of the
    hardcoded `webhookSecretName`/`webhookSecretNamespace`
  - Pass the scoped namespace to the processor
- Else: use existing global secret logic (no change)

Refactor `getWebhookSecret()` (~line 83) to accept namespace and name as arguments
instead of using the hardcoded constants:

```go
func (s *autobuildService) getWebhookSecret(ctx context.Context, namespace, name, secretKey string, allowEmpty bool) (string, error) {
```

The caller decides which namespace/name to pass based on whether `webhookSecretRef`
was provided.

### 5. Webhook Processor — namespace-scoped component listing

**File:** `internal/openchoreo-api/services/autobuild/webhook_processor.go`

Update the `WebhookProcessor` interface and `ProcessWebhook` method to accept an
optional namespace scope:

```go
type WebhookProcessor interface {
    ProcessWebhook(ctx context.Context, provider git.Provider, payload []byte, namespace string) ([]string, error)
}
```

In `findAffectedComponents()` (~line 117):
- If `namespace != ""`: use `client.InNamespace(namespace)` option in `List`
- Else: list across all namespaces (current behavior)

```go
listOpts := []client.ListOption{}
if namespace != "" {
    listOpts = append(listOpts, client.InNamespace(namespace))
}
if err := s.k8sClient.List(ctx, componentList, listOpts...); err != nil {
```

### 6. Update Tests

#### `service_test.go`

- Add test: `webhookSecretRef` provided — secret fetched from specified namespace/name
- Add test: `webhookSecretRef` provided but secret not found in namespace — returns error
- Add test: `webhookSecretRef` omitted — uses global default (existing behavior unchanged)
- Update `newWebhookSecret()` helper to accept namespace and name params
- Update `getWebhookSecret` call sites for new signature

#### `webhook_processor_test.go`

- Add test: namespace param scopes `List` call
- Add test: empty namespace lists all namespaces

#### `webhook_handler_test.go`

- Add test: valid `webhookSecretRef` format parsed correctly
- Add test: invalid `webhookSecretRef` format returns 400
- Add test: `webhookSecretRef` absent — backward compatible

### 7. Regenerate and Validate

```bash
make openapi-codegen   # Regenerate models + server from updated OpenAPI spec
make generate          # DeepCopy (if any type changes)
make manifests         # CRD regeneration (if needed)
make test              # Run unit + integration tests
make lint-fix          # Fix any lint issues
make go.build          # Verify build
```

## Task Checklist

- [ ] 1. Update `openapi/openchoreo-api.yaml` — add `webhookSecretRef` query param
- [ ] 2. Run `make openapi-codegen` — regenerate Go models/server
- [ ] 3. Update `ProcessWebhookParams` in `interface.go` — add `WebhookSecretNamespace`, `WebhookSecretName`
- [ ] 4. Update `service.go` — refactor `getWebhookSecret()` to accept ns/name, branch on params
- [ ] 5. Update `webhook_processor.go` — add `namespace` param, scope `List` call
- [ ] 6. Update `webhook_handler.go` — parse `secretRef`, validate format, pass to service
- [ ] 7. Update `service_test.go` — add per-namespace and backward-compat tests
- [ ] 8. Update `webhook_processor_test.go` — add namespace-scoped listing tests
- [ ] 9. Update `webhook_handler_test.go` — add `secretRef` parsing tests
- [ ] 10. Run `make test && make lint-fix && make go.build` — verify everything passes
