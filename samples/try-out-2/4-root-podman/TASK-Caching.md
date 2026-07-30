We need to think about caching as well when we think about rootless aproaches. It seems we can use the in cluster for caching docker builds. But not sure about the buildpack builds.

## Question
1. Can we think about mirror caching instead of layer caching for buildpacks? install/k3d/registry-cache/README.md
2. Are we doing the layer caching for docker builds?

## References
1. A registry is running in the workflow plane, "openchoreo-workflow-plane" namespace. This is considered as the upstream registry where built images will be pushed.
2. Mirroring: install/k3d/registry-cache/README.md


## Concerns
Because the cache is stored externally in your container registry rather than on the ephemeral disk of a specific Argo pod, any workflow, running on any node, at any time can access those cached layers, as long as it has network access and registry credentials.

Here is a breakdown of how the builders know when to use the cache across different workflows, and the common pitfalls to avoid when setting it up.

How Cross-Workflow Caching Actually Works
Both Kaniko and BuildKit use a deterministic hashing system to figure out if they can reuse a layer. They do not care which Argo Workflow is executing the build; they only care about the inputs.

For every step in your Dockerfile, the builder calculates a cryptographic hash based on three things:

The parent layer: The hash of the layer immediately preceding this step.

The command: The exact string of the command being executed (e.g., RUN npm install).

The files (if applicable): If the step is COPY . ., it hashes the contents of the files being copied.

If Workflow A runs and pushes a layer with the hash sha256:12345 to your cache registry, and then Workflow B runs the exact same Dockerfile an hour later, Workflow B will calculate the same hash (sha256:12345). It will ask the registry, "Do you have this layer?" The registry says yes, and Workflow B downloads it instead of building it from scratch.

Configuring for Cross-Workflow Success
To ensure different workflows share the cache effectively, you just need to point them to the same shared location.

For Kaniko: Ensure all workflows use the same --cache-repo argument.
--cache-repo=ghcr.io/my-org/my-app-cache

For BuildKit: Ensure workflows use matched import and export references.
--export-cache type=registry,ref=ghcr.io/my-org/my-app-cache:main
--import-cache type=registry,ref=ghcr.io/my-org/my-app-cache:main

⚠️ Two Critical "Gotchas" for CI/CD Caching
When multiple Argo Workflows share a cache, you will run into two classic CI/CD problems. Here is how to avoid them:

1. The Git Timestamp Problem (Cache Busting)
When Argo clones your Git repository to start a workflow, Git assigns the current checkout time as the timestamp for all the files. Because Kaniko and BuildKit include file metadata (like timestamps) in their hash calculations, the same file checked out at two different times will generate two different hashes! This immediately busts your COPY cache, forcing a full rebuild even if the code hasn't changed.

The Fix: You need to ensure your build contexts have deterministic timestamps. If you use BuildKit, there are experimental flags to ignore timestamps. Otherwise, a common CI step before building is to run a script that resets file timestamps to the Git commit time (e.g., using a tool like git-restore-mtime).

2. Cache Thrashing Between Branches
If a developer running Workflow A on a feature-branch updates a dependency, and Workflow B runs on the main branch, they might start overwriting each other's shared cache in the registry, leading to cache misses for both of them.

The Fix (Cache Scoping): You should dynamically scope your cache tags based on the Git branch.

BuildKit is excellent at this: you can tell it to import cache from both the main branch and the current PR branch, but only export new cache to the PR branch.

Kaniko is less flexible, so you usually just append the branch name to the cache repo (e.g., --cache-repo=ghcr.io/my-org/my-app-cache:feature-branch).
