# Workflows in OpenChoreo: One Building Block for CI, Provisioning, and Everything in Between

## Introduction

When someone tries out an Internal Developer Platform (IDP) for the first time, the questions tend to repeat themselves. How do I build my source code and ship it to a cluster? What does the day-to-day experience look like for my developers? Does it work with private repositories? Can I write a workflow that provisions a database, or runs a one-off data migration, or rotates a secret? In other words, people want to know whether the platform handles the full set of automation tasks they already run today, not just a narrow slice of CI.

OpenChoreo answers all of these with one building block: a unified Workflow design. The same Workflow and WorkflowRun resources power component CI builds, infrastructure provisioning, data pipelines, scheduled cleanup tasks, and any other automation a team needs. There is no separate “build system” and “provisioning system” to learn. There is one shape, one mental model, and one set of guarantees, used for many jobs.

This post walks through how we designed those Workflows. The design has a long history. We went through several iterations before we settled on the current shape, and most of those iterations were about making the model more reusable, more general, and more honest about who is doing what. By the end of this post you should understand why there are two custom resources instead of one, how parameters flow from the Platform Engineer down to the developer, why we chose to put the workflow engine in its own plane, and how all of this fits with the rest of OpenChoreo.

## What we wanted to optimize for

Before we wrote any YAML or controller code, we wrote down a short list of things the design had to get right. These goals shaped almost every decision that came afterwards.

**Reusability.** A workflow is not a single-use script. The same docker build workflow should serve dozens or hundreds of components, with different repositories, different build arguments, and different environments. Writing a new workflow per component would defeat the purpose of having a platform at all. We wanted authoring a workflow to feel like authoring a library function, not a one-off shell script.

**A clear split between Platform Engineer and developer concerns.** An IDP only earns its name if it serves both audiences well. The Platform Engineer (or DevOps or SRE, depending on your org) needs to lock down the parts that must stay consistent — security scanning, branch policy, registry credentials, runtime images. The developer needs to express the parts that change per component — repository URL, build context, environment variables. The design has to make both sides feel ergonomic, and it has to make the boundary between them obvious.

**A single shape for very different jobs.** A docker build looks nothing like a Terraform plan, which looks nothing like a database migration, which looks nothing like a nightly data export. We did not want a special CRD for each one. We wanted one shape that could host all of them, so that learning one workflow teaches you all of them.

These three goals — reusability, separation of concerns, and a generalized shape — are what led us to two CRs (a Workflow and a WorkflowRun), to a dedicated Workflow Plane, and to a parameter system with very specific roles.

## Two CRs: abstraction and execution

OpenChoreo splits the workflow concept into two custom resources.

The **Workflow** is the abstraction. It is the template that says “this is what a docker build looks like in our organization,” or “this is how we provision a database for a Project.” It is authored by Platform Engineers, DevOps, or SREs. It declares which parameters developers may set, which values are hardcoded, which steps run, and which side resources (like secrets) are needed. It is the contract.

The **WorkflowRun** is the execution. It is what a developer creates when they actually want something to happen. It points at a Workflow by name and supplies the developer-facing parameters. The OpenChoreo controller picks it up, renders the run template, applies it to the Workflow Plane, and tracks the result.

This split is deliberate, and it mirrors the Netflix approach of separating an abstract “Objective” from the concrete model selection: clients only know the high-level intent, while the platform handles the messy details behind the scenes. In OpenChoreo, developers only know the high-level intent (“build my service”, “provision my database”), while Platform Engineers control the messy details (which engine, which steps, which images, which credentials).

Why use the word **Workflow** rather than **Pipeline**? Both are common in the community. We already use the word **Pipeline** for the DeploymentPipeline resource, which describes how releases are promoted between environments. Reusing it for builds and automation would have collided with that meaning, so we went with Workflow.

Both CRs come in **namespace-scoped** and **cluster-scoped** variants. ClusterWorkflow lives at the cluster level and can be reused by any namespace. Workflow lives in a namespace and is scoped to that namespace’s components. The same pattern shows up across OpenChoreo (ClusterComponentType, ClusterTrait, and so on), and the rule is the same: cluster-scoped resources can only reference other cluster-scoped resources.

## Why a separate Workflow Plane

