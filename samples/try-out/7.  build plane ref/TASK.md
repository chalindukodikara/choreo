Currently, workflows get buildplane from either the projects buildplane ref or the 0th index buildplane in the namespace or the cluster.

We need to change that and each workflow should have the buildplane ref.

1. Add a buildplaneref field to the workflow spec.
api/v1alpha1/workflow_types.go

buildPlaneRef:
  kind: BuildPlane or ClusterBuildPlane
  name: buildplane-name

2. Remove the buildplane ref from the project spec.
api/v1alpha1/project_types.go

3. Add that to all the workflow samples.
- samples/getting-started/workflows
- samples/component-workflows

4. Update the workflow controller to use the buildplaneref from the workflow spec.
- internal/controller/workflowrun/controller.go

buildPlaneResult, err := controller.ResolveBuildPlane(ctx, r.Client, workflowRun)
if err != nil {
logger.Error(err, "failed to get build plane",
"workflowrun", workflowRun.Name,
"namespace", workflowRun.Namespace)
setBuildPlaneResolutionFailedCondition(workflowRun, err)
return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}
if buildPlaneResult == nil {
logger.Info("No build plane found for project",
"workflowrun", workflowRun.Name)
setBuildPlaneNotFoundCondition(workflowRun)
return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

5. Check any other place we need to fix.
