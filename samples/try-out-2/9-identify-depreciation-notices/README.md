# External Compatibility Notice Monitoring

This document records the refined scope and tooling decision from
`TASK.md`.

The goal is not to duplicate Dependabot or e2e test coverage. The goal is to
get early, actionable notices when an external API or integration contract that
OpenChoreo depends on is being deprecated, sunset, retired, or changed in a way
that can break us and needs lead time to fix.

Scope excludes findings that exist only under:

- `samples/private-registry/*`
- `samples/private-repo-registry/*`
- `samples/try-out/*`
- `samples/try-out-2/*`
- `website/*`

## Scope Decision

Use Slack external compatibility notices for high-signal external API and
contract changes.

Do not use Slack external compatibility notices for routine dependency, chart,
or image version drift. Those are already handled by Dependabot, e2e tests, or
normal release/test workflows.

### In Scope for Compatibility Notices

| Area | Why it matters | Signal to monitor |
| --- | --- | --- |
| GitHub REST API usage | Workflow templates make direct GitHub REST API calls and pin `X-GitHub-Api-Version`. API version sunset can break those workflows. | GitHub REST API `/versions`, breaking changes page, changelog, and optional `Deprecation` / `Sunset` header probes. |
| GitHub, GitLab, and Bitbucket webhooks | These are inbound, not outbound API calls, but they are still external contracts. Header, signature, event, or payload changes can break auto-build flows. | Provider webhook docs, changelogs, release notes, and lifecycle notices. |
| AWS CodeCommit git/CLI usage | OpenChoreo has CodeCommit-specific workflow and request-model paths. Service lifecycle, CLI, or auth changes can affect those flows. | AWS CodeCommit docs, AWS What's New, AWS CLI notes, and AWS Service Health. |
| LLM provider APIs and models | Agents use OpenAI-compatible LangChain integrations, but model names are user/deployment-provided rather than OpenChoreo-managed defaults. | Do not send repo-level Slack notices for user-selected model retirements. Treat model replacement as user/operator responsibility unless OpenChoreo later ships a platform-managed default model. |
| Kubernetes API removals | OpenChoreo ships CRDs, Helm charts, and Kubernetes manifests. Removed API versions can break installs or upgrades. | Lightweight static scan for Kubernetes API versions already removed in supported releases, plus current Helm/e2e coverage. Add rendered `pluto` or `kubent` scans if templating or upgrade planning exposes a gap. |
| External CRD/API contracts used directly | Argo Workflows, External Secrets, Gateway API/kgateway, OpenCost, OpenSearch, Prometheus, and similar APIs can break generated types, manifests, or query behavior. | Project release notes, lifecycle notices, and targeted compatibility scans where available. |

### Out of Scope for Slack Compatibility Notices

| Area | Existing coverage | Decision |
| --- | --- | --- |
| Go modules | Dependabot `gomod` at `/`, weekly. | Keep Dependabot. Do not create Slack notices for routine updates. |
| Python agent dependencies | Dependabot `uv` at `/agents/*`, weekly. | Keep Dependabot. Use Slack only for provider/API retirements or unusually high-risk SDK behavior changes. |
| GitHub Actions versions | Dependabot `github-actions` at `/` and `/.github/actions/setup-go`, weekly. | Keep Dependabot. |
| Helm chart dependencies declared in charts | Dependabot `helm` at `/install/helm/*`, monthly. | Keep Dependabot. Use Slack only for chart repo moves, chart deprecation, or CRD/API breaking changes. |
| Root, `cmd/*`, `install/*`, and `agents/*` Dockerfiles | Dependabot `docker` at `/`, `/cmd/*`, `/install/*`, and `/agents/*`, monthly. | Keep Dependabot. `/agents/*` was the only required Docker coverage gap because those Dockerfiles build released agent images. |
| Workflow-template image breakage | Covered when relevant `test/*` e2e workflows run. | Do not add Slack notices for normal image drift. Add proactive image checks only if e2e feedback is too late. |
| OpenChoreo-owned images | Owned by this project. | Treat as release/test responsibility, not third-party deprecation monitoring. |
| User-selected agent model names | Configured by users/operators, not owned by OpenChoreo. | Do not monitor through repo-level Slack notices. Users should replace retired models in their own configuration. |

## Repo Findings

### Existing Dependabot Coverage

Current config: `.github/dependabot.yml`.

