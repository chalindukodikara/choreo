# Areas Worked On — Chalindu Kodikara

A comprehensive record of contributions across WSO2 Choreo and OpenChoreo, compiled from
pull requests and issues authored/resolved on GitHub (August 2023 – July 2026).

## Contribution Summary

| Repository | Merged/Total PRs | Closed Issues Assigned | Period |
|---|---|---|---|
| openchoreo/openchoreo | ~185 / 199 | 100 | Mar 2025 – present |
| wso2-enterprise/choreo-cp-declarative-api | ~120 / 128 | — | Jan 2024 – May 2025 |
| wso2-enterprise/choreo-cp-configuration-service | ~58 / 61 | — | Sep 2023 – May 2024 |
| wso2-enterprise/choreo-cp-env-overlay | ~38 / 40 | — | Oct 2023 – Jan 2025 |
| wso2-enterprise/choreo-control-plane | 20 / 20 | — | Sep 2023 – Jul 2024 |
| wso2-enterprise/choreo-console | 16 / 17 | — | Aug 2023 – Jan 2024 |
| openchoreo/community-modules | 16 / 17 | — | Apr 2026 – present |
| openchoreo/backstage-plugins | 9 / 9 | — | Feb 2026 – present |
| openchoreo/sample-workloads | 6 / 7 | — | Jul 2025 – Jan 2026 |
| wso2-enterprise/choreodp-rudder | 3 / 3 | — | Feb – Jun 2024 |
| wso2-enterprise/choreo (issue tracker) | — | 126 | Aug 2023 – Sep 2025 |
| **Total** | **~470 / 500+** | **226** | **~3 years** |

Work spans the full platform stack: React frontend, Ballerina microservices, Go/Kubernetes
controllers, Argo Workflows CI, Helm/Kustomize deployment, observability integrations, and
Backstage developer-portal plugins.

---

## 1. OpenChoreo — CI / Build Architecture (Flagship Area)

