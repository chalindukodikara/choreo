I, Chalindu Malshika Kodikara Kodikara Arachchige, worked at WSO2 LLC from July 15, 2023 to July 31, 2026. I served as a Software Engineer from July 15, 2023 to July 31, 2025, and as a Senior Software Engineer from August 1, 2025 to July 31, 2026.


I was part of the WSO2 Developer Platform (formerly Choreo) engineering team from July 2023 to February 2024. My work focused on configuration management, developer-facing console workflows, declarative APIs, platform reliability, and production support.


Configuration Management

Designed and implemented the Choreo Configuration Service for managing user-facing and internal platform configurations.
Implemented configuration groups and configuration values, including create, update, retrieve, search, delete, validation, and source-type handling.
Integrated the service with Secret Manager and Azure Key Vault for secure configuration storage and value-reference retrieval.
Built the database access layer, SQL scripts, update-time handling, and cleanup behavior for removed configuration values.
Added correlation IDs, structured error handling, concurrency fixes, performance improvements, and unit tests for the service, database, Secret Manager client, and Cloud Manager client.
Supported the rollout of the service through Kubernetes manifests, environment overlays, configuration mounts, and release pipeline updates.


Choreo Console & Build Configuration UI

Contributed React and TypeScript changes to the Choreo Console, the developer-facing UI for Choreo users.
Improved build/deploy configuration flows, including direct deploy, deploy side panels, proxy component deployment, and BYOI component deployment.
Improved endpoint-related user experience by surfacing endpoint URLs in the endpoint details side panel.
Fixed console issues related to duplicate configuration names, project creation, component list refresh behavior, role dialog layout, and design-system component states.


Declarative API

Worked on Choreo's Declarative API, which handled declarative resource requests into the WSO2 Developer Platform.
Implemented and maintained resource kinds such as Project, Component, Build, Deployment, and ComponentConfig.
Built storage-layer behavior for inserting, searching, updating, deleting, and retrieving declarative resources.
Added support for resource metadata, component identifiers, build output metadata, and component-type-specific behavior.
Improved API correctness, performance, and test coverage for declarative resource flows.


Choreo V3 Control Plane

Contributed to Choreo's V3 control-plane implementation, which moved the platform toward a Kubernetes-style reconciliation model.
Implemented and improved project, environment, environment-template, component, deployment, and build controllers.
Helped migrate controller logic from Ballerina to Go.
Worked on idempotency, concurrency handling, periodic reconciliation, JWT handling, and controller status correctness.
Implemented build-controller functionality using Argo Workflows, buildpacks, image caching, organization-scoped workflow namespaces, and auto-deployment behavior.


Customer Support & Platform Reliability

Served on the customer support rotation, resolving production issues for enterprise customers.
Responded to production incidents, restored service, and addressed underlying root causes.
Investigated issues related to deployment failures, execution-log access, private endpoint behavior, build failures, and intermittent runtime errors.
Contributed to platform stabilization by analyzing production logs and reducing recurring server errors.


From January 2024 to January 2025, I was part of the research team that explored an improved architecture for the WSO2 Developer Platform based on the lessons learned from Choreo. This work led to the creation of OpenChoreo in January 2025.


Research Phase

Researched a reconciliation-based platform architecture modeled on Kubernetes.
Worked on resource modeling, controller behavior, and platform architecture ideas for the next version of the developer platform.
Performed PostgreSQL performance testing on Azure VMs with approximately 50 million records.
Evaluated search query performance under a one-second threshold and tested concurrent access with up to 1,000 concurrent users.


Initial Version of OpenChoreo

Worked on the initial OpenChoreo prototype before the Kubernetes-native implementation.
Implemented data-handling logic for different database implementations.
Worked on the early controller-runtime implementation using a custom YAML resource model.
Helped validate the platform architecture before the team moved to a Kubernetes-native design.


