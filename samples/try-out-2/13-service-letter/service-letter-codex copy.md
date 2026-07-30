# Service Letter and Contribution Summary

**To Whom It May Concern**

This is to certify that **Chalindu Malshika Kodikara Kodikara Arachchige** (Passport Number: **N9084035**) was employed at **WSO2 LLC** as a permanent employee from **July 15, 2023, to July 31, 2026**.

He served as a **Software Engineer** from **July 15, 2023, to July 31, 2025**, and, based on his performance and contributions, was promoted to **Senior Software Engineer**, a role he held from **August 1, 2025, to July 31, 2026**.

During his tenure, Chalindu worked on **WSO2 Choreo / WSO2 Developer Platform** and **OpenChoreo**, WSO2's open-source Kubernetes-native internal developer platform. His work spanned product-facing frontend development, backend platform services, declarative APIs, Kubernetes controllers, CI/CD and workflow systems, observability, security, release engineering, and production operations.

Chalindu's contributions demonstrate the profile of an engineer who can take platform capabilities from design to production: understanding customer and developer problems, designing APIs and control-plane abstractions, implementing services and controllers, hardening them with tests and security controls, rolling them out across environments, and supporting them in production.

## Main Areas of Work

### 1. Configuration Management for WSO2 Choreo

Chalindu owned a major configuration-management area in Choreo by designing and implementing the **Choreo CP Configuration Service**, a backend service used for customer-facing and internal platform configuration management.

This work included:

- Designed and implemented configuration-management APIs for creating, updating, searching, retrieving, and deleting configuration groups and values.
- Built the service's database layer, validation logic, OpenAPI definitions, update semantics, source-type handling, empty-configuration handling, and environment-template mapping.
- Integrated secure storage through **Secret Manager / Azure Key Vault**, including reference-based retrieval of sensitive values and deletion of removed values from the key vault.
- Added production-readiness capabilities such as correlation ID propagation, structured error handling, request validation, JWT and organization-identity handling, concurrency fixes, and latency optimizations.
- Added unit tests across service endpoints, core logic, database modules, Secret Manager clients, Cloud Manager clients, and integration behavior.
- Owned rollout work across Kubernetes manifests, SQL scripts, secret provider classes, config mounts, database triggers, and dev/stage/prod environment overlays.

This contribution is significant because it was not a narrow feature change. It was end-to-end platform service ownership: API design, backend implementation, secure secret handling, data modeling, testing, deployment, and production hardening.

### 2. Choreo Declarative API and V3 Control Plane

Chalindu was a major contributor to the **Choreo CP Declarative API** and Choreo V3 control-plane work, which enabled Choreo resources to be managed declaratively through APIs and CLI-driven workflows.

His work in this area included:

- Implemented and maintained platform resource kinds such as Project, Component, Build, Deployment, Component Config, Environment, and Environment Template.
- Designed and implemented storage-layer behavior for resource persistence, search, update, delete, resource hierarchy metadata, and PostgreSQL-backed data management.
- Improved API behavior for component creation, deployment retrieval, build triggering, component config retrieval, project updates, environment updates, and resource revision handling.
- Added production features and fixes for MI components, BYOI components, web app buildpack components, display names, component identifiers, component handles, port fields, unsupported components, and build output metadata.
- Addressed reliability problems such as 500-level failures, race conditions, JWT/header extraction issues, project-creation idempotency, initial-environment creation failures, and concurrency behavior in Choreo API.
- Helped migrate control-plane reconciliation logic from Ballerina to Go, including project, environment, component, deployment, environment-template, and build controllers with reconcile loops and periodic sync.
- Built and improved build-controller functionality using Argo Workflows, image caching, multi-architecture builds, buildpack language/version support, organization-scoped workflow namespaces, concurrent-build fixes, and auto-deployment behavior.
- Added unit and integration tests for core resource logic, service endpoints, controllers, clients, and automated integration paths.

