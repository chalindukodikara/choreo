# Task: Early Warning for External API and Dependency Deprecations

## Goal

OpenChoreo depends on external APIs, SaaS services, SDKs, container images,
Helm charts, Kubernetes APIs, and hosted registries. When one of those providers
publishes a deprecation, sunset, retirement, or breaking-change notice, we want
to know early enough to act before it breaks a build, an agent, an install, or a
control-plane/data-plane workflow.

Build an inventory and configure external tooling that sends actionable
deprecation notices to Slack. Prefer GitHub Actions, hosted dependency tooling,
RSS/feed watchers, vendor notification settings, and other external scanners.
OpenChoreo application/runtime code changes are out of scope for this task.

The original prompt called out GitHub APIs, Claude/LLM APIs, and "anything else
that matters" under the non-excluded parts of:

- `agents/*`
- `cmd/*`
- `internal/*`
- `internal/openchoreo-api/*`
- `internal/occ/*`
- `pkg/*`
- `samples/getting-started/workflow-templates/*`

Do not use findings from the excluded sample/website areas listed by the task
owner. If a dependency appears only there, leave it out of the inventory.

This task should use "deprecation" rather than "depreciation" throughout.

## Research Notes

Use these as starting points, not as the complete inventory:

- GitHub REST API versions are date-versioned. GitHub documents that closing
  API versions emit `Deprecation` and `Sunset` response headers before returning
  `410 Gone`, and currently lists `2026-03-10` and `2022-11-28` as supported
  REST API versions.
  Source: https://docs.github.com/en/rest/about-the-rest-api/api-versions
- GitHub documents breaking changes per REST API version. This is the feed to
  watch for versioned API impact.
  Source: https://docs.github.com/en/rest/about-the-rest-api/breaking-changes
- AWS CodeCommit is a concrete service-specific dependency in the repo. Monitor
  AWS documentation, AWS What's New, and AWS Service Health for availability,
  lifecycle, and retirement notices that could affect CodeCommit-backed sample
  or customer workflows.
  Source: https://docs.aws.amazon.com/codecommit/latest/userguide/welcome.html
- Kubernetes maintains a Deprecated API Migration Guide with removed API
  versions by release. As of the current guide, Kubernetes v1.32 stopped serving
  `flowcontrol.apiserver.k8s.io/v1beta3` for `FlowSchema` and
  `PriorityLevelConfiguration`.
  Source: https://kubernetes.io/docs/reference/using-api/deprecation-guide/
- OpenAI publishes model and endpoint deprecations, with documented minimum
  notice periods for model retirements.
  Source: https://developers.openai.com/api/docs/deprecations
- Anthropic publishes Claude model deprecations and platform release notes.
  Sources:
  - https://platform.claude.com/docs/en/about-claude/model-deprecations
  - https://platform.claude.com/docs/en/release-notes/overview
- Renovate supports Go modules, GitHub Actions, Docker, Helm, Kubernetes, and
  Python managers. It is a candidate for broader dependency/update coverage, but
  this repository already has Dependabot configured.
  Source: https://docs.renovatebot.com/modules/manager/

## Current Repo Findings

### Existing automation

OpenChoreo already has `.github/dependabot.yml` covering:

| Ecosystem | Existing coverage |
| --- | --- |
| Go modules | `/`, weekly |
| GitHub Actions | `/` and `/.github/actions/setup-go`, weekly |
| Dockerfiles | `/`, `/cmd/*`, `/install/*`, monthly |
| Helm charts | `/install/helm/*`, monthly |
| Python `uv` dependencies | `/agents/*`, weekly |

Do not propose "add dependency automation" as if none exists. The task is to
assess whether Dependabot coverage is sufficient, then either extend it, add
targeted scanners, or replace it with Renovate only if the extra coverage is
worth the operational cost.

