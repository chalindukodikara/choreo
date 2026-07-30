# Remove privileged Podman usage from OpenChoreo build workflows

## Goal

OpenChoreo's sample CI workflows currently build and publish container images from Argo Workflow pods that run `ghcr.io/openchoreo/podman-runner` with `securityContext.privileged: true`. This works for local demos, but it is a poor fit for managed Kubernetes clusters and enterprise environments where privileged pods are commonly blocked by policy.

Replace or redesign the image build/publish path so OpenChoreo component workflows can run without privileged pods while preserving the developer-facing workflow contract.

## Problem Statement

Today the build templates use Podman inside the workflow pod:

- Dockerfile/Containerfile builds run `podman build` and write `/mnt/vol/app-image.tar`.
- Buildpack builds run `pack build --docker-host inherit` after starting a Podman service, then save the resulting image with `podman save`.
- Publish steps load the tar with Podman, tag it for the target registry, and push it.

Because Podman is being run inside a Kubernetes container with overlay storage, the templates opt into `securityContext.privileged: true`. This asks cluster operators to allow a high-risk pod capability just to build application images.

The task is not simply "remove one YAML field". The replacement must still support:

- Dockerfile/Containerfile builds.
- Buildpack builds for GCP, Paketo, and Ballerina builders.
- Build environment variables and build arguments.
- Private registry push credentials.
- The current OpenChoreo workflow model where `Workflow`/`WorkflowRun` render Argo workflows and reusable `ClusterWorkflowTemplate` steps.
- The existing handoff between build and publish steps, unless the task intentionally redesigns that handoff.

## Current Implementation References

Workflow API and controller:

- `api/v1alpha1/workflow_types.go`
- `api/v1alpha1/workflowrun_types.go`
- `internal/controller/workflowrun/*`

Developer-facing sample workflows:

- `samples/getting-started/ci-workflows/*`
- `samples/from-source/services/go-google-buildpack-reading-list/reading-list-service.yaml`

Privileged workflow templates:

- `samples/getting-started/workflow-templates/containerfile-build.yaml`
- `samples/getting-started/workflow-templates/gcp-buildpacks-build.yaml`
- `samples/getting-started/workflow-templates/paketo-buildpacks-build.yaml`
- `samples/getting-started/workflow-templates/ballerina-buildpack-build.yaml`
- `samples/getting-started/workflow-templates/publish-image.yaml`
- `samples/getting-started/workflow-templates/publish-image-k3d.yaml`
- `samples/getting-started/workflow-templates/generate-workload.yaml`

Note: the original task listed only the build templates. If the goal is "no privileged pods in the build workflow", the publish and workload-generation templates must also be included because they use the same privileged Podman runner pattern.

## Solution Options

### Option 1: Rootless Podman / Buildah in the existing runner image

Keep the current workflow shape and replace privileged Podman with rootless Podman or Buildah.

How it would work:

- Build a new runner image configured for rootless execution.
- Run as a non-root user.
- Use user namespaces, subordinate UID/GID ranges, and rootless storage under the user's home directory.
- Use `fuse-overlayfs` where native overlay is unavailable.
- Move container storage config from `/etc/containers/storage.conf` to `$HOME/.config/containers/storage.conf`.
- Replace `podman run ghcr.io/jqlang/jq` with the `jq` binary in the runner image so JSON parsing does not require nested containers.
- Test whether `podman build`, `podman save`, `podman load`, and `podman push` work without `privileged: true` on the supported Kubernetes distributions.

Pros:

- Smallest conceptual change to current templates.
- Preserves the `/mnt/vol/app-image.tar` handoff.
- Podman and Buildah already understand Dockerfile/Containerfile builds.

Cons:

- Rootless Podman still depends on cluster/node support: user namespaces, `/etc/subuid` and `/etc/subgid` inside the runner image, `newuidmap`/`newgidmap`, `fuse-overlayfs` or compatible kernel overlay, and sometimes relaxed seccomp/AppArmor behavior.
- Running rootless container engines inside Kubernetes can be fragile across managed clusters.
- Buildpack flow still depends on `pack` talking to a daemon-like Podman service.

Best fit:

- A short-term migration if OpenChoreo wants minimum workflow changes and can document the exact cluster prerequisites.

### Option 2: Rootless BuildKit for Dockerfile/Containerfile builds

Use BuildKit in rootless mode for Dockerfile/Containerfile workflows.

How it would work:

- Replace `podman build` with `buildctl` or a daemonless BuildKit invocation.
- Build and push directly to the target registry, or export an OCI/docker tar if the publish step must remain separate.
- Run BuildKit with the rootless image and Kubernetes security context recommended by the BuildKit project.
- Configure cache export/import later if needed.

Pros:

- Purpose-built for modern Dockerfile builds.
- Strong cache support.
- Docker's Kubernetes Buildx driver supports `rootless=true` and creates pods without `securityContext.privileged`.
- Avoids running a full Podman service in every build step.

Cons:

- Does not directly solve buildpack workflows unless those are separately redesigned.
- Rootless BuildKit still has node/kernel prerequisites and may need unconfined seccomp/AppArmor or `--oci-worker-no-process-sandbox` depending on the environment.
- Direct push changes the current "build tar, then publish" workflow unless OpenChoreo keeps tar export.

Best fit:

- Recommended path for Dockerfile/Containerfile builds.

### Option 3: Cloud Native Buildpacks lifecycle as a platform integration

Stop using `pack build --docker-host inherit` inside the workflow pod. Instead, integrate with the CNB lifecycle/platform API so buildpack workflows can export directly to an OCI registry using registry credentials.

How it would work:

