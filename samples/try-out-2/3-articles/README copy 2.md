# Designing Workflows in OpenChoreo

## Introduction

When a team tries out an internal developer platform, one of the first questions is not about the UI, the logo, or the dashboard. The first question is usually much more direct: "Can this platform build and deploy my code in the way my team needs?"

That question sounds simple, but it hides many smaller questions. Can the platform build from a public GitHub repository? Can it build from a private Git repository? Can it use SSH credentials, basic auth credentials, or AWS CodeCommit credentials? Can it build with a Dockerfile? Can it build with buildpacks? Can it run a security scan? Can it create a database before the application is deployed? Can it run a one-off task that is not connected to an application at all?

These are not edge cases for a developer platform. These are normal platform needs. Every organization has its own source control setup, security rules, build tools, deployment rules, and approval process. If the platform only supports one fixed build path, it may work well for the demo, but it will become hard to use in real teams.

This is the problem we wanted to solve with OpenChoreo Workflows.

We did not want to create one special feature for Docker builds, another feature for buildpacks, another feature for database provisioning, and another feature for generic automation. That path usually leads to many similar APIs that behave slightly differently. It also makes the platform harder to explain. Instead, we wanted one workflow model that can support CI workflows and also support other automation tasks.

The result is a small set of Kubernetes custom resources. The main two are `Workflow` and `WorkflowRun`. A `Workflow` defines what can be run. A `WorkflowRun` starts one execution of that workflow. The same model can be used to build a component, run a report, provision infrastructure, or trigger any other automation that the platform team wants to expose.

In this article, I will walk through why OpenChoreo Workflows are designed this way, how the main resources fit together, and how we separate platform engineer concerns from developer concerns. I will use the Dockerfile build workflow as the main example because it shows most of the important parts: developer parameters, hardcoded platform decisions, reusable workflow steps, private repository secrets, extra resources, and workflow run status.

## The Problem: Automation Is Not Only CI

Most developer platforms start by solving source-to-image builds. This makes sense because building an application from source code is the most common first step before deployment. A developer creates a component, gives the platform a Git repository, chooses a build method, and expects a container image to be produced.

But after the first use case works, more requests come in.

A team may ask for a workflow that creates a database. Another team may ask for a workflow that creates a repository in GitHub. Someone else may want a workflow that calls an internal API to register a service. A security team may want a workflow that runs a scan and stores the result. A platform engineer may want to run a cleanup job. These are all workflows, but they are not all CI workflows.

If the platform has a hardcoded "build workflow" concept, every non-build use case becomes awkward. The platform either needs a second abstraction for generic jobs, or users must force generic automation into the build model. Both options are poor.

OpenChoreo takes a different path. A build is just one kind of workflow. A generic automation task is also a workflow. What changes is the template, the parameters, and how the UI or CLI presents it to users.

This gives us a more useful base model:

- A platform engineer defines a reusable workflow.
- A developer or another user provides only the values that should change for a specific run.
- OpenChoreo renders the final workflow engine resource and applies it to the workflow plane.
- OpenChoreo tracks the execution status in a vendor-neutral way.

That last point is important. Today, OpenChoreo uses Argo Workflows as the default workflow engine. The `runTemplate` normally renders an Argo `Workflow`. But the OpenChoreo API is not trying to expose every Argo concept directly to developers. The developer sees a `WorkflowRun`, parameters, logs, and tasks. The platform engineer can still use Argo-specific features inside the workflow definition.

This split gives both personas what they need.

## The Core Abstraction

The main design is simple:

- `Workflow` is the reusable definition.
- `ClusterWorkflow` is the cluster-scoped version of the same idea.
- `WorkflowRun` is one execution of a `Workflow` or `ClusterWorkflow`.
- `WorkflowPlane` or `ClusterWorkflowPlane` tells OpenChoreo where workflow execution should happen.

The names are close to other workflow systems, but the split is important in OpenChoreo. A `Workflow` is not a running job. It is a template with a parameter schema, a workflow engine template, optional extra resources, and optional external references. A `WorkflowRun` points to that template and provides parameter values.

This is similar to how many systems separate a class from an object, or a deployment template from one deployment. The workflow definition can be reused many times. Each run can use different input values.

For example, a Dockerfile builder workflow can be defined once by a platform engineer. One developer can use it to build a service from `service-go-greeter`. Another developer can use the same workflow to build a web app from a different repository path. Both are using the same platform-approved build process, but each run still has its own repository URL, app path, Dockerfile path, and build arguments.

