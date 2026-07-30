Want to run argo workflows without root access as it is hard to have root access in a cloud environment and enterprises reluctant to have such privilege.

References
How openchoreo workflow and workflow run works
- api/v1alpha1/workflowrun_types.go
- api/v1alpha1/workflow_types.go
- internal/controller/workflowrun/*

Sample openchoreo workflows
- samples/getting-started/ci-workflows/*

Workflow runs
- samples/from-source/services/go-google-buildpack-reading-list/reading-list-service.yaml

Workflow templates which uses priveleage (root access for podman). Securitycontext.
- samples/getting-started/workflow-templates/gcp-buildpacks-build.yaml
- samples/getting-started/workflow-templates/containerfile-build.yaml
- samples/getting-started/workflow-templates/paketo-buildpacks-build.yaml
- samples/getting-started/workflow-templates/ballerina-buildpack-build.yaml