OpenChoreo already has three planes: a control plane (where the CRDs and controllers live), one or more data planes (where workloads actually run), and an optional observability plane (where logs, metrics, and traces land). We added a fourth plane for workflows.

The motivation is simple. The workflow engine — Argo Workflows in our default install — has its own performance, isolation, and resource needs. A long docker build can chew through CPU and memory. A flood of WorkflowRuns shouldn’t affect the responsiveness of the control plane or the latency of running services in the data plane. Giving workflows their own home keeps these concerns from leaking into each other.

Putting the engine in its own plane also makes it easy to scale workflow capacity independently, to apply different node pool policies, and to swap in different engines later without disrupting anything else.

That said, you don’t have to deploy the Workflow Plane on its own cluster. If your throughput is modest and you want a simpler footprint, you can install the Workflow Plane components inside the control plane or the data plane. The plane is a logical boundary first; whether it gets its own physical cluster is a deployment decision you make based on your performance and isolation needs.

## Architecture

At a high level, the lifecycle looks like this. A developer creates a WorkflowRun in the control plane. The OpenChoreo controller picks it up, looks up the referenced Workflow, builds a context (parameters, metadata, external references), evaluates the CEL expressions in the run template, and applies the resulting Argo Workflow CR (and any side resources) to the Workflow Plane. Argo runs the steps. The controller watches the result and reflects status back into the WorkflowRun.

> _Diagram: control plane (CRs and controller) → workflow plane (engine, runs, side resources) → optional secret store, optional data plane targets._

The result is that developers and Platform Engineers only ever look at OpenChoreo CRs. The Argo CR sits underneath and does the heavy lifting, but you don’t have to write it from scratch every time, and you don’t have to teach every developer how to read it.

## Where parameters come from

A workflow that runs the same way every time is not very useful. A workflow that lets developers change anything they want is dangerous. The interesting design space is in between, and it’s where the Platform Engineer earns their title.

OpenChoreo Workflows recognize three kinds of parameters, and the design forces you to be explicit about which is which.

**Hardcoded parameters** are set by the Platform Engineer in the Workflow definition itself. They never change at run time. Examples: forcing `trivy-scan: "true"` so every build is scanned, pinning `branch: main` so all production builds come from a single branch, baking in a registry hostname, or fixing a runtime image tag for compliance. Hardcoding is the right choice when the value is non-negotiable.

**Developer-provided parameters** are declared in the Workflow’s `parameters` schema. They are the inputs the developer fills in when they create a WorkflowRun. Examples: the repository URL, the application path inside that repository, the build context, the list of build arguments, the timeout. The Platform Engineer’s job here is to choose what is safe and useful to expose.

**System-generated parameters** come from OpenChoreo itself. The Platform Engineer references them in the run template using CEL expressions, and the controller fills them in at run time. Examples: the WorkflowRun’s name (`metadata.workflowRunName`), the namespace it lives in (`metadata.namespaceName`), and labels that OpenChoreo attaches automatically (`metadata.labels['openchoreo.dev/component']`, `metadata.labels['openchoreo.dev/project']`). System-generated parameters are how the workflow plugs into the rest of the platform — naming conventions, ownership tracking, image tagging — without anyone having to type those values by hand.

Naming these three kinds of parameters explicitly helped us a lot. It gave us a vocabulary for design reviews, and it gave Platform Engineers a checklist when they author a new workflow: for each value the run needs, is it hardcoded, developer-provided, or system-generated?

## How reusability is achieved

Once you have one shape and three parameter types, reusability comes from two places.

The first is the Workflow itself: a single Workflow CR like `dockerfile-builder` can be referenced by many WorkflowRuns, each one supplying different developer-provided parameters. The Workflow is written once and used everywhere.

The second is the engine layer underneath. Inside the run template, OpenChoreo doesn’t try to reinvent task orchestration. It uses Argo’s ClusterWorkflowTemplate as the unit of reuse. We recommend writing each individual task — checking out source, building an image, pushing it to a registry, scanning it — as its own ClusterWorkflowTemplate. Then your OpenChoreo Workflows compose those small templates into bigger flows. The same `containerfile-build` template can be used by `dockerfile-builder`, `paketo-buildpacks-builder`, and any future image builder you create. Reuse compounds.

This is the same instinct you see in well-factored codebases: small, well-named building blocks beat one giant function. Argo gives you the small blocks; OpenChoreo gives you a place to compose them and a clear contract with the developer.