This is the heart of the design. We want reuse without removing flexibility.

## The Workflow Plane

OpenChoreo has the idea of different planes. The control plane stores and reconciles the platform resources. The data plane runs user workloads. The observability plane handles monitoring and logs. Workflows have their own execution concern, so OpenChoreo also has a workflow plane.

The workflow plane is the Kubernetes cluster or namespace where workflow engine resources are applied. With the current default setup, this means Argo Workflow resources are created in the workflow plane. The workflow plane also has access to the secret store that workflows need when they create temporary secrets for Git or registry access.

The workflow plane can be dedicated if workflow load is high or if the organization wants stronger isolation. In smaller setups, the same physical cluster may host more than one plane. The API still keeps the concept separate because the concern is separate: workflow execution has different scaling, security, and lifecycle needs from application serving.

A workflow chooses a workflow plane through `workflowPlaneRef`. If it is omitted, the default is the cluster workflow plane named `default`.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: dockerfile-builder
spec:
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
```

This small field matters because the platform team can decide where a workflow should run. A heavy image build workflow can run in a workflow plane with enough CPU, memory, storage, and network access. A lightweight report workflow can run somewhere else if needed.

## Separating Platform Engineer and Developer Concerns

One of the main goals of OpenChoreo Workflows is to keep the platform engineer and developer responsibilities clear.

Developers should not need to understand every detail of Argo Workflows, service accounts, external secret templates, registry push secrets, or shared volumes. They should provide the values that are specific to their application or task.

Platform engineers should be able to lock down the parts that must be consistent across the organization. They should define the build flow, choose the base images, wire the service account, decide where images are pushed, decide which steps run, and control which workflow types are allowed for which component types.

In practice, workflow inputs usually fall into three groups.

The first group is hardcoded platform values. These are values the platform engineer writes directly in the workflow template. For example, the image tag strategy may be fixed. A security scan may always be enabled. A service account may always be `workflow-sa`. The developer does not need to pass these values, and in some cases should not be allowed to change them.

The second group is developer-provided values. These are values that change from one component or run to another. Examples include the Git repository URL, the application path, the Dockerfile path, build arguments, and build environment variables. These values are described in the workflow parameter schema and supplied in the `WorkflowRun`.

The third group is system-provided values. These values come from OpenChoreo at runtime. Examples include the workflow run name, the control plane namespace name, workflow run labels, and the workflow plane secret store. These values are useful when rendering the final workflow engine resource.

This split is one of the most useful parts of the design. It lets a platform team offer a workflow as a product. Developers get a clean form or CLI command. Platform engineers keep control over the execution model.

## Anatomy of a Workflow

A `Workflow` or `ClusterWorkflow` has a few important sections:

- `parameters` defines the input schema for the run.
- `runTemplate` defines the workflow engine resource that will be rendered and applied.
- `resources` defines extra Kubernetes resources that must be created before the run starts.
- `externalRefs` resolves other OpenChoreo resources and makes their data available to templates.
- `ttlAfterCompletion` defines how long completed workflow runs should stay around.

The Dockerfile builder workflow is a good example. Its parameter schema asks for repository settings, Docker build settings, build environment variables, and build arguments.

```yaml
parameters:
  openAPIV3Schema:
    type: object
    required:
      - repository
    properties:
      repository:
        type: object
        required:
          - url
        properties:
          url:
            type: string
            description: "Git repository URL"
          secretRef:
            type: string
            default: ""
            description: "Secret reference name for Git credentials"
          revision:
            type: object
            default: {}
            properties:
              branch:
                type: string
                default: main
              commit:
                type: string
                default: ""
```

This schema is not only for validation. It is also a contract for the UI and CLI. Because the schema is OpenAPI v3 based, the platform can render strings, arrays, enums, defaults, required fields, and nested objects in a structured way. The schema makes the workflow self-describing.

The `runTemplate` is the execution template. In the default OpenChoreo workflow setup, this is usually an Argo `Workflow`. It can use CEL expressions such as `${parameters.repository.url}` or `${metadata.workflowRunName}`. OpenChoreo evaluates these expressions before applying the rendered resource to the workflow plane.

```yaml
runTemplate:
  apiVersion: argoproj.io/v1alpha1
  kind: Workflow
  metadata:
    name: ${metadata.workflowRunName}
    namespace: ${metadata.namespace}
  spec:
    arguments:
      parameters:
        - name: git-repo
          value: ${parameters.repository.url}
        - name: branch
          value: ${parameters.repository.revision.branch}
        - name: workflowrun-name
          value: ${metadata.workflowRunName}
