# Task: Eliminate Privileged (Root) Access from Argo Workflow Build Templates

## Problem

All OpenChoreo CI workflow templates currently run with `securityContext: privileged: true` because they use Podman (via `ghcr.io/openchoreo/podman-runner:v1.1`) for container image building and pushing. This is a significant security concern:

1. **Cloud environments restrict privileged pods.** Most managed Kubernetes services (EKS, GKE, AKS) enforce Pod Security Standards that reject `privileged: true` containers. Enterprises applying the "Restricted" or "Baseline" Pod Security Standard cannot use OpenChoreo's build workflows at all.
2. **Blast radius.** A privileged container has full host kernel access. A compromised build step (e.g., malicious dependency in a user's source code) could escape the container and compromise the node.
3. **Compliance.** SOC 2, PCI-DSS, and similar frameworks flag privileged workloads. Enterprise adoption is blocked by security review.

### Why Podman Currently Needs Privileged Mode

The `podman-runner` container requires `privileged: true` for three reasons:

1. **Container-in-container:** `podman build` and `podman run` (used to invoke `pack`, `jq`, `yq`, `occ`) create nested containers, which require kernel namespace manipulation.
2. **Overlay filesystem:** The storage config writes to `/etc/containers/storage.conf` and uses `fuse-overlayfs`, which needs FUSE device access and elevated mount capabilities.
3. **`pack build` with Podman daemon:** The buildpacks workflows start `podman system service` as a daemon, then `pack build` talks to it via the Docker-compatible API. This daemon needs namespace/cgroup access.

### Affected Workflow Templates

All use `privileged: true` + `ghcr.io/openchoreo/podman-runner:v1.1`:

| Template | Purpose | Uses `podman build`? | Uses `pack build`? | Uses `podman run` for tooling? |
|---|---|---|---|---|
| `containerfile-build.yaml` | Build from Dockerfile/Containerfile | Yes | No | Yes (jq for build args) |
| `gcp-buildpacks-build.yaml` | Build with GCP Buildpacks | No | Yes | Yes (jq for env args) |
| `paketo-buildpacks-build.yaml` | Build with Paketo Buildpacks | No | Yes | Yes (jq for env args) |
| `ballerina-buildpack-build.yaml` | Build with Ballerina Buildpack | No | Yes | Yes (jq for env args) |
| `publish-image.yaml` | Push built image to registry | No (load + tag + push) | No | No |
| `generate-workload.yaml` | Generate workload CR + push to API | No | No | Yes (occ, yq, jq via podman run) |

### Runner Image (`install/base-images/Dockerfile`) — To Be Replaced

The current `ghcr.io/openchoreo/podman-runner:v1.1` is built from `install/base-images/Dockerfile`:

- **Base:** Alpine 3.23
- **Installed:** Podman 5.7.0, fuse-overlayfs 1.16, iptables, curl
- **Bundled:** pack CLI v0.40.1 (Cloud Native Buildpacks)
- **Config:** `DOCKER_HOST=unix:///run/podman/podman.sock`

With the new plan, **Podman and pack CLI are no longer needed.** Each workflow step will use its own purpose-built image:

| Workflow Step | Image |
|---|---|
| Dockerfile builds | `quay.io/buildah/stable` or `moby/buildkit:rootless` (PoC both) |
| Buildpack builds | CNB builder image directly (e.g., `paketobuildpacks/builder-jammy-full`) |
| Image publish | `gcr.io/go-containerregistry/crane` or `quay.io/skopeo/stable` |
| Tooling (generate-workload, etc.) | Lightweight tooling image with jq, yq, occ, curl |

For the tooling image, two options:
1. **Custom base image** — replace `install/base-images/Dockerfile` with a slim Alpine image that only bundles `jq`, `yq`, `occ`, and `curl`. No Podman, no pack, no fuse-overlayfs.
2. **Plain Alpine** — use `alpine:3.23` directly and install `jq`/`yq`/`curl` via `apk` in the workflow script, with `occ` copied from an init container or mounted from a shared volume. No custom image to maintain.

Decision on which approach to use for the tooling image is part of the PoC.

### Reference Code

- Runner image Dockerfile: `install/base-images/Dockerfile`
- CRD types: `api/v1alpha1/workflowrun_types.go`, `api/v1alpha1/workflow_types.go`
- Controller: `internal/controller/workflowrun/*`
- CI workflow definitions: `samples/getting-started/ci-workflows/*`
- Workflow templates: `samples/getting-started/workflow-templates/`
- Example component using workflows: `samples/from-source/services/go-google-buildpack-reading-list/reading-list-service.yaml`

---

## Prerequisites

### Kubernetes Version: 1.35+ (k3d: `rancher/k3s:v1.35`)

Rootless container builds require Kubernetes **1.33+** where [user namespaces are enabled by default](https://kubernetes.web.cern.ch/blog/2025/06/19/rootless-container-builds-on-kubernetes/). This allows `hostUsers: false` in pod specs, mapping container UID 0 to an unprivileged host UID — the foundation for all rootless approaches.

The k3d cluster version has been updated to **`rancher/k3s:v1.35`**, up from the previous 1.32.9 default.

---

## Plan

### 1. Replace Runner Image + Bundle Tooling

**Goal:** Retire `ghcr.io/openchoreo/podman-runner:v1.1`. Each workflow step uses a purpose-built image instead of one monolithic privileged runner. Eliminate `podman run` for CLI tools that don't need a container runtime.

**Changes:**
- Replace `install/base-images/Dockerfile` with either:
  - A **slim custom tooling image** (Alpine + jq, yq, occ, curl — no Podman, no pack), or
  - **Plain `alpine:3.23`** with tools installed at runtime or via init containers (no custom image to maintain).
- Replace all `podman run --rm -i ghcr.io/jqlang/jq:1.7.1 ...` with direct `jq ...` calls.
- Replace all `podman run --rm mikefarah/yq ...` with direct `yq ...` calls.
- Replace all `podman run --rm ghcr.io/openchoreo/openchoreo-cli:latest-dev ...` with direct `occ ...` calls.
- Dockerfile build steps switch to `quay.io/buildah/stable` or `moby/buildkit:rootless` (Step 2).
- Buildpack build steps switch to the CNB builder image directly (Step 3).
- Publish steps switch to `crane` or `skopeo` image (Step 4).

**Affected templates:** All — every template currently using `ghcr.io/openchoreo/podman-runner:v1.1`.

**Why first:** This is the foundation. Once Podman is removed from the image, `privileged: true` has no reason to exist in the tooling-only templates (`generate-workload`), and the build templates can switch to dedicated rootless images.

---

### 2. Dockerfile/Containerfile Builds — Test Buildah and BuildKit

**Goal:** Replace `podman build` with a rootless alternative. Evaluate both options and pick the best fit.

#### Option A: Buildah (Rootless)

- Replace `podman build` with `buildah build` (nearly 1:1 CLI swap — Podman uses Buildah internally).
- Replace `podman push` with `buildah push` or `skopeo copy`.
- Configure rootless storage on emptyDir volumes with `fuse-overlayfs` or native `overlay`.
- Set `hostUsers: false` + `runAsUser: 1000` + `runAsGroup: 1000` in pod security context.

| Aspect | Details |
|---|---|
| Daemon | None (daemonless by design) |
| Maintenance | Red Hat, used in OpenShift |
| Strengths | Simplest migration, smallest attack surface, CERN production-proven |
| Weaknesses | No multi-stage parallelism, manual storage config, fuse-overlayfs slow for large copies |

#### Option B: BuildKit (Rootless)

- Run BuildKit as a sidecar (`moby/buildkit:rootless`) or use `buildctl-daemonless.sh`.
- Build step uses `buildctl --addr unix:///run/buildkit/buildkitd.sock build ...`
- BuildKit pushes directly to registry as part of the build output.

| Aspect | Details |
|---|---|
| Daemon | Sidecar or daemonless script |
| Maintenance | Docker/Moby ecosystem |
| Strengths | Parallel multi-stage builds, superior layer caching, registry-based cache |
| Weaknesses | RootlessKit conflicts with K8s user namespaces, may need `seccomp: Unconfined`, sidecar adds Argo complexity |

**Evaluation criteria:** Build both PoCs against `containerfile-build.yaml`, compare build time, security context requirements, and workflow complexity. Pick one.

**Kaniko is ruled out** — archived by Google (June 2025), uncertain long-term maintenance.

---

### 3. Buildpack Builds — Direct CNB Lifecycle Creator

**Goal:** Replace `pack build` (which needs `podman system service` running as a privileged daemon) with direct invocation of the CNB lifecycle creator inside the builder image.

**Changes:**
- Replace `pack build` in `gcp-buildpacks-build.yaml`, `paketo-buildpacks-build.yaml`, and `ballerina-buildpack-build.yaml` with direct lifecycle calls:
  ```yaml
  container:
    image: docker.io/paketobuildpacks/builder-jammy-full:0.3.603
    command: ["/cnb/lifecycle/creator"]
    args: ["-app=/workspace", "-run-image=...", "my-image:tag"]
  ```
- Remove `podman system service` daemon startup from all buildpack templates.
- Remove the `DOCKER_HOST` socket dependency.

**Why this approach:**
- No daemon, no privileged mode, no user namespaces required for the build itself.
- The CNB lifecycle is designed to run unprivileged — it's the same code `pack` invokes, just without the Docker/Podman wrapper.
- Eliminates the most complex privileged dependency (a running container daemon inside an Argo Workflow pod).

**Affected templates:**
- `gcp-buildpacks-build.yaml` (GCP Buildpacks)
- `paketo-buildpacks-build.yaml` (Paketo Buildpacks)
- `ballerina-buildpack-build.yaml` (Ballerina Buildpack)

---

### 4. Image Publish — Rootless Push

**Goal:** Replace `podman load/tag/push` in `publish-image.yaml` with a rootless alternative.

**Changes:**
- If Buildah is chosen: use `buildah push` or `skopeo copy` (both rootless, daemonless).
- If BuildKit is chosen: BuildKit can push as part of the build output (`--output type=image,push=true`).
- For CNB lifecycle builds: use `crane push` or `skopeo copy` to push the built OCI image.
- Set `hostUsers: false` + non-root security context.

---

### 5. Generate Workload — Rootless

**Goal:** Remove `privileged: true` from `generate-workload.yaml`.

After Step 1 (bundling jq/yq/occ), this template no longer needs `podman run` at all — it only uses CLI tools and `curl`. The `privileged: true` can be dropped entirely and replaced with a minimal non-root security context.

---

## Execution Order

| Step | What | Complexity | Removes `privileged`? |
|---|---|---|---|
| 1 | Bundle jq/yq/occ into runner image | Low | Partially (tooling calls) |
| 2 | PoC Buildah vs BuildKit for Dockerfile builds | Medium | Yes (for `containerfile-build`) |
| 3 | CNB lifecycle for Buildpack builds | Medium | Yes (for all buildpack templates) |
| 4 | Rootless image publish | Low | Yes (for `publish-image`) |
| 5 | Drop privileged from `generate-workload` | Low | Yes (after step 1) |

Steps 1 and 5 can land first as quick wins. Steps 2-4 can proceed in parallel once Step 1 is done.

samples/getting-started/workflow-templates/*

samples/getting-started/ci-workflows/*