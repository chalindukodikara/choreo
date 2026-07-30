# Workflows in OpenChoreo: One Model for CI and Automation

## Introduction
When a team tries an internal developer platform (IDP), the first questions are usually not about architecture diagrams. They are about day-to-day work.

How do I build my code? How do I deploy it? Can I build from a private repository? Can I add security scans? Can I run a database migration? Can I provision a database? Can I trigger a scheduled job when a release goes out?

Most platforms answer these with a mix of different tools: one for CI, another for infra automation, a third for one-off jobs, and a set of glue scripts in between. That works for a while, but it becomes hard to manage over time. Teams end up with many ways to do the same thing. Reuse is low, and every team has a slightly different workflow. Security and governance are also hard, because the platform team has to inspect every new pipeline or script.

In OpenChoreo, we wanted a simpler story:

1. A platform engineer should be able to define reusable workflows once.
2. Developers should be able to run those workflows safely with only the inputs they are allowed to provide.
3. CI workflows and “generic automation” workflows should feel like the same product, not two different products.

This post explains how we designed OpenChoreo Workflows, why we ended up with two APIs (`Workflow` and `WorkflowRun`), and how we keep platform concerns and developer concerns separate without slowing anyone down.

## The Problem We Were Solving
If you zoom out, “workflow” sounds like a fancy word. In practice it is just automation.

For CI, automation means things like:

1. Check out code
2. Build an artifact (container image, JAR, binary, etc.)
3. Scan it
4. Push it to a registry
5. Store logs and reports

For platform automation, it means things like:

1. Provision infrastructure (a database, a bucket, a queue)
2. Create secrets and config
3. Run a migration
4. Run a data job
5. Run a cleanup task

Even though these tasks are different, they share the same shape:

1. A reusable definition should exist.
2. Each execution should take some inputs.
3. It should run somewhere.
4. It should report status.
5. It should be auditable and clean up after itself.

The first design decision was to treat these as one model, not separate features.

## What We Wanted From Workflows
We iterated on workflows a few times. The requirements stayed consistent:

1. **Reusability**  
   The same workflow definition should work across projects and teams. We should not copy/paste a “docker build” pipeline 50 times.

2. **Clear ownership (platform vs developer)**  
   A platform engineer should be able to say: “This is the way we build images here.”  
   A developer should be able to say: “This is the repo and the path to my service. Run the build.”

3. **A single model for CI and generic automation**  
   If we support a CI workflow model, we should not force users to learn a separate model for infra workflows.

4. **A good UI/CLI experience**  
   We should not make developers edit long YAML for common actions. The schema should be first-class so the UI and CLI can render sensible input forms.

5. **Multi-tenant safety**  
   Workflows often need credentials. They also run code. Both are risky areas. We needed a model that supports guardrails and isolation.

6. **Scale without turning the control plane into a bottleneck**  
   Automation can be noisy. A busy organization can easily have thousands of runs per day. We did not want those runs to overload the OpenChoreo control plane.

## The Core Model: `Workflow` and `WorkflowRun`
OpenChoreo uses two custom resources (CRs):

1. `Workflow` (and `ClusterWorkflow`)  
   The reusable definition. Platform engineers typically author these.

2. `WorkflowRun`  
   A single execution request. Developers (or the platform) create these when they want to run something.

This split is deliberate. It makes the ownership and responsibilities obvious:

1. Platform engineers own the definition and the allowed inputs.
2. Developers own the specific values for one execution.

If you have used Kubernetes, the pattern is similar to other abstractions:

1. A reusable template-like resource.
2. A run/execution resource that references it.

### Why Not a Single Resource?
We could have made one resource that mixes both definition and execution. That is convenient for quick demos, but it does not scale well.

When definition and execution are combined:

1. Copy/paste becomes the default reuse strategy.
2. Platform engineers cannot easily enforce standard behavior.
3. Developers can accidentally change “platform policy” details inside their own runs.
4. UI and governance are harder because there is no stable, reusable contract.

Separating `Workflow` from `WorkflowRun` gives us a clean control surface.

## The Workflow Plane
We also introduced a separate plane for running workflows: the **Workflow Plane**.