## The Workflow CR, field by field

Here is a Workflow you can read end-to-end. It is the standard `dockerfile-builder` shipped with OpenChoreo.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Workflow
metadata:
  name: dockerfile-builder
  labels:
    openchoreo.dev/workflow-type: "component"
  annotations:
    openchoreo.dev/description: "Build with a provided Dockerfile/Containerfile/Podmanfile"
spec:
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
  ttlAfterCompletion: "1d"
  parameters:
    openAPIV3Schema:
      type: object
      required:
        - repository
      properties:
        repository:
          type: object
          description: "Git repository configuration"
          required:
            - url
          properties:
            url:
              type: string
              description: "Git repository URL. Supports https://, ssh:// and AWS CodeCommit (codecommit://repo or codecommit::region://repo) formats."
              x-openchoreo-component-parameter-repository-url: true
            secretRef:
              type: string
              default: ""
              description: "Secret reference name for Git credentials."
              x-openchoreo-component-parameter-repository-secret-ref: true
            # ...
        docker:
          type: object
          default: {}
          description: "Docker build configuration"
          properties:
            context:
              type: string
              default: "."
              description: "Docker build context path relative to the repository root"
            filePath:
              type: string
              default: "./Dockerfile"
              description: "Path to the Dockerfile relative to the repository root"
        buildEnv:
          type: array
          default: []
          description: "Environment variables passed as --env to podman build"
          items:
            type: object
            required: [name, value]
            properties:
              name: { type: string }
              value: { type: string }
        buildArgs:
          type: array
          default: []
          description: "Docker build arguments declared with ARG in the Dockerfile (passed as --build-arg)"
          items:
            type: object
            required: [name, value]
            properties:
              name: { type: string }
              value: { type: string }
  runTemplate:
    apiVersion: argoproj.io/v1alpha1
    kind: Workflow
    metadata:
      name: ${metadata.workflowRunName}
      namespace: ${metadata.namespace}
    spec:
      arguments:
        parameters:
          - name: component-name
            value: ${metadata.labels['openchoreo.dev/component']}
          - name: project-name
            value: ${metadata.labels['openchoreo.dev/project']}
          - name: workflowrun-name
            value: ${metadata.workflowRunName}
          - name: git-repo
            value: ${parameters.repository.url}
          - name: branch
            value: main
          - name: commit
            value: ${parameters.repository.revision.commit}
          # ...
      serviceAccountName: workflow-sa
      entrypoint: build-workflow
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
  externalRefs:
    - id: git-secret-reference
      apiVersion: openchoreo.dev/v1alpha1
      kind: SecretReference
      name: ${parameters.repository.secretRef}
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
          refreshInterval: 15s
          secretStoreRef:
            kind: ClusterSecretStore
            name: ${workflowplane.secretStore}
          target:
            name: ${metadata.workflowRunName}-git-secret
            creationPolicy: Owner
            template:
              type: ${externalRefs['git-secret-reference'].spec.template.type}
          data: |
            ${externalRefs['git-secret-reference'].spec.data.map(secret, {
              "secretKey": secret.secretKey,
              "remoteRef": {
                "key": secret.remoteRef.key,
                "property": has(secret.remoteRef.property) && secret.remoteRef.property != "" ? secret.remoteRef.property : oc_omit()
              }
            })}
