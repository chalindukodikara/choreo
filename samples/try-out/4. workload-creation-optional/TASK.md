- Currently, in the component workflow run controller, after everything is completed we check whether Workload is in the argo workflow.
- To check whether that step is there, we can check the tasks section in the status. If the `generate-workload-cr` step is there, then only we can try to create the workload.


References
- internal/controller/componentworkflowrun/controller.go
- api/v1alpha1/componentworkflowrun_types.go
