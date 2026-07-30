We have decided to add annotations to workflows.
More details, read: samples/try-out/2. component-workflows/discussion.md

New Tasks
Update the annotation into "backstage.io/component-workflow-parameters" since the backend will avoid using the annotations.
1. WorkflowRun controller will temporary use this until we implement a new approach.

2. we will ignore auto-build which uses this annotation, as we will fix it later.

3. update sample workflows.
- samples/getting-started/workflows/*
- samples/component-workflows/docker.yaml
- samples/component-workflows/react.yaml
- samples/component-workflows/google-cloud-buildpacks.yaml
