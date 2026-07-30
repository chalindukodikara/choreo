# OpenClaw on OpenChoreo — Article Task

## Goal

Publish a platform-engineer-facing article on running OpenClaw as a ChatOps surface for OpenChoreo (component creation, CI, CD). Before writing, stand up a working local instance so every claim, YAML snippet, and screenshot in the article comes from a real, reproducible setup.

## Audience Context

### 1. Platform Engineers (The Architects)
Builders and maintainers of this integration — the ones "behind the curtain."

- **What they do:** Deploy OpenChoreo, set up the OpenClaw agent on the Workflow Plane, and grant it secure, scoped API access.
- **Their goal:** Get out of the business of writing boilerplate YAML for developers and manually troubleshooting basic pipeline failures.
- **Why they care:** By setting up OpenClaw as a middleman, they enforce governance. The AI can only generate OpenChoreo-compliant configurations and operate within the explicit boundaries (Cells) the platform team has defined. They dictate the security guardrails, ensuring the AI can't accidentally nuke a production database.

### 2. Developers (The End Users)
Daily consumers of the integration — they shouldn't need to know how the underlying Kubernetes engine works.

- **What they do:** Interact with OpenClaw via Slack, Discord, or an internal portal. Use natural language to request infrastructure ("I need a new Redis cache connected to the User Service") or ask the AI why a build failed.
- **Their goal:** Ship code fast without learning infra tools or reading CI/CD logs.
- **Why they care:** Reduces cognitive load. No context-switching to write a Component manifest or parse a 500-line Jenkins/GHA error log — the AI handles the translation and triage.

---

## Pre-Article Work (do this first)

### Phase 1 — Local OpenChoreo instance

Following the public guide at https://openchoreo.dev/docs/getting-started/try-it-out/on-k3d-locally/ (release-v1.0 OCI charts, not local checkout). Control plane + data plane + workflow plane are required for CI/CD demos; observability plane skipped for this iteration.

- [x] Create k3d cluster: `k3d cluster create --config install/k3d/single-cluster/config.yaml` + seed machine-id on server node
- [x] Install prerequisites via `k3d-prerequisites.sh` (Gateway API v1.4.1, cert-manager v1.19.4, ESO 2.0.1, kgateway v2.2.1, OpenBao 0.25.6 with seeded dev secrets, ClusterSecretStore `default`, CoreDNS rewrite)
- [x] Install Thunder v0.28.0 (IdP) in `thunder` ns
- [x] Install `openchoreo-control-plane` v1.0.0 via OCI chart into `openchoreo-control-plane` ns (backstage ExternalSecret created first)
- [x] Apply default resources: `samples/getting-started/all.yaml` + label `default` ns as control plane (creates `default` Project, development/staging/production Environments, default DeploymentPipeline, service/worker/web-application/scheduled-task ClusterComponentTypes)
- [x] Install `openchoreo-data-plane` v1.0.0 + register `ClusterDataPlane/default`
- [x] Install `openchoreo-workflow-plane` v1.0.0 + twuni docker registry + shared workflow templates + register `ClusterWorkflowPlane/default`
- [x] Verified: all pods Ready in CP/DP/WP namespaces; `clusterdataplane/default` and `clusterworkflowplane/default` registered; cluster-agent logs show "connected to control plane"
- [x] Test scope: existing `default` namespace, default `default` Project, default `development` Environment — the cluster itself is throwaway (whole k3d cluster gets deleted via `k3d cluster delete openchoreo`)
- [x] MCP server confirmation moved to Phase 3 — `pkg/mcp/...` is built into `openchoreo-api` and exposed at `/mcp` (verified live; see Phase 3)

### Phase 2 — OpenClaw ComponentType

- [x] Read OpenClaw deployment surface from docs.openclaw.ai: Node 22.14+/24, Gateway listens on 18789, JSON config at `OPENCLAW_CONFIG_PATH`, state dir at `OPENCLAW_STATE_DIR`, channels configured via JSON `channels[]` with per-channel token env vars, model provider API key required
- [x] Drafted namespace-scoped `ComponentType/openclaw-gateway` at `samples/try-out-2/1-openclaw/openclaw-componenttype.yaml`:
  - `spec.workloadType: deployment`
  - `spec.parameters` (openAPIV3Schema) — gatewayPort, mcpServerURL, agentProvider (anthropic), agentModel (claude-opus-4-6 / sonnet / haiku), enabledChannels (slack/discord/telegram/teams/whatsapp/matrix/signal/imessage, `minItems: 1`)
  - `spec.environmentConfigs` — replicas (locked 1–1), imagePullPolicy, logLevel, resources, agentApiKeySecretKey, per-channel `channelSecrets.*` OpenBao keys
  - `spec.allowedTraits` — `ClusterTrait/observability-alert-rule` (the only trait available in default install)
  - `spec.allowedWorkflows: []` — OpenClaw is deployed from a pre-built image; no builder needed
  - `spec.validations` — at least one channel enabled; replicas == 1 (singleton gateway); workload has at least one endpoint
  - `spec.resources` — Deployment, Service, ConfigMap (renders `config.json` from parameters.enabledChannels), ExternalSecret (pulls agent API key + per-channel bot tokens from the ClusterSecretStore), HTTPRoute for external web-UI / webhook exposure
