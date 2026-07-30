I, Chalindu Malshika Kodikara Kodikara Arachchige, worked at WSO2 LLC from July 15, 2023 to July 31, 2026. I served as a Software Engineer from July 15, 2023 to July 31, 2025, and as a Senior Software Engineer from August 1, 2025 to July 31, 2026.


I was part of the WSO2 Developer Platform (formerly Choreo) engineering team from July 2023. My work there spanned configuration management, the developer console experience, and the platform's declarative API, along with customer support and platform reliability.


Configuration Management

Designed and implemented the Choreo Configuration Service, the backend responsible for managing both user-facing and internal platform configuration.
Built create, update, search, retrieve, and delete behavior for configuration groups and configuration values, including empty-configuration handling and source-type tracking.
Integrated secure configuration storage through the platform's Secret Manager and Azure Key Vault, with reference-based retrieval for sensitive values and cleanup of removed values from the vault.
Added payload, uniqueness, type, and scope validation, correlation IDs for request tracing, structured error handling, and performance and concurrency fixes.
Owned this service end to end — from design through database schema, Kubernetes deployment, and rollout across development, staging, and production — until my move to OpenChoreo.


Build & Deploy Configuration UI (Choreo Console)

Designed and implemented the build- and deploy-time configuration experience in the Choreo Console using React and TypeScript.
Added a direct-deploy option, including a split-button flow, that let developers deploy components without stepping through the full configuration wizard.
Built the deploy-time configuration side panel and the bring-your-own-image and proxy/API component deploy flows, and surfaced endpoint URLs and endpoint configuration in the endpoint side panel.
Resolved a range of console defects, including duplicate configuration names on the deploy page, project-name persistence, component-list refresh, and design-system component states.


Declarative API

Contributed to the Choreo Declarative API, the kind-based, Kubernetes-style API that let customers manage Choreo resources — Project, Component, Build, Deployment, and Component Config — declaratively rather than through the console.
Implemented the storage layer with insert, search, update, and delete behavior, along with component creation, build outputs such as build ID and commit hash, and component-config retrieval.
Added support for WSO2 MI and bring-your-own-image component types, component handles, display names, UUIDs, port fields, and correct free-tier limit handling.
Added unit tests across the service and the Build, Deployment, Component, and Component Config kinds, and wired the test stage into the Azure build pipeline.


Customer Support & Platform Reliability

Served on the customer support rotation, resolving production issues for enterprise customers.
Responded to production incidents, restoring service and addressing the underlying root causes.
Contributed to a platform stabilization effort, analyzing production logs to identify and reduce recurring server errors.


From January 2024 to January 2025, I was part of the research team that explored an improved architecture for the WSO2 Developer Platform, drawing on the lessons learned from operating it at scale. This research led to the creation of OpenChoreo in January 2025.


Research Phase

Was part of the team that researched a reconciliation-based platform architecture modeled on Kubernetes.
Ran a PostgreSQL performance study on Azure VMs to evaluate query performance at scale, using a dataset of approximately 50 million records to determine the maximum data volume the database could handle while keeping search queries within a one-second threshold.
Validated database behavior under concurrent access, scaling up to 1,000 concurrent users.


Initial Version of OpenChoreo (Pre-Kubernetes)

Worked on the initial implementation of OpenChoreo, an early attempt to build our own Kubernetes-style platform with a custom resource model and controller runtime tailored to the platform's needs.
Owned the data-handling layer, implementing PostgreSQL-backed persistence with insert, search, update, and resource-revision behavior across the platform's resource kinds.
Implemented controller reconciliation logic, including project, environment, and component controllers with reconcile loops, update support, and idempotent, concurrency-safe resource creation.
Contributed to migrating the control-plane controllers from Ballerina toward Go as the model matured.


Since January 2025, I have been a bootstrap contributor and a maintainer of OpenChoreo, WSO2's open-source, Kubernetes-native internal developer platform. From its 0.1.0 release through its 1.2.x releases I have been the highest contributor to the project, working closely with Kubernetes and Docker.


Initial Platform (Kubebuilder)

Built the initial implementation of OpenChoreo on the Kubebuilder framework.
Implemented the initial deployment, build, project, and component controllers.
Authored the project's license-header validation tool and added the formatting and linting tooling enforced across the codebase in CI.


Communication Between Planes & Multi-Cluster

Implemented the initial communication between the control plane and the data plane using direct cluster access.
Authored the accepted proposal for control-plane and data-plane separation and added initial multi-cluster deployment support.
Added support for multiple Kubernetes authentication methods and token-based retrieval of cluster clients.


Secret Management, Private Repositories & Registries

Introduced OpenBao, an open-source key vault, integrated through the External Secrets Operator for secret management across the platform.
Implemented self-service Git secret management through the API server and the developer portal, backed by the key vault, which laid the foundation for managing other secret types.
Designed and implemented support for SSH- and PAT-based private repositories, including their APIs and UI/UX, and extended source support to AWS CodeCommit.
Added support for private and external container registries for both image push and pull.


Workflow Plane, CI & Workflows

Owned the CI and workflow story of OpenChoreo, driving it from design proposals through delivery across multiple milestones.
Designed and implemented the CI model, authoring the accepted BuildPlane proposal and later generalizing it into the Workflow Plane so that any data plane cluster could execute build and workflow workloads.
Expanded the model into a schema-driven, generic Workflow Plane capable of running any workflow — such as database migrations and infrastructure provisioning — beyond builds.
Implemented multiple build types on Argo Workflows — Buildpack-based builds using Google Cloud Buildpacks, Paketo Buildpacks, and Ballerina buildpacks, along with Docker builds — providing broad language support.
Built the workflow lifecycle end to end, including WorkflowRun create, status, log, and event APIs, admission webhooks, allowed-workflow validation, and TTL-based cleanup, and hardened workflow templates by removing root access, pinning build images by digest, and closing a shell-parameter injection risk.


AWS Observability Modules

Implemented the AWS observability modules for OpenChoreo, delivering the first complete external-observability integration in the project.
Built log and metric collection through AWS CloudWatch and distributed tracing through AWS X-Ray, using an OpenTelemetry collector and a tracing adapter.
Added multi-cluster support across the logs, metrics, and tracing modules, along with CloudWatch-backed Kubernetes event querying and HTTP metrics with runtime topology.


Testing & Quality

Ran a sustained test-coverage campaign across the OpenChoreo codebase, adding unit and integration tests for controllers, the Kubernetes resources package, the API handlers, and the workflow-run APIs.
Added coverage for Git secret creation, private-repository clone paths, and default workflow-template validation, and stabilized flaky end-to-end UI tests.


Samples & Documentation

Added PHP, React, Ballerina, GCP, Docker, buildpack, and multi-language samples along with their documentation.
Authored getting-started material, including the single-cluster installation guide and the contributor guide, and maintained the official sample workloads used for onboarding, demos, and release validation.


Release Management

Acted as the release manager for three pre-GA releases: 0.2.0, 0.5.0, and 0.7.0.
Handled version bumps through later releases, fixed release pipelines, updated per-release sample references, and coordinated developer-portal enablement for releases.