This is similar to the way many platforms think about control plane and data plane:

1. The control plane handles configuration, permissions, and intent.
2. The data plane handles heavy execution work.

Automation is heavy work. It can produce a lot of logs, create many short-lived resources, and run CPU-heavy tasks like builds and scans. If we run all of that “inside the control plane cluster,” it can reduce reliability for everything else.

The workflow plane can be:

1. A separate cluster (for stronger isolation and scale).
2. A separate namespace or set of nodes (for smaller setups).
3. Co-located with another plane when requirements are small.

The key point is not where it runs, but that OpenChoreo treats workflow execution as a first-class subsystem with its own capacity planning.

## Architecture (High Level)
At a high level, the flow looks like this:

1. A platform engineer defines a `Workflow` and points it to a workflow plane.
2. A developer creates a `WorkflowRun` and supplies values for the workflow parameters.
3. OpenChoreo controllers validate and render the execution definition.
4. The workflow engine runs the job in the workflow plane.
5. The run status and results are reflected back on the `WorkflowRun`.

`Diagram.png`

In the rest of this post, we will look at the main pieces that make this work.

## Keeping Platform and Developer Concerns Separate
Workflows are shared infrastructure. If everyone can change everything, you do not have a platform. You have a collection of personal scripts.

We found it useful to classify “inputs” into three groups:

### 1. Hard-coded inputs (platform-owned)
These are values that should be consistent and not negotiable.

Examples:

1. Always run a security scan.
2. Always publish images to a company registry.
3. Always use a specific base runner image.
4. Always block pushes when the severity is above a threshold.

These values live in the `Workflow` definition and cannot be changed by the developer for a specific run.

### 2. Developer-provided inputs (developer-owned)
These are values that differ per component or per execution.

Examples:

1. Repository URL
2. Branch / commit
3. Application path within the repo
4. Build arguments
5. Timeouts

These values are supplied in the `WorkflowRun` by the developer (or by the platform UI/CLI on their behalf).

### 3. System-provided inputs (context-owned)
These are values generated by OpenChoreo, or values that come from the platform context.

Examples:

1. Workflow run name (for naming resources)
2. Namespace name
3. Labels like project and component
4. Workflow plane configuration details
5. Resolved references (for example, a `SecretReference`)

These values are useful when the workflow needs to “connect” to platform context without the developer manually typing those details.

This classification helped us keep the model clear. It also helped when we designed the variable system.

## The `Workflow` Resource in Detail
A `Workflow` is the reusable contract. It includes:

1. A parameter schema (OpenAPI v3)
2. A run template (engine-specific)
3. Optional resources to create for the run
4. Optional external references
5. An optional TTL for cleanup

The most important thing is that the `Workflow` defines what inputs exist and how those inputs map into an execution.

### Parameters: OpenAPI Schema as the Contract
We use an OpenAPI v3 schema under `spec.parameters.openAPIV3Schema`.

This matters for three reasons:

1. Validation is consistent.  
   If a field is required, it is required. If a type is an array, it is an array. The platform can reject bad inputs early.

2. UI/CLI can render it.  
   A schema lets the UI present a text field, a select dropdown, a list editor, or a nested object form. This is much easier than making developers edit raw YAML for every run.

3. We can add descriptions and “hints.”  
   A good schema reduces confusion. “repository.url” can have a clear description, examples, and rules.

This is one of the biggest differences between “a pipeline system” and “a product.” A product tells you what it expects. It does not make you guess.

Here is a shortened example (trimmed to focus on the parts that matter):

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Workflow
metadata:
  name: dockerfile-builder
  labels:
    openchoreo.dev/workflow-type: component
  annotations:
    openchoreo.dev/description: Build with a Dockerfile/Containerfile/Podmanfile
spec:
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
  ttlAfterCompletion: 1d
  parameters:
    openAPIV3Schema:
      type: object
      required: [repository]
      properties:
        repository:
          type: object
          required: [url]
          properties:
            url:
              type: string
              description: Git repository URL
              x-openchoreo-component-parameter-repository-url: true
            secretRef:
              type: string
              default: ""
              description: Secret reference for Git credentials
              x-openchoreo-component-parameter-repository-secret-ref: true
        docker:
          type: object
          default: {}
          properties:
            context:
              type: string
              default: "."
            filePath:
              type: string
              default: "./Dockerfile"
