# Service Letter

**WSO2 LLC**

---

**To Whom It May Concern**

This letter is to certify that **Chalindu Malshika Kodikara Kodikara Arachchige** (Passport Number: **N9084035**) was employed at WSO2 LLC as a permanent employee from **July 15, 2023 to July 31, 2026**.

He served as a **Software Engineer** from July 15, 2023 to July 31, 2025, and — in recognition of his consistently strong performance — was promoted to **Senior Software Engineer**, a role he held from August 1, 2025 to July 31, 2026.

During his tenure, Chalindu was a core engineer on the **WSO2 Choreo / WSO2 Developer Platform** (WSO2's SaaS Internal Developer Platform) and a founding-team contributor to **OpenChoreo**, WSO2's open-source Internal Developer Platform project. Across these platforms he authored **over 460 merged pull requests** and resolved **more than 220 tracked engineering issues**, spanning frontend, backend services, Kubernetes control-plane engineering, CI/CD systems, security, and cloud observability.

---

## Summary of Contributions

### 1. OpenChoreo — Open-Source Internal Developer Platform (2025 – 2026)

OpenChoreo is an open-source, Kubernetes-native Internal Developer Platform. Chalindu was a **bootstrap contributor** to the project and, at the time of writing, is its **highest individual contributor** (600+ commits and 180+ merged pull requests in the core repository — nearly double the next contributor), with additional contributions across the project's Backstage plugins, community modules, and sample repositories.

His key areas of ownership:

- **CI / Build subsystem (architect and primary author).** Designed and implemented OpenChoreo's entire build platform, from the initial Build Controller through its evolution into the **Workflow Plane** — an Argo Workflows–based CI engine with pluggable providers. This included the Workflow and WorkflowRun custom resources, their Kubernetes controllers and admission webhooks, schema-based workflow design, Cloud Native Buildpacks support (Ballerina, React, PHP, Docker, and more), build caching strategies, and multi-architecture image builds.

- **Platform API design.** Designed and built REST APIs for builds, workflows, and workflow runs (creation, deletion, logs, and events), and contributed extensively to component APIs and the platform's OpenAPI specifications.

- **Extensibility and schema framework.** Implemented the parameter schema system used across ComponentTypes, Traits, and Workflows — including a developer-friendly shorthand schema format (`ocSchema`), full OpenAPI v3 schema support, and schema-driven validation — a foundational abstraction of the platform's templating model.

- **Security hardening.** Removed root access from CI workflow templates, eliminated shell parameter-injection vectors, pinned builder images by digest, and built end-to-end support for private Git repositories, private container registries, Git credential (secret) management, and multiple Kubernetes authentication methods.

- **Multi-cluster and multi-cloud support.** Delivered the platform's initial multi-cluster deployment support, external container registry integrations, and AWS CodeCommit integration.

- **Cloud observability modules.** Authored the AWS CloudWatch community modules for logs, metrics, tracing, and Kubernetes events — including multi-cluster observability setups.

- **Developer experience.** Contributed to the OpenChoreo Backstage plugins (component creation flows, Git secret management, workflow run details), built a large portion of the project's official samples, wrote developer documentation, authored the project's SPDX license-validation tooling, and participated in release engineering across multiple minor releases.

- **Quality engineering.** Drove test coverage across the platform: unit, integration (envtest), and end-to-end test suites for controllers, APIs, and workflow infrastructure.

### 2. Choreo V3 Control Plane — Declarative API and Go Controllers (2024 – 2025)

As part of Choreo's next-generation (V3) architecture, Chalindu was a lead engineer on the **Declarative API** (choreo-cp-declarative-api), the control-plane service that lets users manage Choreo resources — Projects, Components, Builds, Deployments, and Environments — declaratively.

- Designed and implemented the API's **storage layer** on PostgreSQL, including volume/performance testing, race-condition prevention, and idempotent resource creation under concurrency.
- Implemented and maintained the core **resource kinds** (Project, Component, Build, Deployment, Component Config, Environment) in Ballerina, together with automated integration tests and unit test suites.
- Led the **migration of control-plane reconciliation logic from Ballerina to Go**, designing and implementing Kubernetes-style controllers (Project, Component, Environment, Environment Template, Deployment, Build) with reconcile loops, periodic sync, and structured resource-hierarchy metadata.
- Built build capabilities for the platform, including buildpack-based builds for web applications, organization-scoped Argo Workflows namespaces, image caching, and concurrent-build reliability fixes.
- Owned production reliability for these services: diagnosed and fixed customer-reported incidents, 500-level API failures, and deployment/promotion issues across development, staging, and production environments.

### 3. Configuration Management — WSO2 Developer Platform (2023 – 2024)

Chalindu **owned configuration management** for the platform, designing and developing the Configuration Service (Ballerina) used for both internal and customer configuration management.

- Designed and implemented the service's REST APIs, database layer (including schema scripts and cascade/trigger logic), and validation framework.
- Integrated the service with the platform's **Secret Manager and Azure Key Vault** for secure storage and reference-based retrieval of sensitive configuration values.
- Improved reliability and performance: request-latency optimization, concurrent-call failure fixes, correlation-ID propagation for distributed tracing, and comprehensive unit test coverage across the service, database, and client modules.
- Handled the service's full deployment lifecycle — Kubernetes manifests, secret provider configuration, database provisioning, and environment-specific rollout across development, staging, and production.

### 4. Frontend Engineering — Choreo Console (2023 – 2024)

Contributed to the Choreo Console, the platform's React-based developer portal:

- Delivered the **direct-deploy** user experience (split-button deploy, deploy side panels, BYOI deploy flows) and API-proxy deployment improvements.
- Enhanced shared design-system components and fixed user-facing bugs across component creation, project management, and role management flows.

### 5. Platform & Release Operations (2023 – 2025)

Across the platform, Chalindu worked hands-on with production infrastructure: environment overlay configuration for controllers and services across dev/stage/prod, Cilium network policy and ingress configuration, secret mounting via secret provider classes, database scripts for control-plane services, feature-flag-driven rollouts (including the Ballerina observability configuration cleanup delivered across the deployment engine, CI/CD, and app services), and Azure DevOps release pipelines.

---

## Capabilities Demonstrated

Chalindu's work at WSO2 demonstrates end-to-end platform engineering capability:

- **Languages & frameworks:** Go (Kubernetes controllers, controller-runtime/Kubebuilder), Ballerina, React/TypeScript
- **Cloud-native engineering:** Kubernetes operators, CRD/API design, admission webhooks, Helm, Argo Workflows, Cloud Native Buildpacks, multi-cluster architectures
- **Cloud platforms:** Azure (Key Vault, DevOps pipelines), AWS (CloudWatch, CodeCommit)
- **Data & APIs:** PostgreSQL schema design and performance, REST/OpenAPI design, concurrency and idempotency engineering
- **Security:** secret management, CI hardening, supply-chain integrity (image digest pinning), private repository/registry support
- **Quality & operations:** unit/integration/e2e testing, production incident response, release engineering, developer documentation
- **Open source:** top contributor and core maintainer-level ownership on a public CNCF-ecosystem project, including community-facing samples, docs, and plugins

Chalindu consistently took ownership of complex systems from design through production operation, and his contributions were central to both the commercial Choreo platform and the OpenChoreo open-source project.

We wish him every success in his future endeavors.

---

**WSO2 LLC**

*Issued: July 2026*
