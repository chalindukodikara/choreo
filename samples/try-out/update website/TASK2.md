Previously, we had component workflows for component related ci workflows including a system parameters section for some set of required params such as repo. And we had workflows gor generic any kind of workflows.

Now, we removed Component Workflows and we only have Workflows. We use that for both component related and generic workflows. We need to update the documentation of the website to reflect this change.

Component workflow still differs since we add the two annotations,
one is to mark this is a component workflow: "openchoreo.dev/component-workflow: "true""
other one is for UI and auto-build: "openchoreo.dev/component-workflow-parameters: repoUrl: parameters.repository.url, repoBranch: parameters.repository.revision.branch"

And component workflowRun has two labels to link it to the component and project: "openchoreo.dev/component: greeter-service" and "openchoreo.dev/project: default", only for component related workflow runs.
They can access those labels when they define the workflow CR where they can access those labels via CEL as "metadata.labels['openchoreo.dev/component']" and "metadata.labels['openchoreo.dev/project']".

WorkflowRuns which refers to an arg workflow with generate-workload-cr step and workload-cr output, will be special cased where the controller will read that CR and create inside the control plane. This is given to simplify the workload creation process. Otherwise, you need to call the api server and get it created.
So controller has this special case logic and it adds a condition for that as well.

We have more new concepts such as external refs, and so on.

1. Update API References
- website/docs/reference/api/application/workflowrun.md
- website/docs/reference/api/platform/workflow.md

2. Find how to organize the workflow related docs
Since we only have one type of workflow and we are using that for both component related and generic workflows, we need to find a good way to organize the documentation.

Should we go like this?
- user-guide/workflows/overview.md
- user-guide/workflows/CI/*

or
- user-guide/ci/overview.md
- user-guide/generic-workflows/*

or what?

Should these come under user-guide or any other folders inside "website/docs/"?

3. Update the docs for the suggested organization and update the content to reflect the changes. We need to update the whole content as there are many new concepts.