**Role: Primary owner of the CI story in OpenChoreo** — drove the epic
"[Epic] CI Story for OpenChoreo" (#615) from design proposals through multiple milestones
(M1–M5) to production releases. This is the largest and most sustained body of work.

### Build Controller & Build Plane (Feature / Architecture)
- Designed and implemented the initial Build controller and its evolution across three
  generations: provider-coupled → decoupled providers (#36) → pluggable CI engines (#407)
  → schema-driven workflow architecture (#779).
- Authored and implemented the **BuildPlane abstraction** (proposal #247, #221) — a
  dedicated plane for running CI workloads — including allowing any data plane cluster to
  act as a build plane (#236).
- Led the platform-wide rename of **Build Plane → Workflow Plane** (breaking change,
  #2574, milestone M4), coordinated across the core repo and Backstage plugins.
- Introduced **initial multi-cluster deployment support through direct access** (#185)
  and support for multiple Kubernetes authentication methods (#414), including
  token-based cluster client retrieval.

### Schema-Driven Workflows & Component Workflows (Feature / Architecture)
- Authored the "Schema Driven Workflow Architecture" proposal (#708) and implemented it
  for CI (#779) — workflows defined by parameter schemas rather than hardcoded templates.
- Introduced **ComponentWorkflows and WorkflowRuns** (#1037, milestone M2) with tests,
  then unified Component Workflows and Workflows into a single model (#2164).
- Built the workflow run lifecycle end to end: CRUD/status APIs (#2242, #1280), event and
  log APIs (#2325), step-level status exposure (#1633), TTL-based cleanup (#1961),
  deletion via component finalizers (#2329), admission webhooks for validation (#2847),
  immutable workflow refs (#2657), and allowed-workflow validation in the controller (#1896).
- Redesigned Argo Workflow templates to support multiple languages and parameters (#318),
  added React/Ballerina/PHP buildpack support, automated Workload creation through CI
  (#348), and made the workload-creation step optional (#1936).

### Secrets, Private Repositories & External Registries (Feature)
- Designed the architecture for secrets through the workflow plane (issue #888,
  milestone M5) and implemented **self-service Git secret creation** (#1773, #2700)
  with unit tests, plus the corresponding Backstage UI.
- Added **external container registry support** for both image push (#213, #1538) and
  image pull (#246), making registry endpoints configurable (#337).
- Implemented private repository support by default in workflow templates (#1469) and
  repository credentials (#1112); extended source-provider support to **AWS CodeCommit**
  (#1821).

### Build Performance & Reliability (Improvement)
- Implemented the **build cache** (#3675) and simplified buildpack image caching through
  the registry (#865); pinned builder/run/lifecycle images by digest for reproducibility (#3771).
- Fixed a long series of hard production-grade bugs: Argo workflow name/label 63-character
  limits, buildpack failures on Apple Silicon/ARM64 (#298), multi-arch Ballerina buildpack
  images (#4083), concurrent build failures, panics from short git revisions and direct
  type assertions, and consistent build status via condition priority (#383).

### Security Hardening (Improvement)
- **Removed privileged (root) access from Argo workflow templates** (#3600) — rootless
  builds across the workflow plane.
- Fixed a **shell parameter injection vulnerability** in workflow templates (#4193).
- Migrated build images to GHCR (#119) and removed secret refs from workflow specs (#968).

---

## 2. OpenChoreo — Platform Core, Schema System & APIs

- Introduced the **ocSchema shorthand and openAPIV3Schema support** for ComponentTypes,
  Traits, and Workflows (#2511, #2547, #2539), added openAPIV3Schema validations (#2852),
  and completed the migration by removing the legacy format (#2679) — the schema system
  that underpins the platform's templating.
- Improved Component and ComponentType APIs (#830, #899), aligned `allowedWorkflows` with
  `allowedTraits` as structured objects (#2346), renamed `traitOverrides` to
  `traitEnvironmentConfigs` (breaking API change, #2607), and added externalRefs to the
  workflow CEL context (#2353, #2362).
- Built workload creation through the API server (#2469) and the CLI (`occ`) workload
  generation including connections (#359).
- Enabled **log collection via Fluent Bit** (#325) and OpenSearch write permissions (#444).
- Contributed to release engineering: version bumps for v0.3.0/v0.6.0/v0.8.0, release
  pipeline fixes, sample URL migrations per release branch, and the Backstage enablement
  for v0.5.0.
- Added the `licenser` tool for SPDX license header validation to the PR pipeline (#256).

## 3. OpenChoreo — Testing & Quality (Improvement)

Drove a sustained test-coverage push across the codebase:
- Unit and integration tests for the k8sresources package, workflow run APIs and
  controller, workflowplane/workflow/workload controllers, API handlers, git secret
  creation, licenser, and the FinOps agent (#3050–#3143, #3872).
- Validations for default OpenChoreo and Argo workflow templates (#3911), an e2e path for
  private GitHub repo cloning via PAT (#3928), Dependabot coverage for agent Dockerfiles
  (#3941), and flaky e2e UI test fixes (#4044).

## 4. OpenChoreo — Observability Modules (community-modules)

**Owner of the "Modules for External Observability" epic (#3149)** — built the AWS
CloudWatch integration suite:
- **CloudWatch Logs module** (#88) including build-log support and multi-cluster setups (#110).
- **CloudWatch Metrics module** (#93) with metric-alarm namespace resolution fixes.
- **CloudWatch Tracing module** (#99) with multi-cluster setup instructions.
- **Kubernetes events querying backed by CloudWatch** (#282); HTTP metrics and runtime
  topology support (in progress, #323).

## 5. Backstage Developer Portal Plugins (openchoreo/backstage-plugins)

Extended the OpenChoreo Backstage UI for the CI features built in the core platform:
- Git secret creation flows and iterative UX improvements (#214, #406, #465).
- AWS CodeCommit support (#222), workflow-plane rename (#375), type updates for the
  schema changes (#348), reordered component-creation steps (#410), and workload details
  in run views (#436).

## 6. Choreo V3 / Declarative API (wso2-enterprise/choreo-cp-declarative-api)

**One of the core engineers of Choreo's V3 control-plane rewrite** (128 PRs). This work
evolved through three phases:

### Declarative API (Ballerina) — 2024
- Built and productionized the Declarative API for managing Choreo resources as
  kind-files: Component, Build, Deployment, and ComponentConfig kinds; fixed free-tier
  limit handling (500→403), null component paths, unsupported-component filtering, and
  MI/BYOI component-type support.
- Designed and implemented the **storage layer** (#107) with insert/search/update/delete,
  and PostgreSQL volume testing for large kind-file volumes.
- Added unit and automated integration test suites and enabled tests in the PR pipeline.

### Controller Architecture (Ballerina → Go) — 2024
- Implemented the controller-loop pattern for the platform: project, environment,
  environment-template, component, and deployment controllers integrating with V2 services.
- **Led the migration of controllers from Ballerina to Go** (#203, #204, #272), including
  a Ballerina V2-clients library for admission controllers (#276).
- Implemented **periodic sync/reconciliation** (#288), hierarchical resource metadata
  (#262), idempotent project creation, concurrency-safe creation via DB constraints, and
  update support across all resources (#238).

### Build Controller with Argo Workflows — 2025
- Built the V3 build controller on Argo Workflows (#316): buildpack builds,
  multi-architecture (Alpine/Ubuntu) images (#373), organization-scoped workflow
  namespaces (#381), worker-node image caching (#346), React builds via custom
  Dockerfile (#396), git-revision builds (#358), auto-deployment on build (#409), and
  concurrent-build failure resolution (#386).

## 7. Configuration Management Service (wso2-enterprise/choreo-cp-configuration-service)

**Built Choreo's configuration-group service largely from the ground up** (61 PRs,
Ballerina): CRUD APIs for configuration groups, **Azure Key Vault / secret-manager
integration** for secure value storage and reference retrieval, environment-UUID to
template-UUID migration, search integration, correlation-ID request/response
interceptors, structured error handling, concurrency fixes, request-latency
optimization, and unit tests across core, clients, and endpoints. Also delivered its
database schema (SQL scripts with cascade rules and update-time triggers) and its
Kubernetes deployment (manifests, SecretProviderClass, per-environment overlays for
dev/stage/prod).

## 8. Choreo Console — Frontend (wso2-enterprise/choreo-console)

React/TypeScript contributions to the deploy experience (2023):
- **Direct-deploy option with split button** — reduced clicks to deploy a component,
  including proxy-with-policy handling and post-deploy step transitions.
- Endpoint URLs surfaced in the endpoint details side panel; BYOI deploy side panel.
- Bug fixes: duplicate configuration names on the deploy page, project-name persistence
  in the creation dialog, component dropdown refresh, text overlap in the role dialog,
  and design-system `KeyValueCard` state variants.

## 9. Platform Deployment, GitOps & Data Plane (control-plane, env-overlay, rudder)

- Authored the Kubernetes deployment manifests and per-environment (dev/stage/prod)
  GitOps overlays for the Configuration Service and the V3 Choreo API controllers —
  secrets mounting, Cilium network-policy allow-lists, config-toml wiring, JWT
  issuer/audience updates, and controlled rollout/rollback of V3 components.
- Data plane (rudder): implemented **Ballerina observability config removal** on
  redeploy and on component promotion — coordinated across rudder, App Service (gRPC),
  CICD feature flags, and all three environment overlays.

## 10. Production Support & Incident Response

Handled customer-reported production issues on the wso2-enterprise/choreo tracker,
including private-endpoint exposure analysis (Brighton), intermittent zero-status-code
errors (Colorcon), execution-log access failures (Holmesglen), and Azure Monitor alert
investigations — alongside three years of bug fixes labeled across Development, Staging,
and Production environments.

## 11. Samples, Documentation & Developer Experience

- Created and maintained the OpenChoreo samples ecosystem: multi-language sample
  workloads (Go, React, PHP, Ballerina, GCP integrations), restructured samples layout
  (#334, #2701), single-cluster installation guide (#224), contributor guide updates,
  module docs for CI/Argo Workflows, and build-caching documentation.
- Authored design proposals checked into the repo: Build Plane, Build Redesign,
  Schema-Driven Workflow Architecture, and External Registry Image Pulling.

---

## Skills Demonstrated Across These Areas

- **Languages/Frameworks:** Go (Kubernetes controllers, controller-runtime/Kubebuilder),
  Ballerina (microservices), React/TypeScript (console, Backstage), SQL (PostgreSQL/MSSQL).
- **Platform engineering:** CRD/API design, admission webhooks, reconciliation loops,
  multi-cluster architecture, Helm/Kustomize, GitOps overlays, Cilium network policies.
- **CI/CD:** Argo Workflows, Cloud Native Buildpacks, container registries, build
  caching, multi-arch images, Azure Pipelines, GitHub Actions.
- **Cloud & observability:** AWS CloudWatch (logs/metrics/traces/events), OpenSearch,
  Fluent Bit, Azure Key Vault.
- **Security:** rootless CI workloads, injection-vulnerability remediation, secret
  management architecture, image digest pinning.
- **Ownership:** end-to-end epics from design proposal → implementation → tests → docs →
  release → UI integration, sustained over multiple release cycles.

---

*Sources: GitHub PRs authored by `chalindukodikara` and closed issues assigned to
`chalindukodikara` across the repositories listed above, retrieved July 2026 via
read-only GitHub API queries. PR/issue numbers are cited per repository for verification.*