- Treat OpenChoreo as the buildpack platform.
- Use CNB lifecycle phases or `creator` for trusted builders.
- Provide registry credentials to lifecycle phases that need image repository access.
- Keep detector/builder execution non-root, matching the buildpacks security model.
- Decide whether images are pushed directly by the buildpack step or exported into a tar for the existing publish step.

Pros:

- Aligns with the buildpacks platform model.
- Avoids a Docker/Podman daemon socket for buildpack builds.
- Can reduce credential exposure by separating lifecycle phases later.

Cons:

- More implementation work than swapping Podman flags.
- Requires careful mapping of current `pack build` behavior to lifecycle flags, builder/run image handling, volumes, cache, and registry auth.
- Ballerina-specific volume behavior must be tested.

Best fit:

- Recommended path for GCP, Paketo, and Ballerina buildpack workflows once the team is ready to properly own buildpacks as a platform feature.

### Option 4: Kaniko or a maintained Kaniko fork for Dockerfile builds

Use Kaniko-style userspace Dockerfile builds.

How it would work:

- Replace `podman build` with Kaniko executor for Dockerfile workflows.
- Push directly to the registry from the build step.

Pros:

- Designed to build Dockerfiles in Kubernetes without a Docker daemon.
- Simple mental model for many CI users.

Cons:

- The original `GoogleContainerTools/kaniko` repository is archived and no longer maintained.
- Fork selection and long-term maintenance become an OpenChoreo concern.
- Does not solve buildpack workflows.
- Direct push likely changes the current build/publish separation.

Best fit:

- Not recommended as the default unless the project deliberately chooses and validates a maintained fork.

### Option 5: Remote builder service

Move image building out of the workflow pod entirely. The workflow submits build requests to a dedicated builder service that owns the required privileges or runs in a separately governed build cluster.

How it would work:

- OpenChoreo workflow steps package the source context or point to the checked-out source.
- A builder service performs the build and push.
- WorkflowRun status tracks the external build result.

Pros:

- Keeps application workflow pods unprivileged.
- Centralizes cache, credentials, audit, and resource controls.
- Can support BuildKit, buildpacks, and future builders behind one platform API.

Cons:

- Largest architectural change.
- Requires new APIs, operational model, and failure handling.
- Less suitable for a quick sample workflow fix.

Best fit:

- Long-term platform direction if build isolation and governance are first-class requirements.

## Recommended Approach

Use a two-track approach:

1. For Dockerfile/Containerfile workflows, prototype rootless BuildKit first.
2. For buildpack workflows, prototype direct CNB lifecycle usage instead of `pack` plus Podman.

Keep rootless Podman/Buildah as a compatibility fallback only if it proves reliable on the target clusters. Avoid adopting original Kaniko as the default because it is archived.

The first milestone should define whether OpenChoreo still requires `/mnt/vol/app-image.tar`. If the workflow can push directly from the build step, the publish step can become a lightweight status/output step or be removed. If the tar handoff must remain, each replacement builder must prove it can export a tar that the unprivileged publish path can push without Podman.

## Acceptance Criteria

- No build-related Argo workflow pod uses `securityContext.privileged: true`.
- Dockerfile/Containerfile workflow still supports:
  - `repository.url`
  - `repository.revision.branch`
  - `repository.revision.commit`
  - `repository.appPath`
  - `docker.context`
  - `docker.filePath`
  - `buildEnv`
  - `buildArgs`
- GCP, Paketo, and Ballerina buildpack workflows still support `appPath` and build environment variables.
- Private registry push continues to work with `registry-push-secret`.
- The workflow output parameter `image` still resolves to the final pushed image reference.
- The generated workload still receives the expected image reference and annotations.
- The solution works on local k3d and at least one managed Kubernetes target with restricted Pod Security admission.
- Documentation states all node/kernel/runtime prerequisites for the chosen builder.
- Tests or reproducible smoke steps cover:
  - Public Dockerfile build.
  - Public buildpack build.
  - Private registry push.
  - Failure path for missing Dockerfile/app path.
  - Verification that rendered workflow pods are not privileged.

## Investigation Plan

1. Inventory every `privileged: true` workflow template used by the getting-started CI path.
2. Decide whether the build/publish split should stay or whether builders should push directly.
3. Prototype rootless BuildKit for `containerfile-build.yaml`.
4. Prototype CNB lifecycle direct registry export for one buildpack template.
5. Update `publish-image.yaml` and `publish-image-k3d.yaml` or remove them from the build path.
6. Remove `podman run ... jq` usages by adding `jq` to the runner image or using shell-safe JSON handling from a purpose-built helper image.
7. Run the sample workflows end to end.
8. Capture the cluster prerequisites and final architecture decision in docs.

## Research Notes

- Kubernetes user namespaces can map root inside a pod to an unprivileged host user. Current Kubernetes docs describe pod user namespaces as stable and explain the `hostUsers: false` opt-in model.
- Podman supports rootless mode, but it requires user namespace support and subordinate UID/GID ranges.
- Docker Buildx's Kubernetes driver supports `rootless=true` and creates builder pods without `securityContext.privileged`.
- BuildKit rootless mode has practical limitations around overlayfs, `fuse-overlayfs`, seccomp/AppArmor, and node sysctls that must be validated on each target cluster.
- Cloud Native Buildpacks already separates lifecycle phases; detector and builder phases run as non-root, while analyzer/restorer/exporter need registry or daemon access.
- Kaniko can build Dockerfiles in userspace without a Docker daemon, but the original Google repository is archived.

References:

- https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
- https://docs.podman.io/en/stable/markdown/podman.1.html#rootless-mode
- https://docs.docker.com/build/builders/drivers/kubernetes/#rootless-mode
- https://github.com/moby/buildkit/blob/master/docs/rootless.md
- https://buildpacks.io/docs/for-platform-operators/concepts/lifecycle/
- https://github.com/GoogleContainerTools/kaniko