This work demonstrates strong backend and control-plane engineering: API design, data modeling, controller design, concurrency handling, production debugging, and migration of platform components across implementation languages.

### 3. OpenChoreo CI, Build, and Workflow Platform

Chalindu was a bootstrap and core contributor to **OpenChoreo**, where his most important contribution area was the CI/build and workflow execution platform. He helped evolve OpenChoreo from an early build-controller model into a more general workflow-based platform capability.

His major contributions included:

- Designed and implemented major parts of the OpenChoreo build subsystem, including build controllers, build APIs, workflow templates, build status handling, finalizers, events, and test coverage.
- Decoupled the build controller from specific providers and helped shape a pluggable CI execution model.
- Introduced and evolved the **Build Plane / Workflow Plane** abstraction for running build and workflow workloads across platform clusters.
- Designed and implemented Workflow and WorkflowRun APIs, workflow run status exposure, event and log APIs, deletion APIs, workflow references, and controller behavior.
- Implemented Argo Workflows-based CI execution for buildpacks, Docker workflows, Ballerina, React, PHP, and other workload types.
- Added private repository support, self-service Git secret creation, private registry support, external registry integration, AWS CodeCommit support, and secret propagation through workflow execution.
- Added multi-cluster support, including direct control-plane-to-data-plane access and support for running workflow workloads in different data-plane clusters.
- Improved platform extensibility through schema-driven workflow design, OpenAPI v3 schema validation, workflow admission webhooks, allowed workflow validation, external references, and workflow-plane references.
- Improved reliability by fixing workflow name-length failures, controller panic paths, deletion/finalizer behavior, incorrect build statuses, workload-generation failures, buildpack failures, and managed Kubernetes compatibility issues.
- Improved security by removing root access from workflow templates, fixing shell-parameter injection risks, adding license-header validation, pinning workflow images by digest, and improving dependency coverage.

This contribution is recruiter-relevant because it shows architecture and ownership of a real platform subsystem: custom resources, controllers, workflow orchestration, API design, security, extensibility, and production reliability.

### 4. OpenChoreo Observability and Ecosystem Work

Chalindu also worked beyond the core repository to make OpenChoreo usable as a complete developer platform.

His ecosystem contributions included:

- Authored and maintained AWS CloudWatch-backed modules for logs, metrics, traces, and Kubernetes events.
- Added multi-cluster setup support, alarm namespace handling, event querying, runtime topology, and module documentation for AWS observability integrations.
- Contributed to OpenChoreo Backstage plugins, including Git secret creation UX, AWS CodeCommit support, workflow-plane terminology updates, workload/run details, and component-creation flow improvements.
- Added and maintained official sample workloads and web application samples used in quick starts, demos, and release validation.
- Contributed to OpenChoreo releases through sample updates, workflow updates, release-readiness fixes, release pipeline fixes, and documentation updates.

This work shows breadth across backend platform features, developer portals, observability integrations, samples, and release readiness.

### 5. Choreo Console and Developer Experience

Chalindu contributed to **Choreo Console**, the React/TypeScript-based developer portal used by Choreo users.

His frontend and developer-experience work included:

- Implemented and improved the direct-deploy experience, including deploy split buttons, confirmation behavior, and deploy side panels.
- Added endpoint URLs to endpoint details side panels and improved visibility of deployment details.
- Improved BYOI and proxy-component deployment flows, including policy-attached proxy behavior.
- Fixed user-facing bugs in project creation, component list refresh, deployment configuration panels, duplicate configuration names, role dialog layout, and endpoint configuration button behavior.
- Enhanced design-system component behavior, including multi-state support for the `KeyValueCard` component.
- Updated localization and user-facing wording for endpoint and API configuration flows.

This demonstrates that Chalindu's work was not limited to backend systems. He also delivered user-facing product improvements and improved developer workflows in the platform UI.

### 6. Production Operations, Incidents, and Platform Rollout

