OpenChoreo allows users to define configuration schemas in several CRDs, including `ComponentTypes (CTs)`, `Traits`, and `Workflows`.
- A `ComponentType` defines the configuration schema for components of that type. This includes both mandatory and optional parameters that must be provided when creating a Component resource.
- In addition, a `ComponentType` can define optional environment-specific configuration that may be supplied when binding or deploying a component to a particular environment.
- `Traits` support the same two use cases. A Trait can define configuration parameters required at component creation time, as well as optional environment-specific configuration provided during deployment.
- `Workflows` allow platform teams to define mandatory and optional configuration parameters that must be supplied when triggering a workflow.

In all these cases, we define the shape of user-provided configuration using a schema.

Currently, OpenChoreo uses the [Simple Schema language from Kro.](https://kro.run/api/specifications/simple-schema) While it is concise and easy to read, it has several limitations (some are pretty obvious).  More importantly, it is still evolving in Kro (currently version 0.8.5) and is not a widely adopted standard. Releasing OpenChoreo 1.0 with a configuration schema language that is still unstable and evolving may not be a good idea. We would effectively be coupling our public API contract to a language that we do not control, and that is not yet mature.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterComponentType
metadata:
name: worker
namespace: default
spec:
workloadType: deployment

schema:
envOverrides:
imagePullPolicy: 'string | enum="Always, IfNotPresent, Never" default="IfNotPresent"'
replicas: integer | minimum=1 maximum=100 default=1

parameters:
repository:
url: 'string | description="Git repository URL"'
secretRef: 'string | description="Secret reference name for private repository Git credentials (optional for public repos)" '

```
An alternative is to adopt a widely used and well-established schema language. Kubernetes CRDs, Crossplane, and many other stable projects rely on OpenAPI v3 schema for validation. OpenAPI is expressive, extensible, and already familiar to our target users. There is also a mature ecosystem around it with validation tooling, documentation generators, IDE support, and UI integrations. Rather than introducing and maintaining our own schema language, we should seriously consider leveraging an existing standard.

Below is the OpenAPI schema equivalent of the Simple Schema example shown above.
```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ClusterComponentType
metadata:
name: worker
namespace: default
spec:
workloadType: deployment

# Defines two top-level schemas called 'parameters' and 'environmentConfig' or environmentSettings
# 'openAPIV3Schema' - Indicates that the schema is based on OpenAPI. The exact same design is there in K8s CRDs. Allows us to change the schema language in the future.
schema:
openAPIV3Schema:
environmentConfig:
type: object
additionalProperties: false
properties:
imagePullPolicy:
type: string
enum:
- Always
- IfNotPresent
- Never
default: IfNotPresent

replicas:
type: integer
minimum: 1
maximum: 100
default: 1

parameters:
type: object
additionalProperties: false
properties:
repository:
type: object
additionalProperties: false
properties:
url:
type: string
description: Git repository URL
secretRef:
type: string
description: Secret reference name for private repository Git credentials (optional for public repos)


```

OpenAPI is also designed to support vendor-specific extensions.

In OpenChoreo, we generate Backstage templates for ComponentTypes, Traits, and Workflows. With the current Simple Schema approach, there is no clean way to attach UI-specific metadata required by Backstage. OpenAPI provides a structured and extensible mechanism for addressing this requirement. For example, we can introduce an extension such as `x-openchoreo-backstage-portal` to carry UI-related annotations without polluting the core validation schema. @kaviththiranga, please note.

```yaml
schemas:
parameters:
type: object
additionalProperties: false
properties:
repository:
type: object
additionalProperties: false
properties:
url:
type: string
description: Git repository URL
x-openchoreo-backstage-portal:
ui:field: RepoUrlPicker
ui:options:
allowedHosts:
- github.com
secretRef:
type: string
description: Secret reference name for private repository Git credentials (optional for public repos)
```

Decision:
OpenChoreo will support both simple schema and OpenAPIV3Schema as first-class schemas in 1.0. The syntax will be as follows:

```yaml
schema:
    openAPIV3Schema:
       parameters:
       envOverrides:

    # we need to finalize this name
    ocSchema:
      types: ...
      parameters: ...
      envOverrides: ...
```

Adopt `openAPIV3Schema` as the default schema and update all samples for 1.0. We'll move the current simple schema under `schema.ocSchema` (TBD) for 1.0 and it'll be available.

Context:
- samples/component-types/component-with-embedded-traits/component-with-embedded-traits.yaml
- samples/workflows/ci/docker.yaml
Current cel templating (read all): docs/templating/*

Tasks:
1. We need to do more research on this open api v3 schema and how we can adapt it to our use case.

1.1 Can we do the similar thing we did using types and parameters using openAPIV3Schema with parameters section? can we have all the objects inline?
1.1.1 Find references for this internet about this schema type and how we can use it for this. Provide me references and examples and write those into this folder.
- https://json-schema.org/
- read about open api spec and this json schema. Is this openAPIV3Schema is json schema?

1.1.2 Write a similar schema using openAPIV3Schema for the component type and workflow examples we have in our samples. Write those inside the folder "samples/try-out/16. OpenAPI V3 Shema/".
- samples/component-types/component-with-embedded-traits/component-with-embedded-traits.yaml
- samples/workflows/ci/docker.yaml

1.2 What are the problems you see if we support this approach? any limitations? any missing features? any challenges for us to implement this?

1.3 Since now you have some context about the schema of this openapiv3schema, write a markdown file with a reference yaml file containing all the schenarios that we support so far.
- Minimum, maximum, default values, type, descriptions, tags, etc