- [x] Kept the ComponentType **namespace-scoped** in `default` namespace
- [x] Webhook accepted the resource; controller logs "Validation for ComponentType upon creation" — no errors
- [x] Smoke test (`samples/try-out-2/1-openclaw/smoke-component.yaml`): Workload `openclaw` (port 18789, HTTP, external) + Component `openclaw` with `autoDeploy: true` and `parameters.enabledChannels: [slack, discord]`. Result:
  - `ComponentRelease/openclaw-655884856d` + `ReleaseBinding/openclaw-development` + `RenderedRelease/openclaw-development` all reconciled
  - Six rendered resources applied into `dp-default-default-development-f8e58905`: ConfigMap, Deployment, ExternalSecret, HTTPRoute, Service, NetworkPolicy
  - ConfigMap `openclaw-config` contains correctly rendered `config.json` (CEL evaluated `parameters.*` + `enabledChannels.map(...)` to `[{type:slack,tokenEnv:OPENCLAW_SLACK_TOKEN},{type:discord,tokenEnv:OPENCLAW_DISCORD_TOKEN}]`)
  - ExternalSecret `openclaw-gateway-channel-tokens` synced — 9 keys (1 agent API key + 8 channel tokens) pulled from OpenBao
  - Deployment is `ImagePullBackOff` on `ghcr.io/openclaw/openclaw:v0` — **expected**; smoke test was about CEL/schema/wiring, not image availability. Before the article we need a real OpenClaw image reference.
- [x] OpenBao dev secrets seeded for the CT's defaults: `openclaw-agent-api-key`, `openclaw-{slack,discord,telegram,teams,whatsapp,matrix,signal,imessage}-bot-token`

### Phase 3 — MCP integration contract (verified)

Agent doesn't talk to OpenChoreo — it talks to **MCP**. So before wiring real OpenClaw, we proved the MCP integration surface that OpenClaw will plug into.

- [x] Confirmed `openchoreo-api` exposes MCP at `/mcp` on port 8080 (`cmd/openchoreo-api/main.go:188-201`); enabled by default in `cmd/openchoreo-api/config.yaml:147` with toolsets `[namespace, project, component, deployment, build, pe]`
- [x] Auth contract: MCP rejects unauthenticated requests with `MISSING_TOKEN`. Bearer token issued by Thunder via `client_credentials` against `openchoreo-backstage-client` / `customer-portal-client` apps
- [x] Verified the full MCP session protocol works in-cluster (initialize → notifications/initialized → tools/list → tools/call)
- [x] `tools/list` returns the discovery surface OpenClaw's agent will use: `create_component`, `create_workload`, `list_*`, `get_*_schema`, etc. — exactly the verbs the article's three flows need
- [x] `tools/call list_namespaces` with the `customer-portal-client` JWT returns `{"namespaces":[]}` even though `default` exists — **proves MCP is gated by the same Casbin authz wrappers as the REST surface** (this is the article's governance thesis, validated)

### Phase 4 — Guardrails validation (verified)

Negative-test harness at `samples/try-out-2/1-openclaw/smoke-guardrails.yaml`. Both cases produced the expected failure:

- [x] **Case A (empty `enabledChannels`)**: blocked by `vcomponentrelease-v1alpha1.kb.io` admission webhook with `enabledChannels should have at least 1 items`. No ComponentRelease created. Schema-level guardrail (OpenAPI `minItems: 1`) fires at admission.
- [x] **Case B (workload with no endpoints)**: ComponentRelease created (no schema violation), but ReleaseBinding rendering failed with the exact CEL message: `rule[2] "${size(workload.endpoints) > 0}" evaluated to false: openclaw-gateway must declare an endpoint...`. No RenderedRelease produced. CEL-validation guardrail fires at render time.
- [x] Authz guardrail also covered above (Phase 3): unprivileged subject calling MCP gets empty result, not raw cluster data.

### Phase 5 — Out of scope until a real OpenClaw image exists

The remaining phases all need either a published OpenClaw container image or a live messaging workspace, neither of which is in scope for the local test round. Documenting them here as the **article's actionable follow-up**, not as gaps in the test:

- [ ] Pick or build a real OpenClaw image, replace `ghcr.io/openclaw/openclaw:v0` in `smoke-component.yaml`, confirm pod becomes Ready and the Web Control UI serves on the rendered HTTPRoute hostname
- [ ] Provision a Casbin policy granting an `openclaw-gateway` subject the minimum verbs (`component:create`, `workflow:trigger`, `releasebinding:create`) and re-run the MCP tool calls under that token to confirm the surface widens correctly
- [ ] Wire a private Slack/Discord workspace to the OpenClaw config, send the three demo prompts (component-create / CI-trigger / CD-promote), capture transcripts
- [ ] Egress NetworkPolicy refinement (the CT already produces an auto-generated NetworkPolicy; verify it permits MCP + messaging APIs and nothing else)

These are the *article's content*, not the *article's prerequisites*.

---

## Article Writing (after the setup works)

- [ ] Outline (platform-engineer audience; developer experience as a closing section)
- [ ] Draft sections: problem, why OpenClaw fits, why MCP is the other half, the wiring, three demos, guardrails, honest limits
- [ ] Pull YAML and chat transcripts from Phases 2–5 — do not invent snippets
- [ ] Internal review, then publish

## Out of Scope (for this iteration)

- Multi-cluster / multi-environment promotion flows
- Production messaging-platform workspaces
- `ClusterComponentType` / `ClusterTrait` variants
- Mobile-node (iOS/Android) pairing
- Performance / rate-limiting tuning