```

Even in this small snippet, you can see the story:

1. The workflow declares a contract.
2. The developer can supply `repository.url`, optional `repository.secretRef`, and docker settings.
3. The platform can render those settings in a UI.

### Variable Resolution: CEL Expressions
Once we have a parameter schema, we need a way to use those values in the run template. We also need a way to access system context like run name or namespace.

We use CEL expressions for that. A workflow author can refer to:

1. `${parameters.*}`  
   Developer-provided values from the workflow run.

2. `${metadata.*}`  
   Run context, including the workflow run name, namespace, and labels.

3. `${workflowplane.*}`  
   Information from the workflow plane reference (for example, which secret store to use).

4. `${externalRefs['<id>'].*}`  
   Data from a referenced resource that is resolved at runtime.

Some common examples:

1. `${parameters.repository.url}`
2. `${metadata.workflowRunName}`
3. `${metadata.labels['openchoreo.dev/component']}`

The goal is simple: give workflow authors enough power to map inputs into an execution, without creating a new programming language.

### The Run Template: Engine-Specific Execution
The `runTemplate` is the engine-specific manifest that gets executed in the workflow plane.

Today, OpenChoreo uses Argo Workflows as the default engine, so the run template is an Argo `Workflow`. In other words:

1. The `Workflow` resource is an OpenChoreo abstraction.
2. The `runTemplate` is the concrete Argo workflow definition that Argo runs.

This design has a practical benefit. Many teams already understand Argo. It also means we can reuse mature engine capabilities (step orchestration, retries, artifacts, logs).

The workflow author maps:

1. Developer inputs (parameters)
2. Platform constants (hard-coded values)
3. System context (metadata and plane config)

Into fields that the engine understands.

### Resources: Supporting Kubernetes Objects
Workflows often need supporting resources.

For example, if a developer builds from a private Git repository, the workflow may need a secret in the workflow plane with Git credentials. But the secret might differ from run to run. One team uses a GitHub deploy key, another uses a token, another uses CodeCommit and IAM.

The `resources` section exists for these cases. A workflow can say:

1. “If a secret reference is provided, create an `ExternalSecret` in the workflow plane.”
2. “Name it using the workflow run name so it does not collide.”
3. “Delete it when the workflow run is deleted.”

This makes workflows self-contained and safer. It also means we do not have to pre-provision a large number of long-lived secrets for every possible team.

### ExternalRefs: Reading Other OpenChoreo CRs
Sometimes a workflow run needs information that lives in another CR.

A common example is `SecretReference`. The developer might pass a `secretRef` name, and the workflow needs to look up that resource and use its content to create an `ExternalSecret` object.

That is what `externalRefs` are for:

1. The workflow declares which external resources it will read.
2. The controller resolves them at runtime.
3. The resolved data becomes available in CEL expressions.

This is a clean alternative to ad-hoc API calls from inside workflow steps.

## The `WorkflowRun` Resource in Detail
A `WorkflowRun` is an execution request. It references a workflow and provides values for its parameters.

Here is a shortened example:

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: WorkflowRun
metadata:
  name: greeting-service-build-01
  labels:
    openchoreo.dev/project: default
    openchoreo.dev/component: greeting-service
spec:
  workflow:
    kind: Workflow
    name: dockerfile-builder
    parameters:
      repository:
        url: https://github.com/openchoreo/sample-workloads
        revision:
          branch: main
        appPath: /service-go-greeter
      docker:
        context: /service-go-greeter
        filePath: /service-go-greeter/Dockerfile
```

When the `WorkflowRun` is created:

1. The controller validates the parameters against the workflow’s schema.
2. It renders the run template into a concrete engine manifest.
3. It submits that manifest to the workflow plane.

The `WorkflowRun.status` tracks what happened. For Argo, that includes step-level information.

From a developer point of view, the workflow run should answer a few basic questions:

1. Did it start?
2. What step is it on?
3. Did it succeed or fail?
4. If it failed, where and why?

