Need to remove projectName and componentName from the annotation and parameters sections of the workflows. And make the labels available in the CEL context.

Current Workflow: samples/try-out/8. add labels to cel context/current.yaml
New One: samples/try-out/8. add labels to cel context/new.yaml

We should get the labels from the WorkflowRun and add them to the CEL context and let users use those labels.

Tasks
1. Remove the ProjectName and ComponentName from the annotation and parameters sections of the workflow. And use labels to pass to the workflow. So any label added to the workflowrun can be used in the CEL context.
- samples/getting-started/workflows/*
- samples/component-workflows/*

2. Update the workflowrun controller to add the labels of the workflowrun to the CEL context. And use it for rendering. Check the rendering logic to see whether it is correctly doing it.
- internal/controller/workflowrun/controller.go
- internal/pipeline/workflow/*

3. Update READMEs with the current workflow samples and its content to reflect the changes. Check whether those readmes are upto date with the newest workflow spec.
- samples/component-workflows/README.md
- samples/README.md (now component workflow CRs are not there)
- samples/getting-started/README.md

4. Evaluate whether there is any other place we need to update in the codebase to reflect this change. And evaluate whether everything is working fine with this change.