Known likely gap: images embedded as shell variables or YAML strings in workflow
templates, for example `BUILDER="gcr.io/buildpacks/builder@sha256:..."`, may not
be picked up by Dependabot's Dockerfile ecosystem. Verify before deciding.

### External APIs and SaaS dependencies

| Dependency | Where found | Deprecation signal to monitor |
| --- | --- | --- |
| GitHub REST API direct calls | `samples/workflows/github-stats-report/cluster-workflow-template-github-stats-report.yaml` calls `GET https://api.github.com/repos/${ORG}/${REPO}`; `samples/workflows/scm-create-repo/github-create-repo.yaml` calls `POST https://api.github.com/orgs/${OWNER}/repos`; both set `X-GitHub-Api-Version: 2022-11-28` | GitHub REST version page, breaking changes page, changelog, and externally probed `Deprecation` / `Sunset` response headers |
| GitHub API base URL configuration | `install/helm/openchoreo-control-plane/values.yaml` documents Backstage's GitHub API base URL, including GHES `/api/v3`; `docs/integrations/github-actions.md` shows the same setting | Backstage GitHub integration release notes plus GitHub REST/GHES API version notices |
| AWS CodeCommit git/CLI usage | `samples/getting-started/workflow-templates/checkout-source.yaml` handles CodeCommit git remotes; `samples/workflows/scm-create-repo/codecommit-create-repo.yaml` calls `aws codecommit create-repository` through the AWS CLI; `internal/openchoreo-api/models/request.go` exposes `sshKeyId` for CodeCommit SSH auth | AWS service announcements, CodeCommit docs, AWS CLI compatibility notes, and Service Health |
| LLM provider APIs | `agents/finops-agent`, `agents/sre-agent`, `agents/portal-assistant`; `langchain.chat_models.init_chat_model`; `langchain-openai` dependency; configurable model names and API keys | Actual provider model-deprecation pages. Confirm per deployment whether the backend is OpenAI, Anthropic/Claude, or another OpenAI-compatible provider before choosing feeds. |
| OpenCost | `agents/finops-agent` config and prompts | OpenCost releases/changelog and any API compatibility notes |
| OpenChoreo internal APIs over HTTP | `agents/*/src/clients`, `internal/clients/gateway`, `internal/clients/kubernetes`, generated OpenAPI clients | Not the preferred implementation path for this task. Only document these as future in-code options if external monitoring cannot cover a signal. |
| Observability/query APIs | `internal/observer/service/*`; Prometheus-style query endpoints; logs/traces adapters | Prometheus/OpenSearch/adapter API changes and Helm chart/image changes |

### Inbound webhook contracts

These are not outbound API calls, but they are still external contracts. If a
provider changes webhook signatures, event names, headers, or payload shape,
OpenChoreo's auto-build path can break. Track them separately from direct API
calls.

| Dependency | Where found | Deprecation/change signal to monitor |
| --- | --- | --- |
| GitHub webhooks | `internal/openchoreo-api/services/git/github.go` validates `sha256=` signatures and parses push-style payload fields such as `ref`, `after`, `repository.clone_url`, and `commits` | GitHub webhook documentation and GitHub changelog for webhook signature/header/event payload changes |
| GitLab webhooks | `internal/openchoreo-api/services/git/gitlab.go` validates `X-Gitlab-Token` and parses push payload fields such as `ref`, `after`, `project.git_http_url`, and `commits` | GitLab webhook documentation, release posts, and deprecation notes |
| Bitbucket webhooks | `internal/openchoreo-api/services/git/bitbucket.go` parses push payload fields such as branch name, commit hash, and repository HTML link | Atlassian Bitbucket webhook documentation and changelog |

### Container images and registries

Inventory at least these image sources:

- `ghcr.io/openchoreo/*` images in Helm values and workflow templates. These are
  owned by OpenChoreo, so they should be documented but not treated as third
  party unless the registry itself is the concern.
