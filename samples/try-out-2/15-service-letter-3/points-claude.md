# Service Letter — Point-by-Point Validation

Evidence for each bullet in the letter, for manual verification. Every PR/issue/file below was checked against GitHub (via `gh` as `chalindukodikara`) or the local `openchoreo` repo on 2026-07-26.

**Status legend**
- ✅ **Verified** — direct evidence found.
- ⚠️ **Partly verified / reword** — core is supported but a detail is imprecise, self-reported, or lives in a repo I couldn't fully enumerate. Check the note.
- ❌ **Incorrect / unsupported** — evidence contradicts the claim as written.
- 📄 **Not GitHub-verifiable** — employment/narrative fact (HR record or research not tracked in git).

**Repo shorthand:** OC = `openchoreo/openchoreo` · CS = `wso2-enterprise/choreo-cp-configuration-service` · DA = `wso2-enterprise/choreo-cp-declarative-api` · CC = `wso2-enterprise/choreo-console` · CM = `openchoreo/community-modules` · BP = `openchoreo/backstage-plugins` · SW = `openchoreo/sample-workloads` · CHO = `wso2-enterprise/choreo` (issue tracker)

---

## ⚠️ Read first — the flags that matter

1. **❌ OpenChoreo "initial Project, Component, Deployment controllers".** In OC core, only the **Build controller** is Chalindu's (first commit `internal/controller/build`, 2025-01-17). `internal/controller/project` was first added by **Miraj Abeysekara**, `internal/controller/component` by **shirok**. → His Project/Component/Environment/Deployment controller work belongs to the **pre-Kubernetes V3 (DA repo)**, where it is verified. Reword the OC-core bullet to "initial Build controller" and keep project/component/deployment controllers under the Declarative API / V3 section.
2. **⚠️ Choreo Console "build-time configurations / Build and Configure feature".** The 17 CC PRs are all **deploy-time** config (deploy-page side panels, direct deploy, endpoint drawer) — not a build-time "Build and Configure" feature. Reword to "deploy-time configuration."
3. **⚠️ "Generic workflows: database migrations, infrastructure provisioning."** These are examples of what the generic Workflow Plane *can* run — no concrete DB-migration/infra-provisioning workflow ships. Keep as "designed to run arbitrary workflows such as …," not as delivered features.
4. **⚠️ Research PostgreSQL 50M-records / 1,000-users / <1s.** Self-reported (research, not in public git). The related task is CHO #27425; the exact numbers can't be confirmed from GitHub.
5. **⚠️ Ballerina→Go migration & V3 Argo build controller.** Attributed in the source "areas-worked-on" docs to DA PRs (#203/#204/#272, #316, #373, #381, #346, #386, #409). I confirmed the `v3-api-definition`/`v3-revision1` branches exist and contain his controller + PostgreSQL commits, but did **not** individually re-confirm each cited PR — verify on the DA repo.
6. **⚠️ Config-service rollout artifacts** (manifests, overlays, mounts) live in `choreo-control-plane` and `choreo-cp-env-overlay`, which I did not enumerate here — verify there.

---

## Header & Intro

- **Employment dates / titles (SE 2023-07-15→2025-07-31, Sr SE 2025-08-01→2026-07-31).** 📄 HR record — not GitHub-verifiable. First public activity aligns: CC PRs from Aug 2023, CS from Sep 2023.
- **Intro summary paragraph.** ✅ Umbrella for the sections below; each capability is evidenced later.

---

## WSO2 Developer Platform — Choreo

### Configuration Management

- **Designed & implemented the Choreo Configuration Service.** ✅ CS repo, 61 PRs starting CS#2 "Add yaml files required for the creation of the source repo pipeline" (Sep 2023).
- **CRUD + search for config groups & values.** ✅ CS#41 (edit groups via POST), CS#53 (update logic + DB), CS#58 (search folded into get-groups), CS#69 (store/retrieve empty configs), CS#106 (PUT update), CS#44 (unordered value list).
- **Secret Manager + Azure Key Vault integration (store/retrieve/remove).** ⚠️ CS#12 ("Integrate secret manager for config storage in KV and reference retrieval"), CS#18 (value references), CS#71 (remove configs from Key Vault on delete). *"Key Vault" confirmed; the "Azure" specificity comes from the source docs, not the PR titles.*
- **Validation, source-type, update tracking, correlation IDs, error handling, concurrency.** ✅ Validation CS#37/#50/#66/#78; source-type CS#76 (+ CHO#25100); update-time triggers CS#112; correlation IDs CS#52/#89/#94; error handling CS#33/#60/#72; concurrency CS#85.
- **Rollout via DB scripts, K8s manifests, overlays, mounts, release pipelines.** ⚠️ DB script CS#13; release pipeline CS#5/#7/#79/#117. *Manifests/overlays/mounts are in `choreo-control-plane` & `choreo-cp-env-overlay` (per source docs: control-plane #7251/#7310, env-overlay #9402/#9460/#9461) — verify in those repos.*
- **Owned/maintained until moving to OpenChoreo.** ✅ Soft claim; CS activity spans CS#2→CS#117 (Sep 2023–May 2024).

### Choreo Console and Build Configuration

- **React/TypeScript changes to Choreo Console.** ✅ CC, 17 PRs (CC#7830…#8678).
- **UX for "build-time configurations" via "Build and Configure feature".** ⚠️ **Reword.** Evidence is **deploy-time** config: CC#8014/#8015 (config side panel after deploying), CC#7907 (direct deploy). Supporting issues are Deploy-page: CHO#25752, CHO#25878. No "build-time"/"Build and Configure" feature appears in the PRs.
- **Direct deploy, deploy side panels, endpoint info, proxy/BYOI deploy.** ✅ CC#7907 (direct deploy), CC#7983 (proxy-with-policy), CC#7976 (BYOI side panel), CC#7864 (endpoint URLs in drawer), CC#8576 (removed auto-trigger on select).
- **Fixed config-name, project-creation, component-refresh, dialog, design-system issues.** ✅ CC#8609 (duplicate config names), CC#7872/#7838 (project name), CC#7941 (component list refresh), CC#8023 (role dialog overlap), CC#8619 (KeyValueCard states).

### Declarative API and Choreo V3 Control Plane

- **Contributed significantly to the Declarative API.** ✅ DA, 128 PRs.
- **Resources: Project/Component/Build/Deployment/ComponentConfig/Environment/EnvTemplate.** ✅ Component DA#71/#74/#108/#119/#130; Build DA#57/#58; ComponentConfig DA#72/#102/#121; Deployment DA#63; MI DA#70. Environment/EnvTemplate on V3 branches (DA#244 env-update / CHO#31477; env-template per source docs DA#192).
- **Storage layer, hierarchical metadata, resource revisions, idempotency, concurrency-safe creation.** ⚠️ Storage layer DA#107 (source docs) + V3-branch PostgreSQL commits ("Refine postgres update method", "resource revision"), DA#239 (resource revision). *Hierarchical metadata (CHO#262) and idempotency/unique-key (CHO#30982/#31156/#30750) are cited in source docs — verify those issues.*
- **Project/component/environment/env-template/deployment/build controllers.** ✅ (pre-k8s V3) V3-branch commits: project controller DA#241/#243, env update DA#244; source docs add DA#206/#217 (project), #199/#207/#209 (env), #220–#222 (component), #167 (deployment), #253 (build).
- **Migrating controllers Ballerina→Go + K8s reconciliation patterns.** ⚠️ Source docs cite DA#203/#204/#272 (+ periodic sync #288). V3 branches I inspected are Ballerina; the Go-migration PRs were **not** independently re-confirmed — verify on DA.
- **Build controller on Argo (buildpacks, image caching, org namespaces, concurrent builds, auto-deploy).** ⚠️ Source docs cite DA#316 (Argo build controller), #373 (multi-arch), #381 (org namespaces), #346 (image caching), #386 (concurrent builds), #409 (auto-deploy). Not re-confirmed individually — verify on DA.

### Customer Support and Platform Reliability

- **Customer-support rotation; enterprise issues.** ✅ CHO: 126 closed assigned issues. Named escalations: CHO#36771 [Brighton], CHO#37120 [Colorcon], CHO#37242 [Holmesglen].
- **Production incidents, restore, root cause.** ✅ CHO#36879 [INC0033876 Azure Monitor git-bot], CHO#37120, CHO#37242.
- **Logs, deployments, build failures, execution logs, private endpoints, runtime errors.** ✅ CHO#37242 (execution logs), CHO#36771 (private endpoint exposed), CHO#37120 (zero-status runtime), CHO#28243 (deploy 500s), CHO#26762 (404 build endpoint).

---

## Research and Early Platform Prototyping

- **Researched a reconciliation-based (K8s-inspired) architecture; resource/controller/persistence models.** 📄/⚠️ Research is not in public git; the prototype is the DA `v3-api-definition` / `v3-revision1` branches (both confirmed to exist with his commits).
- **Evaluated declarative resource management & controller patterns for the platform.** 📄 Narrative — not independently verifiable.
- **PostgreSQL perf testing on Azure VMs, ~50M records, up to 1,000 concurrent users.** ⚠️ Self-reported (related task CHO#27425 "PostgreSQL volume test"); exact figures not confirmable from GitHub.
- **DB scalability targeting <1s search response.** ⚠️ Self-reported goal — same as above.
- **Early prototype with custom YAML model + in-house controller runtime.** ✅ DA branches `v3-api-definition` (last commit "Add initial bal mock services for the groups") and `v3-revision1`.
- **Data-handling + controller-runtime before the K8s-native move.** ✅ V3-branch commits: reconcile loops, resource-revision updates, PostgreSQL update methods.

---

## OpenChoreo

- **Initiated Jan 2025; bootstrap contributor; maintainer through 1.2.x.** ✅/⚠️ First OC PRs Jan 2025 (build controller). "1.2.x" is forward-looking/self-reported. (This version does **not** claim "highest contributor" — good.)

### Core Platform and Multi-Cluster Architecture

- **Initial K8s-native implementation (Kubebuilder + controller-runtime).** ✅ OC#271 (initial build controller), OC#40 (default resources); `internal/controller/build` first commit is his (2025-01-17).
- **Initial Project, Component, Deployment, and Build controllers.** ❌ **Only Build is his in OC core.** `internal/controller/project` first-added by **Miraj Abeysekara**, `internal/controller/component` by **shirok**. Reword to "initial Build controller," and attribute Project/Component/Deployment controllers to the pre-k8s V3 work (verified above).
- **CRs, status conditions, finalizers, reconciliation, lifecycle.** ✅ OC#141 (finalizer), #149 (conditions), #157 (nil-pointer guard), #181 (requeue), #182 (resource deletion), #383 (condition priority), #2657 (immutable ref), #1961 (TTL). *(Mostly build/workflow CRs.)*
- **Initial direct control-plane ↔ data-plane communication.** ✅ OC#185 "Add Initial Multi-Cluster Deployment Support through Direct Access."
- **Multi-cluster execution + multiple K8s auth methods.** ✅ OC#414 (multiple K8s auth methods), OC#236 (any DP as build plane), OC#185.
- **BuildPlane abstraction → WorkflowPlane.** ✅ OC#247/#249/#251 (Build Plane proposal + impl), OC#2574 ("rename buildplane to workflowplane"); proposal file `docs/proposals/0245-introduce-build-plane.md`.

### Workflow Plane, CI and Generic Workflows

- **Owned CI/workflow area end to end.** ✅ CI epic CHO/OC #615 (per source docs); large body of CI PRs below.
- **Initial CI model → generic Workflow Plane.** ✅ OC#271/#305 (build + APIs), #708/#779 (schema-driven), #2164 (merge workflows), #2574 (workflowplane).
- **Workflows for builds, DB migrations, infra provisioning, automation.** ⚠️ Builds ✅; **DB migrations / infra provisioning are capability examples, not shipped workflows.** Reword to "designed to run arbitrary workflows such as …".
- **Workflow/WorkflowRun resources & CRUD/observe APIs.** ✅ OC#2242 (workflow + workflow-run APIs), #1280 (workflow-run APIs), #2296 (list query param), #3062 (deletion API), #2325 (event/log APIs).
- **Logs, events, step statuses, metadata, admission validation, finalizers, TTL.** ✅ OC#2325 (event/log), #1633 (steps in status), #2847 (admission webhooks), #2329 (deletion on component finalize), #1961 (TTL).
- **Schema-driven definitions; unified component + generic workflows.** ✅ OC#708 (proposal), #779 (impl), #2164 ("merge component workflows and workflows").

### Build Technologies and Source Integrations

- **Argo-based CI: Dockerfile + Cloud Native Buildpacks.** ✅ OC#318 (redesign templates), #303 (buildpacks), #1704 (docker workflow for web apps).
- **Ballerina, React, PHP, web apps, multi-language.** ✅ OC#2 (Ballerina), #89/#90 (PHP), #303 (React/Ballerina), #1704 (web apps).
- **Google Cloud + Paketo + Ballerina buildpacks; multi-arch images.** ✅ Builders present in `install/k3d/build-cache/README.md` (`gcp-buildpacks-build`, `paketo-buildpacks-build`, `ballerina-buildpack-build`); multi-arch OC#4083.
- **Private Git repos via PAT and SSH.** ✅ Both confirmed — `CreateGitSecretRequest` validates `basic-auth`/`ssh-auth`, `kubernetes.io/ssh-auth`; OC#1469/#1114/#1773 (+ CHO#1112).
- **AWS CodeCommit + private/external registries.** ✅ OC#1821 (CodeCommit), #213/#1538 (external registries push), #253 (external registry pull), #337 (configurable endpoint).
- **Automated workload creation via CI, optional for generic workflows.** ✅ OC#348 (automate), #1936 (make optional), #2469 (via API server).

### Secret Management, Security and Build Reliability

- **OpenBao as the open-source key vault.** ✅ `install/k3d/common/values-openbao.yaml`, `test/e2e/k3d/openbao-secretstore.yaml`, OC#1815 (openbao ES service account). Integrated via External Secrets Operator.
- **Self-service Git secret mgmt via API server + portal.** ✅ OC#1773/#2700/#3013; BP#214/#406/#465.
- **Build + registry caching.** ✅ OC#3675 (build cache), #865 (buildpack image caching via registry), #3774 (digests for build/registry cache).
- **Removed root/privileged execution; least-privilege builds.** ✅ OC#3600 (remove root access), #4311 (scope executor role least-privilege), #4277 (isolate privileged pods).
- **Shell-injection fix + digest-pinned images.** ✅ OC#4193/#4277/#4297 (shell injection), #3771 (pin builder/run/lifecycle by digest).
- **Reliability: concurrent builds, workflow deletion, status, name limits, git revisions, ARM64.** ⚠️ Status OC#383; name limits OC#43/#195; git revisions OC#206/#207; ARM64 OC#298; workflow deletion OC#2875/#2861. *"Concurrent builds" fix is DA#386 (V3 era), not OC core — verify or drop from the OC line.*

### OpenChoreo APIs, Schemas and Extensibility

- **API server, service layer, resource APIs, CLI tooling.** ✅ OC#305 (build APIs), #830/#899 (component/type APIs), #2242 (workflow APIs), #359 (CLI workload gen).
- **Schema-driven Component Types, Traits, Workflows, params.** ✅ OC#2511 (ocSchema), #779 (schema-based workflow), #933 (update workflow schema for component).
- **OpenAPI v3 schema support for designing developer/user inputs in Component Types, Traits, and Workflows (+ validation).** ✅ OC#2547 (merged 2026-03-10, "support openAPIV3Schema in component types, traits, and workflows" — unified `SectionToJSONSchema` handling both `ocSchema` and `openAPIV3Schema`), #2852 (openAPIV3Schema validation for CT/traits), #2679 (remove ocSchema once v3 became the standard). Design context: discussion [#2379](https://github.com/openchoreo/openchoreo/discussions/2379) "Rethinking Configuration Schemas in OpenChoreo (Simple Schema vs OpenAPI)" (Proposals category — **authored by `sameerajayasoma`**, so cite it as design context, not your authorship).
- **API consistency: allowed workflows, workflow/context/external refs.** ✅ OC#2346 (allowedWorkflows↔allowedTraits), #2657 (immutable ref), #2353 (context refs), #2362 (→ externalRefs).
- **Workload creation via API server + CLI.** ✅ OC#2469 (API server), #359 (CLI, with connections).

### AWS Observability Modules

- **Owned/implemented external AWS observability modules.** ✅ Epic OC#3149; CM, 16 PRs.
- **CloudWatch logs, metrics, K8s events.** ✅ CM#88 (logs), #93 (metrics), #282 (CloudWatch-backed K8s events).
- **Distributed tracing via AWS X-Ray.** ✅ CM#99 (tracing module) → module dir `observability-tracing-aws-xray` (OTel collector → `awsxray` exporter + Go X-Ray adapter). *(PR title says "cloudwatch" but the module is X-Ray.)*
- **Build-log support + multi-cluster.** ✅ CM#122 (build-log fix), #110 (multi-cluster logs), #115/#124 (multi-cluster tracing).
- **Docs, Helm config, operational guidance.** ✅ CM#125 (docs), #121 (chart lock), #124/#115 (setup instructions).

### Developer Portal, Samples and Documentation

- **Backstage: workflows, Git secrets, CodeCommit, run details.** ✅ BP#214 (git secret), #222 (CodeCommit), #375 (workflow-plane rename), #436 (workload details in run details).
- **Component-creation UX; kept portal in sync with API/schema.** ✅ BP#410 (reorder creation steps), #348 (types for ocSchema change).
- **PHP/React/Ballerina/Docker/GCP/buildpack/web-app/multi-language samples.** ✅ OC#90 (PHP), #303 (React/Ballerina), #976 (GCP), #2701 (samples dir); SW#7 (web-app samples).
- **Workload defs, READMEs, endpoint configs, getting-started.** ✅ SW#3 (workload YAMLs), #4 (endpoint names), #2 (README); OC#224 (single-cluster install guide), #226 (contribute guide).
- **Docs + design proposals: CI, workflows, private repos, registries, caching, multi-cluster.** ✅ Proposals `docs/proposals/0142-standardize-build-conditions.md` (OC#164), `0245-introduce-build-plane.md` (OC#247), `0159-control-plane-data-plane-separation.md`; OC#252 (external-registry proposal), #708 (schema-driven); docs OC#2443/#3639 (caching).

### Testing, Quality and Release Management

- **Unit/integration tests: APIs, K8s resources, controllers, workflows, secrets.** ✅ OC#3050 (workflow-run APIs), #3057/#3075 (k8sresources unit+integration), #3089 (workflow-run controller), #3140 (API handlers), #3143 (workflowplane/workflow/workload controllers), #3013 (git secret), #57 (build controller), #1052 (component workflows), #3872 (finops agent).
- **Workflow-template validation, private-repo test paths, E2E UI fixes.** ✅ OC#3911 (default OC/Argo validations), #3928 (private GitHub clone via PAT), #4044 (flaky E2E UI locator).
- **Tooling: license headers, formatting, dependency mgmt, PR checks.** ✅ OC#256 (`licenser` tool; `tools/licenser` exists), #235 (license header), #3941 (Dependabot for agent Dockerfiles).
- **Release manager for 0.2.0, 0.5.0, 0.7.0 (pre-GA).** ✅ OC#940 (Release v0.5.0), CHO/OC#1154 (Release v0.7.0), #958 (Backstage for 0.5.0); v0.2.0 via post-release bump OC#242 ("Bump version to 0.3.0") — source docs cite OC#238 for v0.2.0.
- **Release pipelines, version bumps, Helm, samples, docs, portal enablement.** ✅ OC#244 (fix release pipelines), #222 (helm release), #979/#1159 (version bumps), #978/#1162 (sample URLs), #958/#959 (Backstage enablement).

---

## Repositories cross-checked (Chalindu's activity)

| Repo | PRs (all states) | Notes |
|---|---|---|
| `openchoreo/openchoreo` | 204 | CI/build/workflow core; 100 closed assigned issues |
| `wso2-enterprise/choreo-cp-declarative-api` | 128 | Declarative API + V3 controllers (main + v3 branches) |
| `wso2-enterprise/choreo-cp-configuration-service` | 61 | Configuration Service |
| `wso2-enterprise/choreo-console` | 17 | React deploy/config UI |
| `openchoreo/community-modules` | 16 | AWS CloudWatch + X-Ray modules |
| `openchoreo/backstage-plugins` | 9 | Developer-portal integration |
| `openchoreo/sample-workloads` | 7 | Samples |
| `wso2-enterprise/choreo` (issues) | — | 126 closed assigned issues (support/incidents) |
| `choreo-control-plane`, `choreo-cp-env-overlay`, `choreodp-rudder` | not enumerated here | rollout/overlay/observability-cleanup — verify separately |

*Verification method: `gh pr list` / `gh issue list` / `gh api` as `chalindukodikara`, plus local `git log`/`grep` in `openchoreo`. Private `wso2-enterprise` PR/issue numbers are visible with your token but should be re-checked before use in any formal document.*
