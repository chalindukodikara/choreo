I was part of the WSO2 Developer Platform (formerly Choreo) engineering team from July 2023 to October 2025. My work focused on its build and deployment pipelines, endpoint management, application configuration, and developer experience.


Build & Deployment Pipeline

Owned the backend redesign that split Choreo's build and deployment into independent pipelines.
Planned and rolled out the migration of all existing customers to the new build-and-deploy pipelines, with per-organization control to adopt it at their own pace.
Built organization-wide governance controls that let administrators enforce settings across the platform, such as requiring container vulnerability scans on builds.


Build Configuration Management

Identified a design problem in how build configurations were structured on the platform and initiated the effort to address it.
Owned the design and implementation of restructuring build configurations across all application types.
Planned and executed the migration of all existing applications to the new structure, rolling it out in phases with no user impact.


Endpoint Management

Served as one of the primary engineers responsible for endpoint management on the platform, maintaining and evolving how applications declare and expose their endpoints.
Designed and implemented UI-based endpoint configuration, enabling endpoints to be defined and managed directly in the UI rather than through a configuration file.
Refined the endpoint management process, making endpoint URLs customizable.


Developer Experience & Onboarding

Contributed to an effort to improve developer onboarding, analyzing and reducing the friction users faced when bringing their existing applications onto the platform.
Designed and delivered Choreo's Git submodule support, enabling applications that depend on submodules to be built on the platform.

Designed and built source-configuration validation into the build, catching configuration errors up front instead of surfacing them as deployment failures later.
Developed a migration tool for the platform's declarative configuration files, enabling users to upgrade to the latest format.


Platform Cost & Infrastructure Efficiency

Investigated rising container-registry storage usage across the platform's environments and built a mechanism that identifies stale images, providing the operations team with tooling to reclaim storage and reduce infrastructure costs.
Designed a tiered user image retention policy across free and premium plans, with configurable retention controls, to keep storage growth sustainable as the platform scaled.


Customer Support & Platform Reliability

Served on the customer support rotation, resolving production issues for enterprise customers.
Responded to production incidents, restoring service and addressing the underlying root causes.
Contributed to a platform stabilization effort, analyzing production logs to identify and reduce recurring server errors.



I have been part of the OpenChoreo team since October 2025. An open-source internal developer platform — where I serve as a project maintainer and lead its authorization system.


Authorization System Design & Architecture (RBAC)

Designed OpenChoreo's authorization system and its permission model.
Evaluated leading policy engines against the platform's needs and built the system on Casbin, enforcing access consistently across the platform's APIs, the developer portal, and the CLI.
Implemented Kubernetes-native policy storage, managing authorization roles and bindings as custom resources (CRDs).

Attribute-Based Access Control (ABAC) Extension

Designed an extensible attribute-based access control extension to the RBAC model, using expression-based conditions on role bindings.
Introduced a range of condition types, giving administrators fine-grained flexibility when authoring access policies.

Platform APIs & Developer Tooling

Designed a service-layer architecture that serves both internal and public APIs from a single implementation.
Contributed to a revamp of the OpenChoreo API server, re-implementing its APIs to follow canonical Kubernetes resource conventions.
Built the CLI and Backstage UI experience for managing authorization resources.
Authored the public documentation for the authorization model and contributed unit test coverage across the authorization, CLI, and service layers..

