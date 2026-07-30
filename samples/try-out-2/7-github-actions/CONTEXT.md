# GitHub Actions External CI — Investigation Context

Context dump of an end-to-end investigation into wiring **GitHub Actions** as an
external CI provider for the OpenChoreo Backstage portal.

- **PR under review:** [#3641](https://github.com/openchoreo/openchoreo/pull/3641)
  — `feat(helm): add GitHub Actions external CI integration for Backstage`
- **Tracking issue:** #3551 (this is "PR A"; PR B is the OIDC/workflow bridge)
- **Follow-up to:** #1788 (the original Jenkins wiring this mirrors)

## TL;DR

PR #3641 wires the **Helm/Backstage config** for GitHub Actions (a `githubActions`
values block, `integrations.github` in the CI ConfigMap, and `GITHUB_HOST` /
`GITHUB_API_BASE_URL` / `GITHUB_TOKEN` env vars). The chart renders correctly and
the schema is in sync.

**However, PR #3641 alone does NOT make the GitHub Actions tab functional.**
The frontend GitHub Actions card authenticates via **per-user GitHub OAuth**
(`ScmAuth`), not via the PAT the PR provisions. Without a configured GitHub
**OAuth provider** (`auth.providers.github`), the card fails with:

```
NotFoundError: No auth provider registered for 'github'   (HTTP 404 at /api/auth/github/start)
```

The PR's claim *"the env vars wired here are exactly what that UI will read"* is
**incorrect** for the Actions card: from `integrations.github` the card reads only
`apiBaseUrl` (for GHES) and uses `host` to match — the `token` is ignored by the
frontend plugin.

---

## What PR #3641 actually changes (9 files)

| File | Change |
|---|---|
| `install/helm/openchoreo-control-plane/templates/backstage/configmap-ci.yaml` | Adds `integrations.github` block (conditional on `githubActions.enabled`; `apiBaseUrl` line rendered only when set) |
| `.../templates/backstage/deployment.yaml` | Injects `GITHUB_HOST`, `GITHUB_API_BASE_URL`, `GITHUB_TOKEN` env (token from Secret key `github-actions-token`, `optional: true`) |
| `.../values.yaml` + `values.schema.json` | New `backstage.externalCI.githubActions.{enabled,host,apiBaseUrl}` block |
| `install/k3d/{single,multi}-cluster/README.md` | Add `github-actions-token` to the `backstage-secrets` creation step |
| `install/k3d/common/values-openbao.yaml`, `install/prerequisites/openbao/setup.sh` | Seed `secret/backstage-github-actions-token` placeholder |
| `docs/integrations/github-actions.md` | New doc (token setup, Helm activation, GHES, annotation, status matrix) |

Verified: `helm template` renders correctly for public-GitHub (no `apiBaseUrl`
line), GHES (with `apiBaseUrl`), and disabled (no `integrations` block / env
vars). `make helm-generate.openchoreo-control-plane` produces **no diff** — the
schema is genuinely in sync.

---

## The core finding: two different GitHub token paths

There are **two independent** GitHub token mechanisms in Backstage. The PR wires
path 1; the Actions card needs path 2.

