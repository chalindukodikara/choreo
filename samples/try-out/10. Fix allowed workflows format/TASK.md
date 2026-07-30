### Current Limitation

The `spec.allowedTraits` field defines allowed traits that can be attached to a component of this type, and `spec.allowedWorkflows` field defines allowed component workflows that can be used for a component of this type.

This is a minor point, but note the inconsistency: `allowedTraits` uses a structured object with a kind and a name, while `allowedWorkflows` is just a flat list of strings. They both reference named resources; they should follow the same pattern.

```yaml
spec:
allowedTraits:
- kind: Trait
name: observability-alert-rule

allowedWorkflows:
- google-cloud-buildpacks
- docker
```

### Suggested Improvement


```yaml
spec:
allowedTraits:
- kind: Trait
  name: observability-alert-rule

allowedWorkflows:
- kind: Workflow
  name: google-cloud-buildpacks
- kind: Workflow (default and there is no any other kind of workflow for now, this is the only value we can have here)
  name: docker
```

1. Do this change in the component type spec and update the codebase to reflect this change.

2. Check whether component controller is doing any validation for the allowed workflows. And fix that to reflect the new format of allowed workflows.

3, Check whether there is any other place we need to update in the codebase to reflect this change. And evaluate whether everything is working fine with this change.

4. Add validations for component controller to check whether the workflow attached to the component is in the allowed workflows list of the component type. And it has an annotation called "openchoreo.dev/workflow-scope: component" to make sure that only component scoped workflows are attached to the component. Otherwise, component should not be ready.