## Instructions
I need to get write a service letter from my experience.

Sample: samples/try-out-2/15-service-letter-3/sample.md
- Need to be written in point form just like in, the sample. 

Context
- samples/try-out-2/14-service-letter-2/areas-worked-on.md
- samples/try-out-2/14-service-letter-2/areas-worked-on-fable.md

Repos:
https://github.com/wso2-enterprise/choreo-console/graphs/contributors?all=1

https://github.com/wso2-enterprise/choreo-cp-declarative-api/graphs/contributors?all=1

https://github.com/wso2-enterprise/choreo-cp-configuration-service/graphs/contributors?all=1

https://github.com/openchoreo/openchoreo/graphs/contributors?all=1

https://github.com/openchoreo/community-modules

https://github.com/openchoreo/sample-workloads

https://github.com/openchoreo/backstage-plugins 

https://github.com/wso2-enterprise/choreo/issues?q=is%3Aissue%20state%3Aclosed%20assignee%3Achalindukodikara

https://github.com/wso2-enterprise/choreodp-rudder

https://github.com/openchoreo/openchoreo/issues?q=is%3Aissue%20state%3Aclosed%20assignee%3Achalindukodikara 

https://github.com/wso2-enterprise/choreo-cp-env-overlay

https://github.com/wso2-enterprise/choreo-control-plane

https://github.com/wso2-enterprise/choreodp-cloud-manager


Write the final file into: samples/try-out-2/15-service-letter-3/service-letter.md

## Example details of me

I, Chalindu Malshika Kodikara Kodikara Arachchige worked at WSO2 LLC from July 15, 2023 to July 31, 2026. I served as a Software Engineer from July 15, 2023 to July 31, 2025, and as a Senior Software Engineer from August 1, 2025 to July 31, 2026.

I was part of the WSO2 Developer Platform (formerly Choreo) engineering team from July 2023 to February 2024. My work focused on configuration management, etc. 

Configuration Management
I designed and implemented the configuration management service to handle the configuration of users and internal configurations.
I designed the UI for that using React. (confirm this: https://github.com/wso2-enterprise/choreo-console)
I owned this until my move to OpenChoreo

Customer Support & Platform Reliability

Served on the customer support rotation, resolving production issues for enterprise customers.
Responded to production incidents, restoring service and addressing the underlying root causes.
Contributed to a platform stabilization effort, analyzing production logs to identify and reduce recurring server errors.

-
-

Then, I was part of the research team that researched on a solution to develop an improved version of WSO2 Developer platform from the lessions learnt. Then, it lead to the start of OpenChoreo in 2025 January as a small team. Then, since it's 0.1.0 release as a bootstrap contributor to its 1.2.x release, I serve as a maintainer of the project. 

Research phased lasted from January 2024 to January 2025.
Research Phase
Part of the team that researched on a reconciliation based solution just like k8s. performed a performance test on postgres using Azure VMs.
The goal of this test was to evaluate the query performance of a database with a large data volume of approximately 50 million records and to identify its limitations. Specifically, we aimed to determine the maximum data volume the database can handle while keeping search query performance within a 1-second threshold. Additionally, we tested the database under concurrent access scenarios, scaling up to 1000 concurrent users.

Initial Version of OpenChoreo without k8s
I worked on the initial implementation of OpenChoreo where we tried to implement our version of k8s with our own yaml structure and controller runtime to support our own needs.
I worked on mainly the data handling part for different databases.
I also worked on controller runtime implementations.

Need to query PRs, commits, to find out: https://github.com/wso2-enterprise/choreo-cp-declarative-api/tree/v3-api-definition
https://github.com/wso2-enterprise/choreo-cp-declarative-api/tree/v3-revision1

Then, later we moved on to k8s based OpenChoreo in January 2025. Since then, now until July 31st of 2026, I am the highest contributor to OpenChoreo. https://github.com/openchoreo/openchoreo. Worked closely with k8s and docker.

Initial Project
I worked on the initial implementation of OpenChoreo using Kuberbuilder project.
I worked on initial deployment, build, project, and component controllers.
Added tools required for license checking and formatting.

Communication Between Planes
I implemnted on the intial communication between planes using direct communication.

Git Secret Management (can we merge this with Private Repository and Registry Support?)
Introduced OpenBao opensource key vault for secret management.
Implemented the secret management capability through the API Server and UI for git secrets with the key vault which led to further improvements for other secrets. 

Private Repository and Registry Support
Designed and implemented the support for SSH and PAT based repositories. Implemented its APIs and UI/UX.

Workflow Plane, CI and Workflows
I owned this area.
I designed and implemented a model for the CI for OpenChoreo.
I expanded the model to support generic workflows in OpenChoreo to run any workflow such as database migrations, infrastructure provisioning, etc.
Implemented different builds: Buildpacks and language support was provided through argo workflows using google cloud buildpacks, paketo buildpacks, ballerina buildpacks and docker builds.

AWS Observability Modules
I implemented the observability modules for AWS for logs, distributed tracing, and metrices using AWS CloudWatch and AWS XRay.

Samples and Documentation
Added PHP, React, Ballerina, GCP, Docker, buildpack, and multi-language samples and their documentations.

Release Management
Acted as the release manager for 3 releases before GA. 0.2.0, 0.5.0, and 0.7.0.