```

The important idea is that the developer does not submit this Argo workflow directly. The platform engineer owns it. The developer submits a `WorkflowRun` with parameter values. OpenChoreo combines the definition, the run, and the runtime context to produce the final execution resource.

## CEL Expressions as the Glue

OpenChoreo uses CEL expressions inside workflow templates to connect runtime values to the workflow engine resource. CEL is used because it can express simple value lookups and also handle more useful logic when needed.

The most common expression sources are `parameters`, `metadata`, `workflowplane`, and `externalRefs`.

`parameters` contains the values supplied by the developer, after defaults from the schema are applied. For example, `${parameters.repository.url}` gives the Git repository URL. `${parameters.buildArgs}` gives the build arguments array.

`metadata` contains system values. `${metadata.workflowRunName}` gives the name of the `WorkflowRun`. `${metadata.namespaceName}` gives the control plane namespace where the run was created. `${metadata.namespace}` gives the enforced workflow execution namespace. `${metadata.labels['openchoreo.dev/component']}` gives a label from the `WorkflowRun`.

`workflowplane` contains values from the selected workflow plane. The common example is `${workflowplane.secretStore}`, which gives the External Secrets Operator `ClusterSecretStore` name attached to the workflow plane.

`externalRefs` contains resolved external custom resources. For example, a workflow can resolve a `SecretReference` and then use `${externalRefs['git-secret-reference'].spec.data}` when rendering an `ExternalSecret`.

This context gives the workflow definition enough information to render a complete resource without making developers pass every low-level value manually.

## Reusing Workflow Steps

OpenChoreo Workflows are more useful when the workflow engine steps are also reusable. In the Dockerfile builder, the `runTemplate` uses Argo `templateRef` to call separate `ClusterWorkflowTemplate` entries.

The flow is roughly:

- Check out the source repository.
- Build the image with the Dockerfile or Containerfile.
- Publish the image to the registry.
- Generate the workload custom resource for OpenChoreo.

Each of these can be maintained as a separate workflow template. That means the platform team can update the checkout step in one place, improve the image build step in one place, and reuse the publish step across multiple builders.

Here is a simplified part of the build workflow:

```yaml
templates:
  - name: build-workflow
    steps:
      - - name: checkout-source
          templateRef:
            name: checkout-source
            clusterScope: true
            template: checkout
      - - name: build-image
          templateRef:
            name: containerfile-build
            clusterScope: true
            template: build-image
          arguments:
            parameters:
              - name: git-revision
                value: '{{steps.checkout-source.outputs.parameters.git-revision}}'
              - name: build-env
                value: '{{workflow.parameters.build-env}}'
              - name: build-args
                value: '{{workflow.parameters.build-args}}'
```

This design gives two levels of reuse. The OpenChoreo `Workflow` is reusable across components and runs. The Argo templates used inside the workflow are reusable across OpenChoreo workflows.

That matters in a real platform. A Dockerfile builder, a Paketo buildpacks builder, and a Google Cloud buildpacks builder may all need the same checkout and publish behavior. They should not each carry a copy of the same logic.

## Private Repository Support

Private repositories are a common requirement. A platform cannot assume all source code is public. OpenChoreo supports this by combining developer parameters, `SecretReference`, `externalRefs`, and workflow resources.

The developer provides a `repository.secretRef` value when the repository needs credentials. That value points to a `SecretReference` in OpenChoreo. The `SecretReference` describes how to fetch the actual secret values from an external secret store.

The workflow declares an external reference:

```yaml
externalRefs:
  - id: git-secret-reference
    apiVersion: openchoreo.dev/v1alpha1
    kind: SecretReference
    name: ${parameters.repository.secretRef}
```

At runtime, OpenChoreo evaluates the `name`, fetches the referenced `SecretReference`, and injects its spec into the CEL context under `externalRefs['git-secret-reference']`. Then the workflow can use that data to render an `ExternalSecret` in the workflow plane.

```yaml
resources:
  - id: git-secret
    includeWhen: ${has(parameters.repository.secretRef) && parameters.repository.secretRef != ""}
    template:
      apiVersion: external-secrets.io/v1
      kind: ExternalSecret
      metadata:
        name: ${metadata.workflowRunName}-git-secret
        namespace: ${metadata.namespace}
      spec:
        secretStoreRef:
          kind: ClusterSecretStore
          name: ${workflowplane.secretStore}
        target:
          name: ${metadata.workflowRunName}-git-secret
          creationPolicy: Owner
          template:
            type: ${externalRefs['git-secret-reference'].spec.template.type}
