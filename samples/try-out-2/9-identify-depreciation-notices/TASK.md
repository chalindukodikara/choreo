# Task: Early Warning for External-API & Dependency Deprecations

## Goal

OpenChoreo talks to, builds on, and ships a number of **external APIs, SDKs,
container images, Helm charts and SaaS services**. When any of those publish a
deprecation/sunset notice (a removed API version, a retired model, a sunset
image tag, a discontinued service), we want to find out **early** — ideally
before it breaks a build, an agent, or a control-plane reconcile — and get a
notice into a **Slack channel**.

Two parts:

1. **Inventory** — enumerate every external API / dependency OpenChoreo relies
   on, and for each one identify *how a deprecation is actually announced*
   (package version bump? HTTP `Sunset` header? a changelog/RSS feed? a
   K8s release note?). The right monitoring mechanism is different for each
   signal type, so the inventory must be grouped by signal, not just by vendor.
2. **Mechanism** — design and (incrementally) implement a way to watch those
   signals and push a notice to Slack, reusing what OpenChoreo already has
   wherever possible.

The original brief listed GitHub APIs, Claude/LLM APIs, and "anything else that
matters" across `agents/*`, `internal/*`, `cmd/*`, `pkg/*`,
`samples/getting-started/workflow-templates/*`. The findings below are the
result of actually walking those paths.

---

## Inventory (evaluation findings)

### A. External SaaS / REST APIs called at runtime

| Dependency | Where | How deprecations surface |
|------------|-------|--------------------------|
| **GitHub** — webhook payloads + REST | `internal/openchoreo-api/services/git/github.go` (webhook parse/verify); Backstage GitHub integration / API base URL, `…/api/v3` for GHES, default `https://api.github.com` (`install/helm/openchoreo-control-plane/values.yaml:~2595`) | `Sunset`/`Deprecation` response headers (RFC 8594); `X-GitHub-Api-Version` REST versioning; GitHub Changelog RSS (`https://github.blog/changelog/feed/`) |
| **GitLab** — webhook payloads | `internal/openchoreo-api/services/git/gitlab.go` | GitLab REST `/v4` versioning; release posts / deprecations page |
| **Bitbucket** — webhook payloads | `internal/openchoreo-api/services/git/bitbucket.go` | Bitbucket Cloud `/2.0` API; Atlassian changelog |
| **AWS CodeCommit** — git over SSH/HTTPS | `git-codecommit.*.amazonaws.com` handling in `samples/getting-started/workflow-templates/checkout-source.yaml` (key-id injection) and `internal/openchoreo-api/models/request.go` | **Already being sunset by AWS** (closed to new customers) — concrete real example of why this task exists. AWS service-health / what's-new feed |
| **LLM API (OpenAI-compatible)** — agents | `agents/finops-agent`, `agents/portal-assistant`, `agents/sre-agent` use `langchain-openai` / `ChatOpenAI`; GPT-5 / o-series models + `reasoning_effort` param (`agents/portal-assistant/src/config.py`, `main.py`) | Model retirement dates + API changelog. **Note:** if these are pointed at Anthropic/Claude or any other backend via an OpenAI-compatible base URL, that backend's model-deprecation schedule applies instead — confirm the actual `base_url`/`llm_name` per deployment before assuming OpenAI. |
| **OpenCost** — cost API | `finops-agent` → `http://opencost…:8081` (`agents/finops-agent/src/config.py:24`) | OpenCost is a bundled OSS project with an evolving HTTP API; watch its releases/changelog |
| **Observability backends** — query APIs | `internal/observer/*` queries OpenSearch (`_search` DSL) and Prometheus (`/api/v1/query`); observer/agents reach `observer:8080`, `opencost:8081` | OpenSearch / Prometheus query-API & client-library version notes |

> Reachability note: most of these are reached through OpenChoreo's own
> `internal/clients/*` (Go) or `httpx`/`langchain` (Python) — there is **no
> vendored `go-github`/`aws-sdk`/`slack-go`/`openai` SDK** in `go.mod`, so
> there is no SDK changelog to lean on for the Go side. Since we're staying
> config-only (no code), the signal for these live APIs is their published
> **changelog/RSS feed** (Layer 2), not in-code header inspection.

### B. External container images / registries (build + runtime)

Pulled by the build workflow templates and Helm charts; tags get sunset and
registries change pull policy / rate limits.