| Surface | Current config | Status |
| --- | --- | --- |
| Go modules | `gomod` at `/`, weekly | Covered |
| GitHub Actions | `github-actions` at `/` and `/.github/actions/setup-go`, weekly | Covered |
| Root Dockerfile | `docker` at `/`, monthly | Covered |
| `cmd/*/Dockerfile` | `docker` at `/cmd/*`, monthly | Covered |
| `install/*` Dockerfiles | `docker` at `/install/*`, monthly | Covered for one-level install Dockerfile dirs |
| `agents/*/Dockerfile` | `docker` at `/agents/*`, monthly | Covered; required because `sre-agent`, `finops-agent`, and `portal-assistant` are built, published, scanned, and released |
| Helm chart dependencies | `helm` at `/install/helm/*`, monthly | Covered for declared chart dependencies |
| Python `uv` dependencies | `uv` at `/agents/*`, weekly | Covered |
| `tools/licenser/go.mod` | No separate Dependabot entry | Intentionally not covered; CI tooling dependency surface, mostly overlapping with the root module, and not required for this task |
| `test/ui/package-lock.json` | No npm Dependabot entry | Intentionally not covered; active e2e/release-smoke tooling, but not product runtime or released image dependency surface |
| Workflow-template image refs | Exercised by `test/*` e2e flows | Covered as breakage detection, not early compatibility notice |

Action from this audit: add `/agents/*` to the Docker section in
`.github/dependabot.yml`. Do not add low-signal coverage for `tools/licenser`,
`test/ui`, sample-only manifests, or e2e fixture Dockerfiles. Do not replace
Dependabot with Renovate for the first implementation.

### Direct External API Usage

| Dependency | Evidence | Monitoring decision |
| --- | --- | --- |
| GitHub REST API | `samples/workflows/github-stats-report/cluster-workflow-template-github-stats-report.yaml`; `samples/workflows/scm-create-repo/github-create-repo.yaml` | Monitor GitHub REST API `/versions`, breaking changes, changelog, and optional deprecation/sunset response headers. |
| GitHub API base URL / GHES config | `install/helm/openchoreo-control-plane/values.yaml`; `docs/integrations/github-actions.md` | Monitor GitHub and GHES API compatibility notes where relevant. |
| AWS CodeCommit | `samples/getting-started/workflow-templates/checkout-source.yaml`; `samples/workflows/scm-create-repo/codecommit-create-repo.yaml`; `internal/openchoreo-api/models/request.go` | Monitor AWS lifecycle, availability, CLI, and auth notices. |
| LLM provider APIs | `agents/finops-agent`; `agents/sre-agent`; `agents/portal-assistant`; `langchain-openai` dependency | Do not monitor user-selected model retirements at repo level. Continue relying on Dependabot/tests for SDK behavior changes; add provider API monitoring only if OpenChoreo owns a concrete provider contract or default. |
| OpenCost | `agents/finops-agent/src/config.py`; `agents/finops-agent/src/clients/mcp.py` | Monitor OpenCost release/lifecycle notes if API compatibility changes can affect the agent. |
| Observability query APIs | `internal/observer/service/*`; agent MCP clients | Monitor Prometheus/OpenSearch/query adapter breaking changes where they affect query behavior. |

### Inbound Webhook Contracts

These are not outbound API calls. They still remain in scope because OpenChoreo
parses provider-owned request headers and payloads.

| Dependency | Evidence | Monitoring decision |
| --- | --- | --- |
| GitHub webhooks | `internal/openchoreo-api/services/git/github.go` | Monitor GitHub webhook signature, header, event, and push payload contract changes. |
| GitLab webhooks | `internal/openchoreo-api/services/git/gitlab.go` | Monitor GitLab webhook token/header and push payload changes. |
| Bitbucket webhooks | `internal/openchoreo-api/services/git/bitbucket.go` | Monitor Bitbucket push payload and event behavior changes. |

### Kubernetes and External CRD/API Contracts

| Dependency | Evidence | Monitoring decision |
| --- | --- | --- |
| Kubernetes APIs | `config/crd/bases/*.yaml`; `install/helm/**/templates/*.yaml`; `install/helm/**/crds/*.yaml` | Add a lightweight static removed-API scan. Do not add `pluto` or `kubent` for the first pass; revisit during future Kubernetes upgrade planning or if templated manifests stop being covered by e2e. |
| Argo Workflows APIs | `install/helm/openchoreo-workflow-plane`; workflow templates; generated Argo types | Monitor Argo releases and CRD/API compatibility. |
| External Secrets APIs | generated types and git secret integration code | Monitor External Secrets releases and CRD/API compatibility. |
| Gateway API/kgateway APIs | Helm templates, samples, and install configuration | Monitor Gateway API/kgateway release and lifecycle notes. |
| OpenCost, OpenSearch, Prometheus | agent and observer query paths | Monitor only breaking API/query behavior changes, not routine chart/image updates. |

## Implemented Approach

1. Keep Dependabot as the dependency update mechanism.
2. Use Docker coverage for `/agents/*` in `.github/dependabot.yml`; this is the
   only required Dependabot coverage change from the audit.
3. Run one scheduled GitHub Action for curated external compatibility and
   breaking-change signals.
4. Run a lightweight static Kubernetes removed-API scan. Do not add `pluto` or
   `kubent` in the first pass because rendered chart/e2e coverage already
   exercises the current Kubernetes 1.36 target; revisit if upgrade planning
   identifies a gap.