```

A Workflow has five parts that matter:

- **Parameters** — the developer-facing schema. What the developer is allowed to set on a WorkflowRun.
- **RunTemplate** — the inline manifest, in the workflow engine’s own format (Argo Workflow in our default install), with CEL expressions where values come from elsewhere.
- **Resources** — extra Kubernetes objects that the workflow needs in order to run (most often secrets and config maps).
- **ExternalRefs** — references to other custom resources whose contents you want to read at run time and use inside the run template or resources section.
- **TTLAfterCompletion** — an optional duration after which finished runs are deleted automatically.

Let’s look at each of those more carefully.

### Parameters

The `parameters` block uses OpenAPI v3 Schema. That is intentional: every UI we care about — Backstage, the OpenChoreo CLI, an internal portal — already speaks OpenAPI v3 and can render forms from it. If you say a field is a string with an enum of three values, the UI shows a dropdown. If you say it is an integer with a minimum and maximum, the UI shows a number input with validation. Lists, nested objects, defaults, descriptions — all of it carries through.

This is how Platform Engineers tell developers what they can configure. The `dockerfile-builder` example asks for the Git repository URL, an optional secret reference for private repositories, the docker build context and Dockerfile path, build environment variables, and build arguments.

### CEL expressions

The run template, the resources, and the external refs all support CEL expressions written as `${...}`. CEL is what wires the abstract Workflow definition to the concrete WorkflowRun.

The most common ones you’ll use:

- `${parameters.<path>}` reads developer-provided parameters. `${parameters.repository.url}` is the URL the developer typed.
- `${metadata.<field>}` reads run-level metadata. `${metadata.workflowRunName}` is the name of the WorkflowRun, `${metadata.namespace}` is the control plane namespace, and `${metadata.labels['openchoreo.dev/component']}` is the component label OpenChoreo attached.
- `${workflowplane.secretStore}` resolves to the ClusterSecretStore name attached to the Workflow Plane. You use this inside the resources section to point ExternalSecrets at the right key vault.
- `${externalRefs['<id>'].spec.<...>}` reads fields from a referenced external custom resource. Used together with the `externalRefs` block.

CEL also supports helpers like `has(...)`, `.map(...)`, ternary expressions, and a few OpenChoreo-specific functions like `oc_omit()` for conditionally dropping a field. Anywhere a value would otherwise be a fixed string in the run template, you can use a CEL expression instead.

### RunTemplate

The run template is what actually executes. In the default OpenChoreo install, this is an Argo Workflow CR, because we ship Argo as the default workflow engine. The OpenChoreo controller renders the CEL expressions, then applies the resulting Argo Workflow to the Workflow Plane. From that point on, Argo runs the steps in the cluster.

This is also where the Platform Engineer maps parameters into the engine’s shape: which developer parameters become Argo arguments, which values are hardcoded, which references pick up labels and metadata. The mapping is explicit, lives in one place, and is easy to review.

### Resources

A workflow often needs more than just a single Argo Workflow object to run successfully. A private build needs Git credentials, which means a Kubernetes secret in the Workflow Plane. A workflow that talks to a cloud API might need a config map. Anything you’d normally `kubectl apply` next to the workflow goes in the `resources` block, and OpenChoreo applies it for you when the run starts and cleans it up when the run is deleted.

The Git secret in the example is a good illustration. The Platform Engineer declares an ExternalSecret that pulls Git credentials from the secret store attached to the Workflow Plane. The `includeWhen` expression makes sure the secret is only created if the developer actually provided a `secretRef` (no point creating an empty secret for public repositories). The secret’s lifecycle is tied to the run, so credentials don’t pile up over time.

If a secret would be the same for every developer, you don’t need to put it here at all — the Platform Engineer can set it up once in the Workflow Plane and forget about it. The `resources` block is for things that genuinely change per run.

### ExternalRefs

`externalRefs` lets you read another custom resource’s contents at run time and use those contents inside CEL expressions. Today this is scoped to OpenChoreo’s `SecretReference` resource, which holds a pointer to a secret stored in a key vault.

The pattern in the example is: the developer gives a `secretRef` name; OpenChoreo looks up the matching `SecretReference` CR; the controller injects its `spec.data` and `spec.template.type` into the CEL context; the resources block uses those values to construct an ExternalSecret. The developer never has to know how the key vault is wired up. They just type a name.

We expect this list to grow as we find more cases where a workflow needs to read from another resource at run time.

### TTLAfterCompletion

By default, completed WorkflowRuns stay around forever. That’s convenient when you’re debugging, and inconvenient when a thousand of them accumulate over a quarter. `ttlAfterCompletion` (in the example, `1d`) tells OpenChoreo to delete a run a fixed time after it finishes — successful or failed. Set it to whatever your team’s retention policy says. Leave it off if you have an external system tracking runs and you want everything kept.

## The WorkflowRun CR

The WorkflowRun is the developer’s side of the contract. It is short, because the Workflow has done the heavy lifting.

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
    kind: Workflow
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

The developer says which Workflow to run and supplies the parameters that the Workflow’s schema asked for. That’s it. When this lands in the cluster, the OpenChoreo controller picks it up, builds the CEL context, renders the run template, applies it to the Workflow Plane, and creates any `resources` declared in the Workflow.

The WorkflowRun’s `status` field tracks what is happening on the engine side: which steps started, which finished, which failed, and any output parameters Argo recorded along the way. You can watch it with `kubectl get workflowrun -w`, look at it in the OpenChoreo UI, or query it via the API. The same status surface works for a docker build, a database provisioning run, or a scheduled cleanup task — because it’s the same CR.

## Building a workflow from scratch

Here is the recipe Platform Engineers follow when they want to add a new workflow.

**1. Author the engine-level building blocks.** For Argo, this means writing one ClusterWorkflowTemplate per logical task: a `checkout-source` template, a `containerfile-build` template, a `push-image` template, and so on. Expose only the values that should be configurable as inputs. The smaller and more focused these templates are, the more often you’ll be able to reuse them across different OpenChoreo Workflows.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ClusterWorkflowTemplate
metadata:
  name: containerfile-build
spec:
  templates:
    - name: build-image
      inputs:
        parameters:
          - name: git-revision
          - name: build-env
          - name: build-args
      container:
        image: ghcr.io/openchoreo/podman-runner:v1.1
        command: [sh, -c]
        args:
          - |-
            # build the image using the inputs above
            ...
        securityContext:
          privileged: true
        volumeMounts:
          - mountPath: /mnt/vol
            name: workspace
```