- `gcr.io/buildpacks/builder`, `gcr.io/buildpacks/google-22/run` (Google CNB)
- `docker.io/paketobuildpacks/builder-jammy-full`, `…/run-jammy-full` (Paketo)
- `docker.io/alpine/git`, `docker.io/mikefarah/yq`, `ghcr.io/jqlang/jq`
- `ghcr.io/openchoreo/*` (own images — out of scope but enumerated)
- **`ttl.sh`** ephemeral registry used by the cloud `publish-image` template —
  third-party, intentionally transient
- (source list: `grep -rhoE '(ghcr|docker|gcr)\.io/…' install/ config/ samples/getting-started/workflow-templates/`)

Builder/run images are **pinned by `@sha256:` digest** in the workflow
templates (e.g. `ballerina-buildpack-build.yaml`, `gcp-buildpacks-build.yaml`,
`paketo-buildpacks-build.yaml`) — a supply-chain contract. Deprecation signal =
upstream tag retired / `:latest` digest drifts away from the pinned one.

### C. External Helm charts / chart repos

From install docs and `.helpers.sh`:
`argoproj.github.io` (Argo Workflows), `twuni docker-registry.helm`,
`opensearch-project/opensearch-k8s-operator`, `project-zot`,
`ghcr.io/asgardeo/helm-charts/thunder`, `ghcr.io/external-secrets/charts`,
`ghcr.io/openbao/charts/openbao`. Deprecation signal = chart version
deprecation flag / repo relocation.

### D. Kubernetes & ecosystem API versions

OpenChoreo is a Kubebuilder operator: it reconciles CRDs and creates native
resources, and the build plane depends on **Argo Workflows** CRDs
(`argoproj.io`). K8s deprecates beta API groups on a fixed cadence. Deprecation
signal = K8s "Deprecated API Migration Guide" + `kubectl`/apiserver warnings;
tooling: **Pluto** / **kube-no-trouble (kubent)** scan manifests for
soon-removed APIs. The repo already runs scheduled scans
(`.github/workflows/periodic-trivy.yaml`, `codeql.yml`) — good precedent.

### E. Language / library APIs

Fast-moving libs whose own APIs deprecate: `langchain` (just reached v1),
`langchain-openai`, `mcp` SDK (note the explicit pin + comment in
`agents/sre-agent/pyproject.toml` about a 1.23 transport-security default
change — exactly the kind of breaking deprecation to catch early),
`kin-openapi`, `oapi-codegen`, `cel-go`, `casbin`, `controller-runtime`.
Signal = Go module / PyPI version + release notes.

---

## Mechanism — layered, by signal type

**This is config-only — no changes to OpenChoreo's own code.** We wire up
existing external tools and GitHub Actions to watch the signals and push to
Slack. No Go `RoundTripper`, no Python `httpx` hook, no in-cluster controller.
No single tool covers A–E, so combine two off-the-shelf layers, cheapest first.

### Layer 1 — Dependency bot (covers B, C, E; cheapest, highest coverage)

There is **no `dependabot.yml` or `renovate.json`** in the repo today. Add one
config file and the bot does the rest:
- **Renovate (recommended)** — one config understands Go modules, `uv`/PyPI,
  Dockerfiles, `@sha256` digest pins in arbitrary YAML (the workflow-template
  builder images), GitHub Actions, and Helm charts. It opens PRs for updates,
  flags **deprecated/abandoned packages** and digest drift, and can post a
  **Dependency Dashboard** + updates to Slack via its native Slack/webhook
  notification support (or the GitHub→Slack app).
- **Dependabot** — simpler, GitHub-native, but weaker on digest pins in
  arbitrary YAML and on "package is deprecated" signalling. Acceptable
  fallback.

This single file covers most of B/C/E with zero custom code.

### Layer 2 — Scheduled GitHub Action "deprecation scan" (covers A-feeds, D)

A new scheduled workflow that just runs existing CLI tools and posts to Slack —
mirror the structure of `.github/workflows/periodic-trivy.yaml` (already a
`schedule:`-triggered scan in this repo, so the pattern and any Slack secret
plumbing may already exist):
- **K8s / Argo API deprecations (D):** run **Pluto** or **kube-no-trouble
  (kubent)** against `config/crd/bases/` and rendered `install/helm/**`
  manifests to catch deprecated/removed API versions.