Across Choreo and OpenChoreo, Chalindu worked on platform operation and reliability, including environment rollout and customer-facing issue resolution.

His operational work included:

- Added and maintained Kubernetes manifests, config maps, SQL scripts, service definitions, controller deployment configuration, secret mounts, and SecretProviderClass resources.
- Rolled out features and services across development, staging, and production environments through environment overlays.
- Added Cilium network-policy allow-list updates, ingress/service URL updates, controller config changes, and secret/config mount fixes.
- Worked on Ballerina observability configuration cleanup across deployment and promotion flows.
- Investigated and resolved customer or production issues, including deployment failures, logs access issues, private endpoint exposure reports, Git bot rate-limit alerts, and intermittent runtime/deployment behavior.
- Stabilized CI and validation paths, including workflow template validation, buildpack fixes, UI e2e test fixes, and release pipeline fixes.

This demonstrates practical production ownership: not only writing features, but making them reliable, secure, deployable, observable, and supportable.

## Capabilities Demonstrated

Chalindu's work at WSO2 demonstrates the following capabilities:

- **Cloud-native platform engineering:** Kubernetes controllers, custom resources, admission webhooks, Helm, multi-cluster architecture, data-plane/control-plane integration, and operator-style reconciliation.
- **CI/CD and workflow architecture:** Argo Workflows, build controllers, workflow planes, workflow templates, Cloud Native Buildpacks, image caching, GitHub Actions, Azure DevOps, release pipelines, and workflow security.
- **Backend and API engineering:** REST API design, OpenAPI schemas, Ballerina services, Go services/controllers, PostgreSQL-backed storage, validation frameworks, idempotency, concurrency handling, and API migration.
- **Security and secrets:** Azure Key Vault / Secret Manager integration, private Git access, registry credentials, secret references, workflow secret propagation, least-privilege workflow templates, and supply-chain hardening.
- **Observability:** AWS CloudWatch logs, metrics, traces, Kubernetes events, OpenSearch-related fixes, runtime topology, and platform monitoring integrations.
- **Frontend and developer experience:** React/TypeScript platform UI work, Backstage plugin development, samples, documentation, developer-facing workflows, and CLI/API enablement.
- **Quality engineering:** Unit tests, integration tests, controller tests, e2e UI tests, workflow-template validations, regression fixes, and release validation.
- **Production ownership:** Incident investigation, customer issue resolution, dev/stage/prod rollout, performance optimization, reliability hardening, and cross-service coordination.

## Evidence Reviewed

This summary was prepared by reviewing the issue and repository history across the requested Choreo and OpenChoreo repositories. In particular, the contribution themes above were cross-checked against:

- All closed issues assigned to Chalindu in `wso2-enterprise/choreo`, covering Choreo Console, configuration service, Declarative API, Choreo V3, production issues, and customer issues.
- All closed issues assigned to Chalindu in `openchoreo/openchoreo`, covering build controllers, workflow plane, workflow APIs, private repositories, registries, schema validation, releases, observability modules, tests, and reliability fixes.
- Supporting pull request and repository history in `choreo-cp-configuration-service`, `choreo-cp-declarative-api`, `choreo-console`, `choreo-cp-env-overlay`, `choreo-control-plane`, `choreodp-rudder`, `openchoreo/openchoreo`, `community-modules`, `sample-workloads`, and `backstage-plugins`.

The raw contribution volume is not the main point of this letter. The important point is the breadth and depth of ownership: Chalindu repeatedly worked on platform areas that required design, implementation, integration, rollout, debugging, testing, security, and production support.

## Closing Statement

Chalindu consistently contributed across the full lifecycle of platform engineering. His work was central to both WSO2's commercial Choreo platform and the OpenChoreo open-source project, and it demonstrates strong capability in backend services, Kubernetes control planes, CI/CD systems, frontend developer experience, observability, security, and production operations.

We wish him every success in his future endeavors.

**WSO2 LLC**

*Issued: July 2026*