**2. Define the OpenChoreo Workflow.** Pick a target Workflow Plane and start with the metadata block. If you’re writing a CI workflow, attach the `openchoreo.dev/workflow-type: "component"` label so the UI and CLI categorize it correctly.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterWorkflow
metadata:
  name: dockerfile-builder
  labels:
    openchoreo.dev/workflow-type: "component"
  annotations:
    openchoreo.dev/description: "Build with a provided Dockerfile/Containerfile/Podmanfile"
spec:
  workflowPlaneRef:
    kind: ClusterWorkflowPlane
    name: default
```

**3. Decide which values are developer-provided and which are hardcoded.** Add the developer-facing values to the `parameters` schema. Hardcode everything else inside the `runTemplate`. Use system-generated values like `${metadata.workflowRunName}` wherever the engine needs naming or ownership wired up.

In the example below, `branch` is hardcoded to `main`, while `git-repo`, `commit`, `build-env`, and `build-args` come from the developer.

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
        - name: component-name
          value: ${metadata.labels['openchoreo.dev/component']}
        - name: workflowrun-name
          value: ${metadata.workflowRunName}
        - name: git-repo
          value: ${parameters.repository.url}
        - name: branch
          value: main
        - name: commit
          value: ${parameters.repository.revision.commit}
        - name: build-env
          value: ${parameters.buildEnv}
        - name: build-args
          value: ${parameters.buildArgs}
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

**4. If this is a CI workflow, attach it to the relevant ComponentTypes.** Platform Engineers control which workflows a particular ComponentType can use. A WebApp ComponentType might allow a dockerfile builder and a buildpacks builder, but nothing else. Restricting the list keeps the developer experience focused.

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

**5. Hand off to developers.** With the Workflow in place and the ComponentType permissions set, developers create WorkflowRuns directly or — for CI — through the higher-level Component flows in the UI and CLI.

## CI workflows vs generic workflows

Inside OpenChoreo, every workflow has the same shape. There is no separate “CI Workflow” CRD. There is just a label.

A workflow that is meant to be attached to a Component for CI carries:

```yaml
metadata:
  labels:
    openchoreo.dev/workflow-type: "component"