- **SaaS API changelogs (A):** watch the feeds for APIs that *don't* deprecate
  via a package bump, using an off-the-shelf RSS/changelog action (e.g. an
  RSS-to-Slack action, or a tiny `curl | diff` step) — GitHub Changelog RSS
  (`https://github.blog/changelog/feed/`), the **actual LLM backend's**
  deprecations page, Kubernetes release notes, AWS "what's new" for CodeCommit.
- **Slack delivery:** post a single digest via a **Slack incoming-webhook**
  (using a repo/org secret) — `slackapi/slack-github-action` or a `curl` to the
  webhook URL.

### Out of scope (deliberately not doing in code)

- Runtime `Deprecation`/`Sunset` response-header interception in
  `internal/clients/*` or the agents' `httpx` clients. It would be the earliest
  signal, but it requires touching OpenChoreo code — noted here only so the
  decision is explicit. The two external layers above are the chosen approach.
- Reusing the in-cluster `ObservabilityAlertsNotificationChannel` /
  `internal/observer/notifications/sender.go` path. It *can* deliver Slack
  (webhook channel with a `blocks` payload, per the CRD's own Slack example),
  but it's built for runtime alerts, not CI dependency scans. A Slack
  incoming-webhook secret in the GitHub Action is simpler and keeps this
  entirely outside the product. Mentioned only as the alternative.

---

## Files to add (all config / CI — no product code)

- `.github/renovate.json` (or `.github/dependabot.yml`) — Layer 1
- `.github/workflows/external-compatibility-scan.yaml` (new, `schedule:`-triggered) —
  Layer 2; mirror `.github/workflows/periodic-trivy.yaml`
- A Slack **incoming-webhook URL** stored as a GitHub Actions secret
  (e.g. `SLACK_EXTERNAL_COMPATIBILITY_WEBHOOK_URL`) — consumed by both layers
- A short `README.md` in this folder documenting the inventory + which
  tool/feed covers each row and who owns it

---

## Decisions to settle before building

1. **Bot choice:** Renovate (better deprecation + digest-pin coverage, external
   app) vs Dependabot (GitHub-native, simpler, weaker on YAML digest pins)?
2. **Which LLM backend is actually in use** per deployment (OpenAI vs
   Anthropic/Claude vs other via OpenAI-compatible `base_url`)? This decides
   *whose* model-deprecation feed Layer 2 watches. Confirm from the running
   config, not from the `langchain-openai` dependency name.
3. **Scope:** in-scope = third-party externals (A–E above). Out-of-scope =
   `ghcr.io/openchoreo/*` own images. Confirm.
4. **Bot output:** should Renovate/Dependabot open PRs/issues (actionable,
   trackable) *and* notify Slack, or Slack-only?
5. **Slack target:** one channel for everything, or split bot-PR noise (Layer 1)
   from the curated deprecation digest (Layer 2)?

---

## Deliverables / definition of done

- [ ] **Inventory committed** as a `README.md` in this folder: every external
      API/image/chart/lib (tables A–E), with its deprecation signal, the
      file(s) that use it, and which layer/tool covers it.
- [ ] Decisions 1–5 resolved.
- [ ] **Layer 1:** Renovate (or Dependabot) config covering Go modules, agent
      PyPI deps, Dockerfiles + `@sha256` digest pins in the workflow templates,
      GitHub Actions, and Helm charts — wired to Slack.
- [ ] **Layer 2:** scheduled GitHub Action running Pluto/kubent (K8s+Argo) and
      changelog/RSS watches (GitHub + the real LLM backend + K8s + AWS
      CodeCommit), posting a digest to Slack via incoming-webhook secret.
- [ ] **Slack delivery proven** end-to-end with at least one real notice (e.g.
      the AWS CodeCommit sunset, or a deliberately deprecated K8s API in a test
      manifest) landing in the target channel.
- [ ] Workflow runs green on schedule and is documented in the README so
      feed/owner maintenance is obvious.

## Open questions

- Do we treat `ttl.sh` (intentionally ephemeral) as a monitored dependency, or
  document it as out-of-contract?
- Should buildpack `@sha256:` digest drift be a *deprecation* signal or is that
  just Renovate's normal digest-update PRs?
- For GHES / self-hosted GitLab, public changelogs don't apply and we've ruled
  out runtime-header interception — is "no automated early signal there,
  rely on the bot + upstream release notes" acceptable?