- `ghcr.io/astral-sh/uv:python3.14-trixie-slim@sha256:...`
- `gcr.io/distroless/*@sha256:...`
- `alpine:3.23@sha256:...`
- `alpine/git:v2.52.0`
- `rancher/kubectl:v1.36.0`
- `rancher/k3s:v1.36.1-k3s1`
- `gcr.io/buildpacks/builder@sha256:...`
- `gcr.io/buildpacks/google-22/run@sha256:...`
- `docker.io/paketobuildpacks/builder-jammy-full@sha256:...`
- `docker.io/paketobuildpacks/run-jammy-full@sha256:...`
- `ghcr.io/openchoreo/buildpack/ballerina@sha256:...`
- `docker.io/buildpacksio/lifecycle@sha256:...`
- `ghcr.io/project-zot/zot`
- `registry:2`
- `ttl.sh/openchoreo-builds` in `publish-image.yaml`

The buildpack builder/run images are pinned by digest in workflow-template shell
variables. Their deprecation/drift signal is not just "package has a newer
version"; it is also "the upstream floating tag moved, the digest is no longer
published, or the builder stack/lifecycle is no longer supported".

### Helm charts and charts as dependency contracts

`install/helm/openchoreo-workflow-plane/Chart.yaml` depends on:

- `argo-workflows` from `https://argoproj.github.io/argo-helm`, currently
  version `1.0.14`

The other OpenChoreo charts currently do not declare chart dependencies in their
`Chart.yaml` files, but they embed image and app configuration in `values.yaml`.
Do not limit the inventory to `Chart.yaml`.

Install docs and helper scripts also reference external chart repositories and
OCI chart sources that should be included in the inventory because a repo move,
chart deprecation, or version retirement can break local setup and documented
install paths:

- cert-manager: `oci://quay.io/jetstack/charts/cert-manager`
- kgateway: `oci://cr.kgateway.dev/kgateway-dev/charts/kgateway-crds` and
  `oci://cr.kgateway.dev/kgateway-dev/charts/kgateway`
- Thunder: `oci://ghcr.io/asgardeo/helm-charts/thunder`
- External Secrets: `oci://ghcr.io/external-secrets/charts/external-secrets`
- OpenBao: `oci://ghcr.io/openbao/charts/openbao`
- Docker registry: `https://twuni.github.io/docker-registry.helm`
- Zot: `http://zotregistry.dev/helm-charts/`
- OpenSearch operator:
  `https://opensearch-project.github.io/opensearch-k8s-operator/`

### Kubernetes and ecosystem APIs

OpenChoreo is a Kubernetes operator and ships CRDs, Helm templates, generated
CRDs under `install/helm/**/crds`, and workflow templates that rely on Argo
Workflows. Kubernetes API removal is a first-class deprecation risk.

Inventory and scan:

- `config/crd/bases/*.yaml`
- `install/helm/**/templates/*.yaml`
- `install/helm/**/crds/*.yaml`
- `samples/getting-started/workflow-templates/*.yaml`
- `install/k3d/**/*.yaml`
- generated Argo Workflow API usage under `internal/dataplane/kubernetes/types`

Use `pluto` or `kubent` for Kubernetes API deprecation scanning. Where possible,
render Helm charts before scanning so templated manifests are checked, not just
source templates.

### Language and library APIs

Track fast-moving libraries whose own API deprecations can break OpenChoreo:

- Go: `controller-runtime`, `k8s.io/*`, `cilium`, `casbin`, `cel-go`,
  `kin-openapi`, `oapi-codegen`, `prometheus` client libraries.
- Python agents: `langchain`, `langchain-openai`, `httpx`, `mcp`, `pydantic`,
  `fastapi`, `uvicorn`, and transitive OpenAI-compatible client behavior.
  `agents/sre-agent/pyproject.toml` already has an explicit comment on the
  `mcp` SDK pin because earlier versions had a different
  `TransportSecuritySettings.enable_dns_rebinding_protection` default. Treat
  that as an example of the kind of SDK behavior change the dependency tooling
  should surface early.