### Path 1 — `integrations.github[].token` (the `GITHUB_TOKEN` PAT)
- Read by **backend** plugins only: catalog ingestion, scaffolder git ops, TechDocs
  (via `@backstage/integration`'s `ScmIntegrationRegistry`).
- Deliberately **never shipped to the browser**.
- This is what PR #3641 + the `github-actions-token` Secret provision.

### Path 2 — `ScmAuth` → per-user OAuth (what the Actions **card** uses)
Proven from the installed package sources:

`@backstage-community/plugin-github-actions/dist/api/GithubActionsClient.esm.js`:
```js
async getOctokit(hostname = "github.com") {
  const { token } = await this.scmAuthApi.getCredentials({          // ← OAuth, not config
    url: `https://${hostname}/`,
    additionalScope: { customScopes: { github: ["repo"] } },
  });
  const configs = readGithubIntegrationConfigs(
    this.configApi.getOptionalConfigArray("integrations.github") ?? []);
  const baseUrl = configs.find(v => v.host === hostname)?.apiBaseUrl; // ← ONLY apiBaseUrl from config
  return new Octokit({ auth: token, baseUrl });
}
```

`@backstage/integration-react/.../ScmAuth.ts` (`getCredentials`):
```js
const token = await this.#api.getAccessToken(uniqueScopes, restOptions);  // #api = githubAuthApiRef (OAuth)
return { token, headers: { Authorization: `Bearer ${token}` } };
```
`ScmAuth` has **no access to config** — it cannot read `integrations.github.token`.
`ScmAuth.createDefaultApiFactory()` wires `ScmAuth.forGithub(githubAuthApiRef)`,
so a github.com request triggers `githubAuthApi.getAccessToken(['repo','read:org','read:user'])`
→ a redirect to the backend GitHub OAuth provider.

**Conclusion:** the card's token is per-user OAuth. The PAT is irrelevant to it.

---

## UI side: already wired, no `.tsx` changes needed

In the `backstage-plugins` repo, GitHub Actions support is already present:

- `packages/app/src/components/catalog/WorkflowsOrExternalCICard.tsx:19,47-51` —
  `github.com/project-slug` annotation gate renders `EntityRecentGithubActionsRunsCard`.
- `packages/app/src/components/catalog/EntityPage.tsx:150,187-188,402-404,497-499` —
  imports `EntityGithubActionsContent`, defines `hasGithubActionsAnnotation`, mounts the tab.
- `packages/app/package.json:22` — `@backstage-community/plugin-github-actions@^0.20.0`.
- `packages/app/src/apis.ts:103` — `ScmAuth.createDefaultApiFactory()`.

So the card **renders** the moment a Component has `github.com/project-slug`. It
just can't **fetch** until the OAuth provider exists.

---

## The actual blocker: GitHub OAuth provider not configured

The Backstage backend **does** register the provider module:

`packages/backend/src/index.ts:59-60`
```ts
// Github provider
backend.add(import('@backstage/plugin-auth-backend-module-github-provider'));
```

But the module only instantiates a provider when `auth.providers.github.<env>`
exists in app-config — and **no such block exists** in any `app-config*.yaml`
(only `openchoreo-auth` and `guest` are configured). Hence the 404.

The failing request also shows `env=development`, so the config must live under a
`development:` block (matching `auth.environment`).

### Fix (spans backstage-plugins repo + chart + GitHub)

1. **Create a GitHub OAuth App** (GitHub → Settings → Developer settings → OAuth Apps):
   - Homepage URL: `http://openchoreo.localhost:8080`
   - Authorization callback URL: `http://openchoreo.localhost:8080/api/auth/github/handler/frame`
   - Capture Client ID + Client secret.

2. **Add provider config** in `backstage-plugins/app-config.production.yaml` under
   `auth.providers:`:
   ```yaml
   auth:
     environment: development
     providers:
       github:
         development:
           clientId: ${AUTH_GITHUB_CLIENT_ID}
           clientSecret: ${AUTH_GITHUB_CLIENT_SECRET}
   ```

3. **Inject `AUTH_GITHUB_CLIENT_ID` / `AUTH_GITHUB_CLIENT_SECRET`** into the
   Backstage deployment (chart `deployment.yaml`), sourced from `backstage-secrets`
   (OpenBao → ExternalSecret → Secret), then `kubectl rollout restart` Backstage.

> This is **outside PR #3641's scope** and confirms the integration is not
> end-to-end functional as merged.

---

## Operational gotchas hit during testing

### 1. ExternalSecret key names must match exactly
The Backstage Deployment reads K8s Secret key **`github-actions-token`**
(`deployment.yaml:178`), and the OpenBao seed path is
**`backstage-github-actions-token`** (`values-openbao.yaml:53`, `setup.sh:164`).

Correct ExternalSecret entry (NOT `github-token` / `backstage-github-token`):
```yaml
  - secretKey: github-actions-token
    remoteRef:
      key: backstage-github-actions-token
      property: value
```
Because the Deployment marks the key `optional: true`, a wrong `secretKey` fails
**silently** (pod boots, `GITHUB_TOKEN` empty). A wrong `remoteRef.key` fails
loudly (ExternalSecret `SecretSyncedError`).