5. Poll a small allowlist of provider pages/feeds for external API and contract
   compatibility notices:
   - GitHub REST API versions and breaking changes.
   - GitHub, GitLab, and Bitbucket webhook contract changes.
   - AWS CodeCommit lifecycle, availability, CLI, and auth notices.
   - Provider API lifecycle notices only when OpenChoreo owns a concrete provider
     contract or default; user-selected agent model retirements are out of scope.
   - Argo, External Secrets, Gateway API/kgateway, OpenCost, OpenSearch, and
     Prometheus breaking-change notices where directly relevant.
6. Persist last-seen notice IDs or content hashes so Slack receives only new
   notices.
7. Send a compact Slack digest through a GitHub Actions secret such as
   `SLACK_EXTERNAL_COMPATIBILITY_WEBHOOK_URL`.

## Slack Notice Criteria

Send a Slack notice only when all of these are true:

- The source is an external provider, upstream project, or hosted service.
- The notice describes deprecation, sunset, removal, retirement, or a breaking
  contract/API change.
- OpenChoreo has a known usage path affected by the notice.
- The team likely needs lead time to migrate, test, or change configuration.

Do not send Slack notices for:

- routine patch/minor dependency updates;
- normal Docker image refreshes;
- normal Helm chart bumps;
- Dependabot PR noise;
- e2e failures that are already surfaced by CI;
- broad changelog entries with no likely OpenChoreo impact.

## Implemented Files

The implementation consists of:

- an updated `.github/dependabot.yml` with `/agents/*` Docker coverage;
- `.github/workflows/external-compatibility-scan.yaml`;
- `.github/workflows/external-compatibility-scan-tests.yaml`;
- `.github/external-compatibility-sources.json` as the monitored source allowlist;
- `.github/scripts/external_compatibility_scan.py` for page/feed/header scanning,
  first-run baselining, dedupe, and Slack payload generation;
- `.github/scripts/test_external_compatibility_scan.py`;
- Slack digest delivery using `SLACK_EXTERNAL_COMPATIBILITY_WEBHOOK_URL`;
- `.github/external-compatibility-sources.md` documentation for maintaining the
  monitored source list.

## Implemented Scanner Behavior

- The workflow runs weekly on Monday at 03:10 UTC and can also be started
  manually.
- RSS/Atom scans check every item returned by the feed. There is no 50-item
  cutoff. HTTP response bodies are limited to 10 MiB to keep processing
  bounded.
- The first run records matching historical notices as a baseline. Later runs
  notify only for findings that are not already in notification history.
- A source-health alert starts after two consecutive failed scans. Further
  alerts are raised at 3, 7, 14, and 30 failures, then every 30 failures. A
  successful scan resets the consecutive-failure count.
- Each failure incident after recovery gets a new generation in its finding ID.
  This prevents a later identical incident from being hidden by the earlier
  incident's notification history.
- If Slack has not received a source-health alert and the source recovers, that
  stale pending failure alert is removed. Other pending compatibility notices
  remain queued until Slack delivery succeeds.
- Observed and notified history are each capped at 2,000 records, keeping the
  most recently observed entries. General pending notices are not silently
  dropped; the workflow reports a warning when more than 200 are pending.
- Slack messages contain at most 10 findings each. State and report artifacts
  are retained for 90 days.
- The scanner test workflow validates the source configuration and runs the
  Python unit tests for matching, state handling, Slack payloads, source-health
  incidents, feed coverage, and Kubernetes API scanning.

## Completed Decisions

- Dependabot was evaluated against the repository layout and active build,
  release, scan, and test paths.
- The only required Dependabot coverage change is `/agents/*` Docker coverage.
- `tools/licenser` Go module coverage was intentionally not added because it is
  CI tooling, most dependencies overlap with the root Go module, and it is not a
  product/runtime dependency surface.
- `test/ui` npm coverage was intentionally not added because it is e2e and
  release-smoke tooling, not a runtime or released image dependency surface.
- `pluto`/`kubent` was intentionally not added for the current Helm charts. A
  lightweight static scan catches API versions already removed in supported
  Kubernetes releases; install/e2e coverage still exercises rendered manifests.
- External compatibility notice capture is implemented as a scheduled/manual GitHub Action
  that scans curated RSS/Atom feeds, selected HTTP response headers,
  machine-readable lifecycle APIs, GitHub REST API versions, and static
  Kubernetes manifest API versions. The implementation
  does not create Slack findings from generic documentation-page keyword
  snippets; reference pages are health-only. The first run baselines historical
  matches, later runs dedupe against the previous successful run's
  `external-compatibility-scan-state` artifact, Slack payloads are chunked
  before delivery, and Slack posting is activated by adding the
  `SLACK_EXTERNAL_COMPATIBILITY_WEBHOOK_URL` secret. Source failures are
  reported after repeated failures so monitoring gaps do not silently hide
  notices without alerting on a single transient outage. Recovered and repeated
  failure incidents, pending delivery, bounded history, and full-feed scanning
  follow the behavior documented above.