Those sound like obvious questions. In practice, many workflow systems fail here. We paid attention to status because it is what makes automation usable.

## Reuse Strategy: Build from Small Steps
Reusability is not only about “one workflow for many runs.” It is also about making workflows easy to compose.

With Argo, the natural unit of reuse is `ClusterWorkflowTemplate`. We recommend writing one template per task:

1. Checkout source
2. Build image
3. Scan image
4. Push image
5. Generate SBOM

Then the workflow run template can stitch these together.

Here is a small example of composing templates:

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
                value: "{{steps.checkout-source.outputs.parameters.git-revision}}"
```

This looks simple, but it has real operational benefits:

1. Task templates can be tested and improved independently.
2. Security fixes in a shared template automatically apply to all workflows that use it.
3. The platform team can review and approve task templates as building blocks.

Over time, this creates a small “workflow library” that teams can depend on.

## CI Workflows vs Generic Automation Workflows
The same abstraction supports both styles:

1. **CI workflows** are associated with a component, and are usually triggered by code changes or manual actions like “Build.”
2. **Generic workflows** are not tied to a component. They can still be run by developers, but the context is different (infra tasks, maintenance, one-off jobs).

We do not introduce a separate resource for “CI workflow.” Instead, we use metadata and conventions so the UI and CLI can categorize workflows.

This matters because it keeps the system small. Developers learn one model. Platform engineers define one kind of reusable workflow. The UI can still show “CI workflows” in a dedicated view when that makes sense.

## UI and CLI: Making Workflows Feel Like a Product
The goal of the UI and CLI is not to hide YAML. It is to help users avoid editing YAML for common things.

Two design choices matter here:

1. Use OpenAPI schema for parameters.
2. Use vendor extensions to detect common fields.

### The CI Workflow Label
In the UI and CLI, workflows intended to be used by components are identified using a label:

```yaml
metadata:
  labels:
    openchoreo.dev/workflow-type: component