- Generated clients: OpenAPI client generation and compatibility with the
  OpenChoreo API specs.

Dependabot already detects many version updates. The task is to add deprecation
awareness and Slack routing, not only version bump PRs.

No Go SDK dependency was found for GitHub, AWS, Slack, OpenAI, or Anthropic in
`go.mod`; do not assume Go-side SDK changelogs will cover those providers.
Prefer provider changelogs/RSS feeds and external probes for those signals.

## Proposed Mechanism

No single signal source covers all cases. Implement this in layers.

### Layer 1: Strengthen dependency automation

Start from the existing `.github/dependabot.yml`.

Evaluate:

- Does Dependabot detect all Dockerfiles currently in `cmd/*`, `agents/*`,
  `install/*`, and root-level files?
- Does it detect Docker image refs inside workflow-template YAML and shell
  variable assignments?
- Does it detect Helm chart dependencies and image values in Helm files?
- Does it detect `uv.lock` and `pyproject.toml` updates in all three agents?

If gaps are small, extend Dependabot or add a narrow scanner. If gaps are broad,
evaluate Renovate because it supports managers for Go modules, Docker, Helm,
Kubernetes YAML, GitHub Actions, and Python. Do not switch tools without writing
down what Renovate catches that the existing Dependabot setup misses.

Output for this layer:

- dependency automation coverage matrix
- PR/issue or Slack path for deprecated packages, retired image tags, stale
  digests, and unsupported chart versions

### Layer 2: Scheduled deprecation scan via GitHub Actions

Add a scheduled GitHub Action. This is the preferred implementation path because
the repo already has scheduled workflows such as
`.github/workflows/periodic-trivy.yaml`, and it avoids adding OpenChoreo runtime
code for a repository-maintenance concern.

The scan should:

- run `pluto` or `kubent` against rendered Helm manifests and static manifests
  to detect Kubernetes/Argo APIs that are deprecated or removed;
- rely on existing `test/*` e2e coverage for workflow-template image breakage;
  add proactive pinned-image/digest checks only if e2e feedback is not early
  enough;
- poll a small allowlist of provider feeds/pages for relevant deprecations:
  GitHub API versions/breaking changes, AWS CodeCommit, Kubernetes API
  deprecations, OpenAI or Anthropic based on the actual configured LLM backend,
  and OpenCost;
- persist the last-seen item IDs or hashes so Slack only receives new notices;
- post a compact digest to Slack with the affected dependency, source URL,
  affected OpenChoreo files, severity, and suggested owner/action.

This layer should not try to solve every changelog on day one. Start with the
highest risk and easiest sources:

1. Kubernetes deprecated API scan.
2. GitHub REST API version and breaking changes.
3. Actual LLM provider model-deprecation page.
4. AWS CodeCommit status/deprecation tracking.
5. Optional workflow-template pinned image digest drift if e2e-only detection
   is not sufficient.

### Layer 3: External response-header checks

Some providers announce deprecation on actual API responses. GitHub explicitly
documents `Deprecation` and `Sunset` headers for closing REST API versions.
Handle this with an external scheduled probe, not OpenChoreo application code.

Add a GitHub Action step or small standalone script that makes safe, low-volume
requests to known provider endpoints and detects:

- `Deprecation`
- `Sunset`
- `Warning`
- provider-specific headers where relevant

Implementation sketch:

- maintain a small YAML/JSON list of endpoints to probe, expected auth needs,
  owner, and affected OpenChoreo files;
- use `curl`, `gh api`, or a small script in the workflow to collect response
  headers;
- dedupe by provider, endpoint, header value, and date;
- route deduped findings into the same Slack delivery path used by Layer 2.

This layer is especially useful for GHES/self-hosted GitLab or provider-specific
OpenAI-compatible deployments where public changelog polling is incomplete.

## Slack Delivery

