# Workflows in 


## Introduction
When someone tries out an internal developer platform (IDP), one of their main concerns is how I can build my source code and deploy it? how is the experience going to be? does it support private repositories? Can I write a workflow for database provisioning? OpenChoreo provides a unified Workflow design for running automation tasks. Whether you need component CI builds, infrastructure provisioning, data pipelines, or any other automation, it all uses the same Workflow and WorkflowRun resources.

Want to know the basics before diving into this? Read my article about OpenChoreo basics where I discuss what, why, how openchoreo solves the developer platform problem and how it is working, the relationship between resources and so on. https://medium.com/p/6af16b309d98

In this blog, I will be discussing about how we designed the OpenChoreo Workflows. This has a long history as we went through multiple iterations to make this more optimized, generalized, and reusable.

When we designed workflows in openchoreo, we wanted to maximize,
1. Reusability
2. Platform Enginee/Devops vs Developer concerns
3. Generalized approach for CI workflows and generic workflows such as infrastructure provisioning

So, we came up with 2 CRs workflow and workflow run. In the community, workflow and pipeline are the most commonly used terms and we went with workflows since we already had pipeline word in the deployment pipeline resource. 

We came up with a seperate plane for workflows called Workflow Plane just like control plane, data plane and observability plane. Workflow plane can be installed inside control plane or data plane as well if you dont need a seperate plane considering your performance requirements.

## archtecture

Diagram.png

## How did we manage different concerns

### Developer/PE Parameters
How a platform/devops engineer can use workflows to define the workflow and its parameters? Here, i will break down how you can define different parameters using OpenChoreo Workflows considering different concerns.

First identify the following parameter types:
1. Hard-coded Parameters
Who provides: Platform Engineers
Example: trivy-scan: "true", branch: "main"
Values defined and locked in the Workflow definition. Used for consistent, non-negotiable settings.

2. Developer-provided Parameters
Who provides: Developers
Examples: repo-url, timeout, application path
Values provided when creating or triggering workflows. Flexible inputs configured per workflow run.

3. System-generated Parameters
Who provides: OpenChoreo and defined by platform engineers to use
Examples: workflowRunName, namespaceName, labels

### Reusability
Reusability is increased through the Workflow Abstraction as the same workflow can be used with different parameters for different purposes. To increase the re-usability, we have used the argo workflow templates as different steps. 

## Abstraction
Workflow is the abstraction and workflow run is the execution. Workflow is written by Platform/DevOps/Site Reliability Engineer and Workflow Run will be created by the developer giving different personas capabilities that an IDP is built for. Workflow can be either namespace level or cluster level (ClusterWorkflow).

I will first go through the CRs and its fields.

Workflow CR
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
              description: "Secret reference name for Git credentials. Expected keys: ssh-privatekey (SSH), username/password (Basic), or aws-access-key-id/aws-secret-access-key (AWS CodeCommit via git-remote-codecommit). Optionally include aws-role-arn to assume an IAM role for CodeCommit access."
              x-openchoreo-component-parameter-repository-secret-ref: true
            ...
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
          description: "Environment variables available during the build (passed as --env to podman build). Reference: https://docs.podman.io/en/stable/markdown/podman-build.1.html#env-env-value"
          items:
            type: object
            required: [name, value]
            properties:
              name:
                type: string
                description: "Environment variable name"
              value:
                type: string
                description: "Environment variable value"
        buildArgs:
          type: array
          default: []
          description: "Docker build arguments declared with ARG in the Dockerfile (passed as --build-arg). Reference: https://docs.podman.io/en/stable/markdown/podman-build.1.html#build-arg-arg-value"
          items:
            type: object
            required: [name, value]
            properties:
              name:
                type: string
                description: "Build argument name"
              value:
                type: string
                description: "Build argument value"
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
          - name: namespace-name
            value: ${metadata.namespaceName}
          - name: git-repo
            value: ${parameters.repository.url}
          - name: branch
            value: main
          - name: commit
            value: ${parameters.repository.revision.commit}
          ...
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