```

This solves an important problem. The workflow needs a Kubernetes secret in the workflow plane, but the developer should not paste raw credentials into the workflow run. The control plane can keep a reference to the secret, and the workflow plane can create a short-lived secret for the run.

The Dockerfile builder supports common Git credential types such as SSH keys, username/password credentials, and AWS CodeCommit credentials through `git-remote-codecommit`. The exact keys are described in the parameter schema. This makes private repository support part of the reusable workflow instead of a separate one-off process.

## Extra Resources Before the Run

A workflow engine resource is often not enough by itself. The run may need secrets, config maps, service account access, or other Kubernetes resources before it starts.

That is why OpenChoreo has the `resources` section. Each entry is a Kubernetes resource template. OpenChoreo renders these resources with the same CEL context and applies them to the workflow plane before the main run resource is applied.

In the Dockerfile builder, the workflow creates a Git secret only when `repository.secretRef` is set. It also creates a registry push secret so the image can be pushed to the configured registry.

This resource lifecycle is tied to the workflow run. OpenChoreo tracks the resources it applied in the `WorkflowRun` status. When the run is deleted, the controller has the information it needs to clean up the related resources.

This is better than asking every workflow step to create and delete its own support resources. The lifecycle is controlled by the platform resource model.

## What a WorkflowRun Looks Like

After the platform engineer defines the workflow, a user can start it by creating a `WorkflowRun`. The run references the workflow and provides parameter values.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: greeting-service-build-01
  labels:
    openchoreo.dev/project: "default"
    openchoreo.dev/component: "greeting-service"
spec:
  workflow:
    kind: ClusterWorkflow
    name: dockerfile-builder
    parameters:
      repository:
        url: "https://github.com/openchoreo/sample-workloads"
        revision:
          branch: "main"
        appPath: "/service-go-greeter"
      docker:
        context: "/service-go-greeter"
        filePath: "/service-go-greeter/Dockerfile"
```

This is the object that represents one execution. It is small compared to the workflow definition because it only contains the values that change for this run.

When the controller sees the `WorkflowRun`, it resolves the referenced `Workflow` or `ClusterWorkflow`. It resolves the target workflow plane. It applies parameter defaults from the workflow schema. It resolves external references. It renders extra resources. It renders the workflow engine resource. Finally, it applies everything to the workflow plane.

After that, the status of the `WorkflowRun` becomes the main place to see what happened.

The status includes the actual run resource reference. For Argo, this points to the Argo `Workflow` created in the workflow plane. The status also includes the extra resources that were applied, start and completion times, conditions, and tasks.

The `tasks` field is important because it gives a vendor-neutral view of workflow steps. Today, those tasks are extracted from Argo workflow nodes. In the future, if OpenChoreo supports another workflow engine, the external status shape can stay similar even if the engine internals are different.

## CI Workflows and Generic Workflows

OpenChoreo Workflows can be used in two broad ways.

The first way is as a CI or component workflow. This is a workflow used to build source code for a component. The Dockerfile builder is an example. These workflows are usually shown in the UI when a developer creates or builds a component.

The second way is as a generic workflow. This is an automation task that is not tied to a component. For example, the GitHub stats report sample fetches repository data from the GitHub API, transforms it, and prints a report. A database provisioning workflow is another example. These workflows still use the same `Workflow` and `WorkflowRun` model, but the UI and CLI may present them differently.

OpenChoreo identifies component workflows with a label:

```yaml
metadata:
  labels:
    openchoreo.dev/workflow-type: "component"
```

This label is intentionally simple. It does not change the core workflow behavior. It helps the UI and CLI categorize workflows and provide a better user experience.

Component types can also restrict which workflows are allowed. For example, a web application component type may allow the Dockerfile builder and a buildpacks builder, while another component type may allow a different set.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterComponentType
metadata:
  name: webapp
spec:
  allowedWorkflows:
    - kind: ClusterWorkflow
      name: dockerfile-builder
    - kind: ClusterWorkflow
      name: gcp-buildpacks-builder