Prefer a direct Slack incoming webhook from GitHub Actions for the first
increment. It is simple, external to OpenChoreo runtime code, and independent of
a running control plane or observability plane.

OpenChoreo already has a webhook-capable notification channel:

- CRD: `config/crd/bases/openchoreo.dev_observabilityalertsnotificationchannels.yaml`
- sender: `internal/observer/notifications/sender.go`
- webhook helper: `internal/observer/notifications/webhook.go`
- sample: `config/samples/v1alpha1_observabilityalertsnotificationchannel.yaml`

The CRD supports `type: webhook` and a `payloadTemplate`; the CRD description
contains a Slack example. Slack is not a first-class channel type, but Slack
incoming webhooks work through the generic webhook channel.

Treat this existing OpenChoreo notification channel as a reference or optional
integration, not the default path for this task. Document the tradeoff:

- direct Slack webhook: simplest for GitHub Actions and independent of a running
  observability plane;
- OpenChoreo notification channel: one platform notification mechanism and
  consistent payload templating, but it may require a reachable OpenChoreo
  control/observability plane.

Recommended first choice: direct GitHub Actions secret such as
`SLACK_EXTERNAL_COMPATIBILITY_WEBHOOK_URL`.

## Deliverables

- [x] Inventory document committed as `README.md` in this task directory.
      It must list each external API, SaaS dependency, image, registry, chart,
      Kubernetes API surface, and fast-moving library, with:
      dependency name, owner/team, source files, deprecation signal, monitoring
      mechanism, severity, and recommended action.
- [x] Existing `.github/dependabot.yml` assessed with a clear coverage matrix.
      Include whether it catches workflow-template image refs and digest-pinned
      images outside Dockerfiles, and note the existing e2e safety net.
- [x] Decision recorded: keep Dependabot, extend Dependabot with auxiliary
      scripts, or migrate/add Renovate. Include the reason.
- [ ] Scheduled external-compatibility-scan GitHub Actions workflow added, starting with Kubernetes API
      scans and provider-page/feed checks for GitHub, the actual LLM provider,
      and AWS CodeCommit. Pinned buildpack/image drift checks are optional
      unless e2e feedback is not early enough.
- [ ] Slack delivery implemented and documented. At least one test notice must
      be sent successfully, using a non-secret test webhook or a mocked receiver.
- [ ] External `Deprecation` / `Sunset` header probe added to the scheduled
      workflow for at least one provider endpoint, or explicitly deferred with
      rationale.
- [ ] Dedupe and severity rules documented so Slack does not become noisy.
- [ ] CI job runs green and has a manual `workflow_dispatch` path for testing.

## Acceptance Criteria

- The inventory is evidence-based, with file paths and source URLs for each
  dependency class.
- The implementation uses external tooling or GitHub Actions and does not add
  OpenChoreo application/runtime code.
- The scan catches at least one known real signal, for example:
  - an AWS CodeCommit lifecycle or availability notice;
  - a Kubernetes API removed in a target Kubernetes version;
  - a known LLM model retirement from the configured provider;
  - optionally, a pinned workflow image whose upstream tag digest has moved.
- Slack messages include: dependency, affected files, source link, deadline or
  sunset date if known, severity, and suggested next action.
- Header-probe tests or mocked workflow checks prove that `Deprecation` and
  `Sunset` response headers are detected and deduped.

## Open Questions

- Confirm the target Slack channel and GitHub Actions secret name for the direct
  Slack webhook.
- Which LLM provider is actually configured in current deployments: OpenAI,
  Anthropic/Claude, or another OpenAI-compatible backend?
- Should `ttl.sh/openchoreo-builds` be treated as a monitored dependency or
  explicitly documented as an intentionally ephemeral development registry?
- Are OpenChoreo-owned images under `ghcr.io/openchoreo/*` in scope for this
  task, or should the first pass focus only on third-party providers?
- Should deprecation findings open GitHub issues/PRs in addition to posting to
  Slack?