Workflow contains:
- Parameters: Defines developer-facing parameters that can be configured when triggering an execution
- RunTemplate: An inline Workflow engine manifest with CEL expressions (${metadata.*}, ${parameters.*}, ${externalRefs['<id>'].spec.*})
- Resources: Additional Kubernetes resources needed for the workflow (e.g., ExternalSecrets for credentials)
- ExternalRefs: References to external CRs (e.g., SecretReference) resolved at runtime and injected into the CEL context
- TTLAfterCompletion: Optional duration after which completed runs are automatically deleted

### Parameters Section
Workflow CR contains "parameters" section defined using the OpenAPIV3Schema and platform engineers can define what they want to take from the developers, these will be nicely rendered in the OpenChoreo UI (Backstage) whether it is a list, enum, string, integer, etc. In the above example, I have a put a sample of docker workflow above (https://github.com/openchoreo/openchoreo/blob/release-v1.0/samples/getting-started/ci-workflows/dockerfile-builder.yaml), which asks for the url, secret for a private repository, build vars, etc as the parameters.

### CEL Expressions
Workflows has multiple cel expressions that you can use for dynamic variable values.
- ${parameters.}: access values defined in the parameters section
Eg: ${parameters.repository.url}
- ${metadata.}: you can access workflow run's labels through metadata.labels[''], or metadata.workflowRunName for the workflow run name. or metadata.namespaceName for the namespace name of the control plane that this workflow run was created.
- ${workflowplane.secretStore} - this will be used inside resources section for you to get the key vault's secret store CR's name that is attached to workflow plane.

### RunTemplate Section
"runTemplate" is the execution that will be applied to the workflow plane for the execution using the worklfow engine that you are using. The above example, run template is an argo workflow CR as we used argo workflow as the default workflow engine in OpenChoreo. PEs can map the parameters that will be given by the developers at the execution, hard coded values, or system provided values to the run template. 

### Resources Section
Workflows can require config maps, secrets, or any other resources that might be needed for the execution of the workflow in the workflow plane. This section is provided for such use cases. For example, for a ci workflow with private repository, you will need to create a secret in the workflow plane, this is crucial if the secret can be different for different developers as this case since developers might have different private repositories that they want to build from. Otherwise, PEs can configure the secret one time and dont need to add it to resources. Added resources' lifecycle is managed as workflow is deleted. As you can see the above example, you can use different cel expression when defining a resource.

### ExternalRefs Section
References to external CRs (e.g., SecretReference) resolved at runtime and injected into the CEL context. Currently, this is only limited to secret reference's which is an OpenChoreo CR. Secret Reference holds the secret's reference that you have added to a key vault. If you want to get the secret reference name from the user, and use its CR's content for defining a resource in the resources section, you can use this.


WorkflowRun CR
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
In the workflow run you have to refer which workflow abstraction that you use and provide the values for the parameters defined in that workflow. When this is applied to the k8s cluster, its controller will pick it up and execute. 

In the status of the workflowrun, it keeps what are the individual steps/actions that ran from the workflow engine (steps in argo workflows). 


## Workflow Module?
This is the default workflow engine used in OpenChoreo and currrently it is not pluggable unless the controller is updated. We are currently discussing to make this a module so that we can install different modules whether it is k8s native or not. Tektone Pipelines, azure pipelines, github actions, etc. 

## Steps for defining OpenChoreo Workflows

1. Define Argo Cluster Workflow Template 
For argo workflows, it is cluster workflow templates. We recommend writing individual task as seperate workflow templates. So it is easy to use and manage. 

1.1. Expose variables from cluster workflow templates
Expose the values that you think that can change and can be given either by the Platform engineer or the developer. Learn argo to learn about how to define variables.

Now you have all the cluster workflows templates and we can reuse those in OpenChoreo Workflows.
E.g. Docker build cluster workflow template with 3 variables git revision, build envs, build args.
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
        command:
          - sh
          - -c
        args:
          - |-
            set -e

            WORKDIR="/mnt/vol/source"
            IMAGE="{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-{{inputs.parameters.git-revision}}"
            DOCKER_CONTEXT="{{workflow.parameters.docker-context}}"
            DOCKERFILE_PATH="{{workflow.parameters.dockerfile-path}}"
            BUILD_ENV_JSON='{{inputs.parameters.build-env}}'
            BUILD_ARGS_JSON='{{inputs.parameters.build-args}}'

            # --- Parameter Validation ---
            echo ">> Image: $IMAGE"
            echo ">> Dockerfile: $DOCKERFILE_PATH"
            echo ">> Docker context: $DOCKER_CONTEXT"

            if [ ! -f "$WORKDIR/$DOCKERFILE_PATH" ]; then
              echo ">> Error: Dockerfile not found at: '$DOCKERFILE_PATH'"
              echo ">> Hint: Verify that the Dockerfile path is correct and relative to the repository root."
              echo ">> Repository contents:"
              ls -la "$WORKDIR/"
              exit 1
            fi

            if [ ! -d "$WORKDIR/$DOCKER_CONTEXT" ]; then
              echo ">> Error: Docker build context directory not found: '$DOCKER_CONTEXT'"
              echo ">> Hint: Verify that the Docker build context points to a valid directory relative to the repository root."
              echo ">> Repository contents:"
              ls -la "$WORKDIR/"
              exit 1
            fi

            mkdir -p /etc/containers
            cat > /etc/containers/storage.conf <<EOF
            [storage]
            driver = "overlay"
            runroot = "/run/containers/storage"
            graphroot = "/var/lib/containers/storage"
            [storage.options.overlay]
            mount_program = "/usr/bin/fuse-overlayfs"
            EOF

            # Build --env flags
            ENV_ARGS=""
            if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
              ENV_ARGS=$(echo "$BUILD_ENV_JSON" | \
                podman run --rm -i ghcr.io/jqlang/jq:1.7.1 -r '.[] | "--env \(.name)=\(.value)"' | \
                tr '\n' ' ')
            fi

            # Build --build-arg flags
            BUILD_ARG_ARGS=""
            if [ -n "$BUILD_ARGS_JSON" ] && [ "$BUILD_ARGS_JSON" != "[]" ]; then
              BUILD_ARG_ARGS=$(echo "$BUILD_ARGS_JSON" | \
                podman run --rm -i ghcr.io/jqlang/jq:1.7.1 -r '.[] | "--build-arg \(.name)=\(.value)"' | \
                tr '\n' ' ')
            fi

            echo ">> Building image"
            podman build -t $IMAGE -f $WORKDIR/$DOCKERFILE_PATH $ENV_ARGS $BUILD_ARG_ARGS $WORKDIR/$DOCKER_CONTEXT
            echo ">> Image built successfully"
            echo ">> Saving image"
            podman save -o /mnt/vol/app-image.tar $IMAGE
        securityContext:
          privileged: true
        volumeMounts:
          - mountPath: /mnt/vol
            name: workspace

```

2. Define OpenChoreo Workflows 

When you define the Workflows, make sure to define which workflow plane this workflow is going to be executed and if you are using argo cluster workflow templates, make sure those exist in that cluster.

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

2.1 Identify the parameters that you want to expose through the Workflow to the developer
Define the parameters section with the identified parameters that you want to get from the developer.
```yaml
  parameters:
    openAPIV3Schema:
    ...
```

2.2 Identify the parameters that you want to hardcode
The CR under runTemplate is k8s engine's CR/configs and our controller will only evaluate the cel expressions and redner it and apply to the workflow plane.

Some parameters you will hardcode and some parameters you will get from the developers. From below, branch is hardcoded to main which can be a requirement in an organization and some other paramters are exposed to the developers.

```yaml
 runTemplate:
    # This is an Argo CR and how it is defined can be learn from argo docs
    apiVersion: argoproj.io/v1alpha1
    kind: Workflow
    metadata:
      name: ${metadata.workflowRunName}
      namespace: ${metadata.namespace}
    spec:
      arguments:
        parameters:
          - name: component-name  # Label in Workflow Run CR 
            value: ${metadata.labels['openchoreo.dev/component']}
          - name: workflowrun-name # Workflow run name
            value: ${metadata.workflowRunName}
          - name: git-repo # Taken from developer
            value: ${parameters.repository.url}
          - name: branch # PE hardcoded branch
            value: main
          - name: commit # Taken from the developer
            value: ${parameters.repository.revision.commit}
          - name: build-env # Taken from the developer
            value: ${parameters.buildEnv}
          - name: build-args # Taken from the developer
            value: ${parameters.buildArgs}
          ...
      templates: # Refer Argo Cluster Workflow Templates
        - name: build-workflow
          steps:
            - - name: checkout-source # Checkout cluster workflow template
                templateRef:
                  name: checkout-source
                  clusterScope: true
                  template: checkout
            - - name: build-image # Docker build cluster workflow template
                templateRef:
                  name: containerfile-build
                  clusterScope: true
                  template: build-image
                arguments:
                  parameters: # Above defined variables and values are passed. Argo syntax.
                    - name: git-revision
                      value: '{{steps.checkout-source.outputs.parameters.git-revision}}'
                    - name: build-env
                      value: '{{workflow.parameters.build-env}}'
                    - name: build-args
                      value: '{{workflow.parameters.build-args}}'
```

3. Allow the Workflow in Component Type if this is a CI Workflow (Only for CI/Component Workflows)
Platform engineers can define what workflows a particular component type can use. For example, web application component type will only need dockerfile workflow and paketo buildpack workflow as workflows. This gives more control for platform engineers.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterComponentType
metadata:
  name: webapp
spec:
  # Restrict components to using only these ClusterWorkflows
  allowedWorkflows:
    - kind: ClusterWorkflow
      name: dockerfile-builder
    - kind: ClusterWorkflow
      name: gcp-buildpacks-builder
```

4. Now developers can create the Workflow Run referring this.

## UI/CLI Concerns

### What is a CI Workflow or Component Workflow?
Workflow can be attached to a component for CI or it can be a generic workflow. A workflow intended to be assigned to a component is called CI workflow or a component workflow and there is no such specific name. Even if a workflow is used by a component, still the workflow can be run independently if the correct parameters are given to the workflow run.

How are we making this difference in the UI/CLI?
In the UI and CLI, you can specifically see CI workflows. This is done through a label. Required for workflows intended to be used by Components. The UI and CLI use this label to identify and categorize a workflow as a CI workflow.
```yaml
metadata:
  labels:
    openchoreo.dev/workflow-type: "component"
```

### Vendor extensions?
Through openApiv3Schema, we support vendor extensions and UI or the CLI has to honour these fields. Currently, we have 5 vendor extensions and those can be attached to 5 different fields.
x-openchoreo-component-parameter-repository-url	Identifies the Git repository URL field	
x-openchoreo-component-parameter-repository-branch	Identifies the Git branch field	
x-openchoreo-component-parameter-repository-commit	Identifies the Git commit SHA field	
x-openchoreo-component-parameter-repository-app-path	Identifies the application path field	
x-openchoreo-component-parameter-repository-secret-ref	Identifies the secret reference field

These helps to provide smooth experience in the UI and CLI. You can see secrets reference are listed to select a secret for your private repository when you try to create a component and it is done through this.

If you want to add more vendor extensions, you need to modify UI/CLI to support those.

### What are the labels in the cel expressions? metadata.labels[]?
When a CI workflow executed through the workflow run, openchoreo adds two labels component name and its project name that this workflow run is attached to. If you need to pass those to the Argo Cluster workflow templates for specific tasks such as Workflow CR generation or image name creation, you can pass those as described above.

## Garbage Collection
How these workflow runs are deleted? You can define a TTL in the workflow which a ttl after completion whether it is succeeded or failed. If you have not defined that then it will live forever. Deletion of workflow will not delete workflow runs as in k8s deleting a cronjob will not delete jobs (need to chekc this) or deleting the Argo Cron Workflows will not delete Argo Workflows. Since workflow runs that is attached to components are belonged to those components, when a component is deleted, its workflow runs are automatically deleted. When a workflow run is deleted, all other resources created by that workflow is also deleted.

# Conclusion
So much information was covered in one article. More articles will come if I found out anything that I couldnt cover here. And also when we release the modular architecture of OpenChoroe Workflows.