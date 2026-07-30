When a component is deleted, any workflowruns related to that component should be deleted when the component is in the finalizers.

1. Filter related workflowruns from the labels, it has "opencoreho.io/component" and "opencoreho.io/project" labels. So we can filter the workflowruns with those labels.
Delete all the workflowruns related to that component.

2. Check whether when a workflowrun is deleted, it is removing the argo workflow and any other resources created for that workflowrun. Check the workflowrun controller and the finalizer logic to see whether it is doing that.

3. Check whether there is any other place we need to update in the codebase to reflect this change. And evaluate whether everything is working fine with this change.