```

This gives platform engineers control over which build paths are available for each kind of component. It also avoids showing users workflows that do not make sense for their component type.

## UI and CLI Support

A good workflow API is not enough. Developers usually interact with workflows through a UI or CLI. That means the workflow definition must carry enough information for those tools to guide the user.

The OpenAPI v3 parameter schema handles the basic shape: strings, arrays, objects, defaults, required fields, and enums. OpenChoreo also supports vendor extensions for fields that have special meaning in component workflows.

Current component workflow extensions include:

- `x-openchoreo-component-parameter-repository-url`
- `x-openchoreo-component-parameter-repository-branch`
- `x-openchoreo-component-parameter-repository-commit`
- `x-openchoreo-component-parameter-repository-app-path`
- `x-openchoreo-component-parameter-repository-secret-ref`

These extensions tell the UI and CLI which parameter is the repository URL, which one is the branch, which one is the commit, which one is the app path, and which one is the secret reference. The tool can then provide a smoother experience. For example, it can show a repository field in the right place, list available secret references, or map component creation inputs into workflow parameters.

This keeps the workflow model generic while still allowing a polished component workflow experience.

If a new special field is needed later, the workflow schema can add another extension. The UI and CLI must also be updated to understand it. That is a fair tradeoff because the core API stays clean, and product-specific behavior stays explicit.

## Garbage Collection and Lifecycle

Workflow runs should not live forever unless the platform wants that behavior. Build runs, temporary secrets, and intermediate resources can pile up quickly.

OpenChoreo supports `ttlAfterCompletion` on the workflow. The value is copied to the workflow run if the run does not set its own value. The format is a duration string such as `1d`, `30m`, or `1h30m`.

```yaml
spec:
  ttlAfterCompletion: "1d"
```

When a workflow run completes, it can be deleted after the TTL. If no TTL is defined, the run is not automatically deleted by this setting.

The cleanup story also matters for resources created by the workflow. OpenChoreo tracks extra resources in the workflow run status. When the `WorkflowRun` is deleted, related resources can be cleaned up. For component workflows, runs that belong to a component can also be cleaned up as part of the component lifecycle.

This is one more reason to keep workflow execution represented as a Kubernetes resource. The normal resource lifecycle gives the platform a clear place to track ownership, status, and cleanup.

## Why This Design Works

The design is useful because it is small but flexible.

The `Workflow` resource gives platform engineers a place to define the reusable contract. It includes the parameter schema, the execution template, extra resources, external references, and TTL. The `WorkflowRun` resource gives users a small object to start one execution. The workflow plane gives the platform a clear execution target. CEL expressions connect the pieces without making every workflow hardcoded in the controller.

This also avoids building a separate API for every automation type. Docker builds, buildpack builds, GitHub reports, database creation, and other automation can all use the same model. The difference is in the workflow definition, not in the platform core.

There is also a clear persona boundary. Developers provide values. Platform engineers define the process. OpenChoreo handles rendering, applying, status tracking, and cleanup.

That boundary is the main reason the abstraction is worth having. Without it, developers either need to learn too much about the workflow engine, or platform engineers need to hardcode too many special cases into the platform.

## Current Limitations and Future Direction

Today, Argo Workflows is the default workflow engine used by OpenChoreo. The model already keeps developers away from most Argo-specific details, but the controller still understands Argo when it applies and reads workflow execution status.

We are discussing how to make the workflow engine more modular. In the future, a workflow module could allow different engines or external systems to back the same OpenChoreo workflow model. Possible directions include Kubernetes-native execution, Tekton Pipelines, GitHub Actions, Azure Pipelines, or other systems.

That does not mean every engine will support every feature in exactly the same way. The useful part is to keep the OpenChoreo-level contract stable: a workflow definition, a run, parameters, status, tasks, logs, and lifecycle. Engine-specific features can still exist behind the template where platform engineers need them.

This is also why the `WorkflowRun` task status is designed as a vendor-neutral view. It gives OpenChoreo a stable status shape even when the underlying engine has its own internal model.

## Conclusion

Workflows are a central part of an internal developer platform because they represent how real work gets done. Building source code is one workflow. Provisioning infrastructure is another. Running a report is another. A platform that treats each of these as a separate hardcoded feature becomes harder to extend over time.

OpenChoreo Workflows use a simple model instead. A `Workflow` defines a reusable automation contract. A `WorkflowRun` starts one execution. A workflow plane runs the rendered workflow engine resource. Parameters, metadata, workflow plane data, and external references are combined through CEL expressions. Extra resources can be created before the run, and status is tracked back on the `WorkflowRun`.

The result is a workflow system that gives platform engineers control without making developers deal with low-level execution details. It works for CI workflows today, and it also gives OpenChoreo a base for more generic automation in the platform.
