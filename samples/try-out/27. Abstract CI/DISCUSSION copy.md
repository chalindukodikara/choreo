Discussion: Abstract OpenChoreo CI Module

Problem
cuRRENTLY, Openchoreo only supports argo workflows and there is no mechanism for extension

Proposal
Propose the following architecture.
1.png

- There will be a CI module with a workflow run controller and a service (adapter)
- Workflow run controller will know how to handle particular CI such as github actions, azure pipelines, jenkins, argo workflows, etc. Adapter will have the standardized APIs and it will know how to get logs, runs, etc.
- There can be multiple CI modules installed to the control plane and now workflows should distinugish between those and know which adapter to call to.

2.png

- WorkflowPlane CR will have `adapterServiceName: argo` which is the service name of the adapter api k8s service name. Can use internally for calling it.
- WorkflowPlane will only be available for k8s native ones, other ones doesn't have it. For those, the same field will be available at Workflow/ClusterWorkflow CR to override.
- If there are multiple workflow run controllers, how do they pick the correct run? workflow will have a field called class. It can be argo, tektone, etc defined by the module writer. Relevant controller will listen to their workflow runs only through watching.

Options
1. Go with above one
Pros
- Can be used by CLI, MCP for logs, etc
- Can support any CI
- If we are moving away from backstage, current support for jenkins, etc through backstage can also be still supported through this

Cons
- Bit complex

2. Only support k8s native engines such as argo and tekton and other supports through backstage
Pros
- Simple, less complex for users

Cons
- Controller needs to support main native engines and not pluggable

Questions
1. Should we go with Option 1 or 2? I prefer option 1.
2. We consider workflow plane cr only available if there is a plane? should we have it even for cases without a plane?