Since January 2025, I have been a bootstrap contributor and maintainer of OpenChoreo, WSO2's open-source Kubernetes-native internal developer platform. From its 0.1.0 release through its 1.2.x releases, I worked across the core platform, workflow plane, API server, controllers, developer portal, samples, documentation, and release engineering.


Initial Kubernetes-Native OpenChoreo Platform

Built the initial OpenChoreo implementation using the Kubebuilder framework.
Implemented the initial project, component, deployment, and build controllers.
Defined and evolved Kubernetes-native APIs, custom resources, status conditions, finalizers, and reconciliation flows.
Added project tooling for license-header validation, formatting, and pull-request quality checks.


Communication Between Planes

Implemented the initial communication between the control plane and data plane using direct cluster access.
Contributed to the separation of control-plane, data-plane, build-plane, and workflow-plane responsibilities.
Added support for multi-cluster execution patterns and Kubernetes authentication methods.
Helped evolve the BuildPlane abstraction into the WorkflowPlane abstraction as the model expanded beyond builds.


Workflow Plane, CI & Workflows

Owned the CI and workflow area of OpenChoreo.
Designed and implemented the CI model for OpenChoreo.
Implemented Workflow and WorkflowRun APIs for creating, listing, deleting, tracking, and observing workflow executions.
Extended the CI model into a generic Workflow Plane capable of running workflows such as database migrations, infrastructure provisioning, and other automation tasks.
Implemented Argo Workflows-based builds using Google Cloud Buildpacks, Paketo Buildpacks, Ballerina Buildpacks, Dockerfile builds, React builds, PHP builds, and web application workflows.
Added workflow logs, events, step-level status, TTL cleanup, validation, workflow references, and workload creation through CI.


Secret Management, Private Repositories & Registries

Introduced OpenBao as the open-source key vault used for OpenChoreo secret management.
Implemented Git secret management through the API server and developer portal.
Designed and implemented support for SSH-based and PAT-based private repositories.
Added support for AWS CodeCommit as a source provider.
Implemented private and external container registry support for image push and pull workflows.


Build Reliability & Security

Added build cache, registry cache, and buildpack image caching to improve build performance.
Fixed build reliability issues related to concurrent builds, workflow deletion, build status handling, Argo Workflow name limits, and Apple Silicon buildpack behavior.
Removed privileged root access from workflow templates to support least-privilege execution.
Fixed shell parameter injection risk in workflow templates.
Pinned build-related images by digest to improve reproducibility and supply-chain integrity.


AWS Observability Modules

Implemented AWS observability modules for OpenChoreo logs, metrics, distributed tracing, and Kubernetes events.
Built CloudWatch-backed log, metric, trace, and event-querying integrations.
Added build-log support and multi-cluster support for the AWS logs module.
Added AWS X-Ray tracing support and improved multi-cluster setup documentation.


Backstage Developer Portal

Extended the OpenChoreo Backstage developer portal for the CI and workflow capabilities built in the core platform.
Implemented Git secret creation flows and improved the related user experience.
Added AWS CodeCommit support to the developer portal.
Updated portal types and UI flows for schema, WorkflowPlane, and workflow-model changes.
Improved component creation steps and added workload details into workflow run views.


Samples & Documentation

Added PHP, React, Ballerina, GCP, Docker, buildpack, and multi-language sample workloads.
Created and updated workload YAMLs, sample README files, endpoint names, web application samples, and getting-started material.
Improved documentation for CI, Argo Workflows, private repositories, private registries, workflow templates, build caching, and multi-cluster observability.
Maintained samples across release branches and fixed release-specific sample issues.


Release Management

Acted as the release manager for three pre-GA OpenChoreo releases: 0.2.0, 0.5.0, and 0.7.0.
Contributed to release pipelines, Helm chart updates, version updates, release URL migrations, and release-readiness work.
Supported release-related Backstage enablement, sample updates, and configuration fixes.