```

This lets the UI do simple things:

1. Show “CI Workflows” as a curated list for a component.
2. Hide unrelated generic workflows from that view.
3. Apply special UX where it is helpful (for example, showing repo fields in a standard way).

### Vendor Extensions for Better UX
Schemas are great, but they do not tell the UI which field is “the repository URL” or “the secret reference.”

So we use a small set of vendor extensions to mark common fields. These are not required for correctness, but they improve the experience.

The current extensions are:

1. `x-openchoreo-component-parameter-repository-url`
2. `x-openchoreo-component-parameter-repository-branch`
3. `x-openchoreo-component-parameter-repository-commit`
4. `x-openchoreo-component-parameter-repository-app-path`
5. `x-openchoreo-component-parameter-repository-secret-ref`

When the UI sees these, it can do things like:

1. Offer a branch picker instead of a plain text input.
2. Offer a secret selector that lists allowed secret references.
3. Validate formats early and show clearer errors.

This is the kind of detail that reduces developer frustration. A platform is successful when it feels boring to use, in a good way.

## Using Labels and Context in CEL
Workflows often need names and identifiers to build resource names.

For example, to build an image name you might want:

1. The component name
2. The project name
3. The workflow run name (for uniqueness)

When a run is created for a component, OpenChoreo applies labels like:

1. `openchoreo.dev/project`
2. `openchoreo.dev/component`

A workflow author can read them using CEL:

1. `${metadata.labels['openchoreo.dev/project']}`
2. `${metadata.labels['openchoreo.dev/component']}`

This is a small feature, but it is important. It avoids asking the developer to provide values that the system already knows.

## Cleanup and Garbage Collection
Automation creates temporary things. If we do not clean them up, clusters become junkyards.

OpenChoreo supports cleanup in two main ways:

### 1. TTL after completion
Workflows can set `ttlAfterCompletion`, which deletes completed runs after a duration. This applies to success and failure.

The reason to include failure is simple: failures can be noisy and frequent. Keeping every failed run forever is not useful. Most teams care about a window of history, not infinite history.

### 2. Component lifecycle cleanup
Workflow runs created for a component are treated as part of that component’s lifecycle.

When a component is deleted, its workflow runs are deleted as well. This avoids the common problem of “dead components” leaving behind thousands of runs.

### Resource cleanup
If a workflow created supporting resources (like secrets) as part of a run, those resources should be deleted when the run is deleted.

This matters for security. Temporary credentials should not live longer than they are needed.

## Workflow Authoring: A Practical Guide
If you are a platform engineer writing workflows, here is the path we recommend.

### Step 1: Write reusable engine steps
For Argo, that means `ClusterWorkflowTemplate`.

Keep each step focused. Do not create one template that does “checkout + build + scan + push + notify.” That is not reusable.

Instead, write:

1. A checkout template
2. A build template
3. A scan template
4. A push template

Each template should expose a small set of parameters and produce useful outputs.

### Step 2: Define the OpenChoreo `Workflow`
Once you have templates, define a workflow that:

1. Declares the parameter schema for developers.
2. Maps those parameters into the run template.
3. Adds platform defaults and policy.

This is also where you decide what developers are allowed to control.

For example:

1. A developer can change `repository.url`.
2. A developer can change `repository.revision`.
3. A developer cannot disable scanning if your platform requires scanning.

### Step 3: Restrict workflows per component type (for CI workflows)
If a workflow is intended for CI builds, platform engineers can restrict which workflows a component type can use.

Example:

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

This helps governance. It also keeps the UI simpler. Developers see the workflows that make sense for their component type.

### Step 4: Developers create `WorkflowRun`s
Finally, developers (or the platform UI/CLI) create workflow runs.

In a polished experience, developers should not need to know the full workflow run YAML. They should fill in a form or run a CLI command, and the platform should create the run resource for them.

## What About Pluggable Workflow Engines?
Today, the workflow engine is not pluggable without changing the controller. Argo is the default.

We want to move to a modular design where different engines can be installed and selected (for example, Tekton, GitHub Actions, Azure Pipelines, or even non-Kubernetes backends).

Why is this hard?

Because workflow engines differ in:

1. Execution model (steps, DAGs, jobs)
2. Artifact handling
3. Secret injection
4. Status model and events

To support multiple engines, the platform needs a stable abstraction for:

1. Submitting a run
2. Tracking status
3. Getting logs and outputs
4. Handling retries and timeouts
5. Cleaning up resources

We have a direction for this, but it is still ongoing work.

## Lessons Learned (So Far)
It is tempting to focus on the YAML. The real work is not YAML. It is the system around it.

Here are a few things we learned while iterating on workflows:

### 1. A schema is not optional
If you do not define a schema, the UI cannot help. Validation becomes uneven, and teams end up with “tribal knowledge” about what each workflow expects.

OpenAPI schemas also make workflows easier to review. A platform engineer can read the schema and understand what a developer can control.

### 2. Reuse needs guardrails
If anyone can write and run any workflow, reuse does not happen. Teams will build their own versions.

Reuse improves when:

1. Common steps are shared and maintained.
2. Component types limit workflow choices to what is supported.
3. The platform team invests in a small library of good workflows.

### 3. Isolation matters
Builds and automation runs can spike quickly.

Separating execution into a workflow plane gives you:

1. Better control over capacity
2. Better isolation for noisy workflows
3. Clearer operational ownership

### 4. Cleanup is a feature, not a nice-to-have
Teams usually care about automation reliability. They rarely think about cleanup until it becomes a problem.

TTL and lifecycle cleanup prevent a class of “slow failures” where the platform looks fine, but the cluster is full of old runs and old secrets.

## Conclusion
OpenChoreo Workflows are built around a simple idea:

1. Define a reusable `Workflow` once.
2. Execute it many times via `WorkflowRun`.

The rest is about making that idea work in a real platform:

1. Clear ownership between platform engineers and developers.
2. A strong contract using OpenAPI schemas.
3. A variable system (CEL) that can use inputs and platform context.
4. Reusable building blocks in the workflow engine.
5. UI/CLI features that make common actions easy.
6. A workflow plane so execution scales without hurting the control plane.
7. Cleanup and lifecycle management so automation does not leave a mess.

In follow-up posts, we can go deeper into workflow plane topology, how we structure shared templates, and how we plan to make the workflow engine modular.