```

This single label is what the UI, the CLI, and the validation layer use to decide whether a workflow shows up in a Component’s build dropdown. Workflows without the label are still perfectly runnable — you can write a database provisioning workflow, a maintenance workflow, or anything else, and trigger it via a WorkflowRun directly. Even a workflow that does carry the label can still be run on its own, as long as the developer (or another system) supplies the right parameters.

This is one of the design choices we’re happiest with. We did not want to fork the model into “CI Workflow” and “Generic Workflow”. Instead we kept the type system uniform and used a label to capture the additional UX rules.

## Vendor extensions for a smoother UI

Some fields in a parameter schema are special enough that the UI and CLI want to render them with extra context. The classic example is the secret reference for a private Git repository: instead of asking the developer to type a name, the UI should show a dropdown of available secret references. Another example is the repository URL field, which the UI may want to validate or annotate.

OpenChoreo carries these hints through OpenAPI vendor extensions — fields prefixed with `x-`. Today five extensions are recognized:

- `x-openchoreo-component-parameter-repository-url` — the Git repository URL field.
- `x-openchoreo-component-parameter-repository-branch` — the Git branch field.
- `x-openchoreo-component-parameter-repository-commit` — the Git commit SHA field.
- `x-openchoreo-component-parameter-repository-app-path` — the application path inside the repository.
- `x-openchoreo-component-parameter-repository-secret-ref` — the secret reference field.

Set one of these to `true` in the schema and the UI and CLI will treat the field accordingly. If you want to add a new extension, you add it here and update the UI and CLI to honor it. We deliberately keep this list short. Vendor extensions are a UX optimization, not a place for arbitrary schema annotations.

## How OpenChoreo finds your component

When a CI WorkflowRun is created via a Component, OpenChoreo automatically attaches two labels to it:

```
openchoreo.dev/component: <component-name>
openchoreo.dev/project:   <project-name>
```

You can read those labels from inside the run template using `${metadata.labels['openchoreo.dev/component']}` and `${metadata.labels['openchoreo.dev/project']}`. Most CI workflows use these labels to derive the image name, to record provenance for the build, or to write back results that the Component controller can read later. Because the labels are attached automatically, the Platform Engineer doesn’t have to ask the developer to type the component name twice.

## Garbage collection

Without a clean-up plan, you will eventually have tens of thousands of completed WorkflowRuns sitting in the cluster, none of which anyone is going to read.

OpenChoreo gives you three layers of garbage collection.

First, the Workflow’s `ttlAfterCompletion` field. When set, completed runs are deleted automatically after the chosen duration. This is the easiest knob, and it’s usually enough.

Second, ownership-based cleanup. WorkflowRuns created for a Component are owned by that Component. When the Component is deleted, its WorkflowRuns are deleted with it. You don’t have to track down history-of-builds manually.

Third, resource-level cleanup. Anything declared in the Workflow’s `resources` block is owned by the WorkflowRun. When the WorkflowRun is deleted (whether by TTL, by Component deletion, or by hand), those resources go with it. The ExternalSecret created for a private build, the temporary config map, the credentials object — all cleaned up.

One thing that’s deliberately *not* automatic: deleting a Workflow does not delete the WorkflowRuns that reference it. This matches the rest of Kubernetes — deleting a CronJob doesn’t delete its Jobs, and deleting an Argo CronWorkflow doesn’t delete its Workflows. The history outlives the schedule. If you really want to remove the runs as well, do it explicitly.

## What’s next: pluggable engines

In the current design, the workflow engine is Argo Workflows, and it is not pluggable without changing the controller. We deliberately started this way. Picking one engine let us focus on the abstraction and ship something useful, instead of getting stuck designing for hypothetical engines we didn’t need.

Now that the abstraction has settled, we are working on turning the engine into a module. The plan is that you’ll be able to install a different engine — Tekton Pipelines, GitHub Actions, Azure Pipelines, or something else — without changing the Workflow and WorkflowRun shapes. The Workflow’s `runTemplate` would carry the engine-specific manifest (Tekton’s PipelineRun, a GitHub Actions workflow definition, etc.), and the controller would route to the right backend based on the WorkflowPlane configuration.

When that lands, switching engines (or running multiple engines side by side) becomes a deployment decision, not a rewrite of every workflow.

## Conclusion

There is a lot of ground covered in one article. The short version: OpenChoreo Workflows are one building block for any automation that runs on Kubernetes. The Workflow CR is the abstraction Platform Engineers author. The WorkflowRun CR is the execution developers create. The two are connected through a parameter system that names hardcoded, developer-provided, and system-generated values explicitly, and through a CEL templating layer that wires it all together. The engine sits in its own plane so it can scale and fail without taking the rest of the platform with it.

We will write more posts as we learn more. The big ones we expect to publish next are about the modular workflow engine, debugging WorkflowRuns in production, and patterns for non-CI workflows like infrastructure provisioning. If something in this design surprises you, or if you want a workflow shape we don’t support, let us know — that feedback is how this design got to where it is, and it’s how it will keep evolving.