### 2. Seeding the PAT into OpenBao
```bash
read -rs GH_PAT   # paste; not echoed, not in shell history
kubectl exec -n openbao openbao-0 -- sh -c '
  export BAO_ADDR=http://127.0.0.1:8200 BAO_TOKEN=root
  bao kv put secret/backstage-github-actions-token value="'"$GH_PAT"'"'
unset GH_PAT
```
Then force the ExternalSecret to re-sync (don't wait for `refreshInterval: 1h`)
and restart Backstage (env vars from `secretKeyRef` are read at container start,
they do not hot-reload):
```bash
kubectl annotate externalsecret backstage-secrets -n openchoreo-control-plane \
  force-sync=$(date +%s) --overwrite
kubectl rollout restart deployment -n openchoreo-control-plane \
  -l app.kubernetes.io/component=backstage
```
> Never paste a real PAT inline on the command line — it lands in shell history
> and in `kubectl exec` process args (visible via `ps`/audit logs). Rotate any
> token exposed that way.

### 3. `helm upgrade --reuse-values` breaks on newly added chart values
Enabling the integration on an existing release with `--reuse-values` throws:
```
nil pointer evaluating interface {}.enabled
  at <.Values.backstage.externalCI.githubActions.enabled>
```
`--reuse-values` replays only the previously-supplied values and does **not**
merge newly-added chart defaults (the `githubActions` block didn't exist at the
prior release). Use `--reset-then-reuse-values` (Helm ≥ 3.14) so current chart
defaults — including `host: github.com` — are merged before overrides:
```bash
helm upgrade --install openchoreo-control-plane install/helm/openchoreo-control-plane \
  --namespace openchoreo-control-plane \
  --reset-then-reuse-values \
  --set backstage.externalCI.githubActions.enabled=true
```
(Watch for missing line-continuation `\` — a dropped backslash makes the shell
run `--set …` as a separate command: `zsh: command not found: --set`.)

> The activation snippets in `docs/integrations/github-actions.md` use a bare
> `--set` with no value-preservation flag; run verbatim against a live release
> they reset all other values to defaults (placeholder `.invalid` domains → the
> chart's placeholder-domain guard fails). The doc should add
> `--reset-then-reuse-values`.

---

## Recommendations for PR #3641

1. **Correct the premise:** the Actions card reads `apiBaseUrl` from
   `integrations.github`, not `token` / `GITHUB_TOKEN`. Update the PR description
   and the doc accordingly.
2. **Document the OAuth-App prerequisite** in `docs/integrations/github-actions.md`
   (or explicitly defer the "card lights up" claim to PR B). As written the doc
   implies the PAT alone surfaces runs — it does not.
3. **Fix the helm activation snippets** to use `--reset-then-reuse-values`.
4. **Close the missed secret path:** `install/quick-start/.helpers.sh:910-914`
   also creates `backstage-secrets` and was NOT updated with
   `github-actions-token` (the two READMEs and both OpenBao seeds were). Optional
   key, so non-breaking, but inconsistent.
5. Minor doc nits: classic-PAT scopes (`repo`,`actions:read`) vs fine-grained
   (`Actions: Read`,`Metadata: Read`) are mixed; the `…/integrations/github/locations`
   link is about catalog locations, not CI credentials.

### What is actually correct in the PR
- Chart renders for all three states; schema regenerates with no diff.
- `enabled=false` default + `optional: true` Secret key preserve backward compat.
- Conditional `apiBaseUrl` rendering is the right call for public GitHub.
- The "merge collision" worry is **not** real: production loads
  `app-config.production.yaml` (no `integrations` block) + `app-config.ci.yaml`;
  local dev doesn't load `app-config.ci.yaml`. The two `integrations.github`
  blocks never co-exist.

---

## VERIFIED END-TO-END: the OAuth-provider fix works (and what it takes)

The core finding above was confirmed empirically on a live k3d cluster. Registering
a `github` auth provider is **the** missing piece — once present, the Actions card
authenticates via per-user OAuth and renders runs (including for private repos).

### Sanity test (provider registration only)
Patched the `backstage-ci-config` ConfigMap (feeds `app-config.ci.yaml`, loaded after the
baked-in `app-config.yaml`) to add `auth.providers.github.development` with **dummy**
creds, restarted Backstage. `GET /api/auth/github/start` progression proved the route
goes from absent → registered:

| Request | Before | After provider added |
|---|---|---|
| `/api/auth/github/start` | `404 No auth provider registered for 'github'` | `400 Must specify 'env' query` (route now exists) |
| `/api/auth/github/start?env=development` | `404` | `302` → `https://github.com/login/oauth/authorize?...&redirect_uri=http%3A%2F%2Fopenchoreo.localhost%3A8080%2Fapi%2Fauth%2Fgithub%2Fhandler%2Fframe&...&client_id=...` |

The `redirect_uri` in the 302 is exactly the OAuth-App callback URL — confirming the
required callback is `http://openchoreo.localhost:8080/api/auth/github/handler/frame`.

### Full test (real OAuth App)
With a real GitHub OAuth App (callback set to the URL above) and its client id/secret in
the provider block, the card worked end-to-end after a one-time browser GitHub login.
Test target: **private** repo `chalindu-kodikara/ao-travels` (16 workflow runs). Private
repos work because `GithubActionsClient.getOctokit()` requests `customScopes.github:["repo"]`,
so the per-user OAuth token can read private-repo runs the user has access to. The Backstage
**catalog entity** (not the OpenChoreo Component CR) carried
`github.com/project-slug: chalindu-kodikara/ao-travels`; the card reads the annotation off
the entity.

> The browser OAuth handshake cannot be completed headlessly — full verification needs a
> human to click through the GitHub login once. Everything up to the 302 is scriptable.

### The complete wiring chain (all four links required)
Putting creds in the Secret is necessary but **not sufficient**. The chain is:

```
OpenBao KV  →  ExternalSecret  →  K8s Secret keys  →  Deployment env vars  →  app-config provider block
(seed value)   (data mapping)     (github-oauth-*)     (AUTH_GITHUB_*)         (auth.providers.github)
```

If any link is missing, `/api/auth/github/start` stays `404`. Observed failure mode when a
user seeded OpenBao + mapped the ExternalSecret but stopped there:
- ✅ `backstage-secrets` had `github-oauth-client-id` / `github-oauth-client-secret`, ExternalSecret `SecretSynced`.
- ❌ Deployment had **no** `AUTH_GITHUB_CLIENT_ID` / `AUTH_GITHUB_CLIENT_SECRET` env (only `GITHUB_HOST/API_BASE_URL/TOKEN`).
- ❌ app-config had no `auth.providers.github` (the `integrations.github` block present is the unrelated PAT path).
- Result: still `404`. The chain breaks **after** the Secret.

### Gotcha: `backstage-secrets` is ExternalSecret-owned (`creationPolicy: Owner`)
On the OpenBao-backed install, `backstage-secrets` is managed by an ExternalSecret with
`creationPolicy: Owner`, so **`kubectl patch secret backstage-secrets` is overwritten on
the next sync.** Correct path:
1. Seed OpenBao: `bao kv put secret/backstage-github-oauth-client-id value=...` and
   `secret/backstage-github-oauth-client-secret value=...` (convention: `secret/backstage-<secretKey>`, value under `value` property).
2. Append two entries to the ExternalSecret `spec.data` (JSON patch appends; run once):
   ```bash
   kubectl patch externalsecret backstage-secrets -n openchoreo-control-plane --type=json -p='[
     {"op":"add","path":"/spec/data/-","value":{"secretKey":"github-oauth-client-id","remoteRef":{"key":"backstage-github-oauth-client-id","property":"value"}}},
     {"op":"add","path":"/spec/data/-","value":{"secretKey":"github-oauth-client-secret","remoteRef":{"key":"backstage-github-oauth-client-secret","property":"value"}}}
   ]'
   ```
3. `kubectl annotate externalsecret backstage-secrets -n openchoreo-control-plane force-sync=$(date +%s) --overwrite`

### Manual unblock today (reverts on `helm upgrade`)
**A. Inject env into the Deployment** (append once; duplicates if re-run):
```bash
kubectl patch deploy backstage -n openchoreo-control-plane --type=json -p='[
  {"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"AUTH_GITHUB_CLIENT_ID","valueFrom":{"secretKeyRef":{"name":"backstage-secrets","key":"github-oauth-client-id"}}}},
  {"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"AUTH_GITHUB_CLIENT_SECRET","valueFrom":{"secretKeyRef":{"name":"backstage-secrets","key":"github-oauth-client-secret"}}}}
]'
```
**B. Add the provider block** to the `backstage-ci-config` ConfigMap (`auth.providers.github.development`
with `clientId: ${AUTH_GITHUB_CLIENT_ID}` / `clientSecret: ${AUTH_GITHUB_CLIENT_SECRET}`).
Patch A triggers the rollout that also picks up B. Verify the `302` afterward (the image
has no `curl`/`wget` — use the bundled `node`).

### Still missing from chart + README (PR-B of #3551 scope)
None of the OAuth-provider wiring ships today. To make it install-native and
upgrade-safe:
- `install/helm/openchoreo-control-plane/templates/backstage/deployment.yaml` — render
  `AUTH_GITHUB_CLIENT_ID` / `AUTH_GITHUB_CLIENT_SECRET` env (from `backstage-secrets`),
  gated on `githubActions.enabled`.
- `backstage-plugins` `app-config.yaml` + `app-config.production.yaml` — add the
  `auth.providers.github.<env>` block (env key matches `auth.environment`, default `development`).
- `install/prerequisites/openbao/setup.sh` + `install/k3d/common/values-openbao.yaml` —
  seed `backstage-github-oauth-client-id` / `-client-secret` placeholders.
- `install/k3d/{single,multi}-cluster/README.md` — add the two `github-oauth-*` entries to
  the `backstage-secrets` ExternalSecret block (mirroring `github-actions-token`).

### Doc status
`docs/integrations/github-actions.md` was updated this session with: the two-credential
clarification, a new **section 4 "Enable per-user GitHub sign-in"** (OAuth App + provider
block + OpenBao/ExternalSecret cred storage + verification), the catalog-entity annotation
note, an updated "what ships" row, and troubleshooting for the `404` / `redirect_uri`
mismatch / private-repo cases. It documents the OAuth provider as **manual / PR-B-pending**.
