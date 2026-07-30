# Rootless Workflow Templates — Context & Current State

## Problem

All OpenChoreo CI workflow templates ran with `securityContext: privileged: true` because they
used `ghcr.io/openchoreo/podman-runner:v1.1` for container image building, pushing, and CLI
tooling (jq, yq, occ via `podman run`). This blocked deployment on managed Kubernetes (EKS,
GKE, AKS) that enforce Pod Security Standards, created host-level blast radius from compromised
builds, and failed SOC 2/PCI-DSS compliance reviews.

### Why Podman Needed Privileged Mode

1. `podman build` / `podman run` create nested containers requiring kernel namespace manipulation
2. Storage config uses `fuse-overlayfs` which needs FUSE device access and elevated mount capabilities
3. Buildpack workflows (`pack build`) start `podman system service` as a daemon that needs namespace/cgroup access

## Solution — Current Approach

Replace the monolithic `ghcr.io/openchoreo/podman-runner:v1.1` with `quay.io/podman/stable:v5.8.2`
and enable Kubernetes user namespaces (`hostUsers: false`) on all build/publish templates. This
keeps Podman as the build tool but confines `privileged: true` to user namespaces — UID 0 inside
maps to an unprivileged host UID.

For tooling-only templates (`generate-workload`), Podman is eliminated entirely in favor of
`alpine:3.23.4` with a `ghcr.io/openchoreo/ci-tools:1.0` init container for jq/yq/curl and an
occ init container.

**K8s requirement:** 1.33+ (user namespaces GA, `hostUsers: false` enabled by default).

---

## Current State

### Workflow Templates (ClusterWorkflowTemplate)

| Template | metadata.name | Builder | Security Model |
|---|---|---|---|
| `containerfile-build.yaml` | `containerfile-build` | Podman v5.8.2 (`podman build`) | `hostUsers: false`, `privileged: true` (inside user namespace) |
| `paketo-buildpacks-build.yaml` | `paketo-buildpacks-build` | Podman v5.8.2 + pack CLI v0.40.1 | `hostUsers: false`, `privileged: true` (inside user namespace) |
| `gcp-buildpacks-build.yaml` | `gcp-buildpacks-build` | Podman v5.8.2 + pack CLI v0.40.1 | `hostUsers: false`, `privileged: true` (inside user namespace) |
| `ballerina-buildpack-build.yaml` | `ballerina-buildpack-build` | Podman v5.8.2 + pack CLI v0.40.1 | `hostUsers: false`, `privileged: true` (inside user namespace) |
| `publish-image-k3d.yaml` | `publish-image` | Podman v5.8.2 | `hostUsers: false`, `privileged: true` (inside user namespace) |
| `publish-image.yaml` | `publish-image` | Podman v5.8.2 | `hostUsers: false`, `privileged: true` (inside user namespace) |
| `generate-workload-k3d.yaml` | `generate-workload` | Alpine 3.23.4 + ci-tools + occ | `runAsUser: 1000`, `runAsNonRoot: true`, no privileged |
| `generate-workload.yaml` | `generate-workload` | Alpine 3.23.4 + ci-tools + occ | `runAsUser: 1000`, `runAsNonRoot: true`, no privileged |

### CI Workflows (ClusterWorkflow)

| File | metadata.name | Build Template |
|---|---|---|
| `dockerfile-builder.yaml` | `dockerfile-builder` | `containerfile-build` |
| `paketo-buildpacks-builder.yaml` | `paketo-buildpacks-builder` | `paketo-buildpacks-build` |
| `gcp-buildpacks-builder.yaml` | `gcp-buildpacks-builder` | `gcp-buildpacks-build` |
| `ballerina-buildpack-builder.yaml` | `ballerina-buildpack-builder` | `ballerina-buildpack-build` |

### Production vs k3d Differences

| Template | k3d | Production |
|---|---|---|
| `publish-image` | `host.k3d.internal:10082`, `--tls-verify=false` | `ttl.sh/openchoreo-builds`, `--tls-verify=true` |
| `generate-workload` | `http://...`, `curl -s`, Host headers for routing | `https://...`, `curl -sk`, no Host headers |

---

## Architecture

### Containerfile Build (containerfile-build.yaml)

```
Pod (hostUsers: false)
├── Init: ghcr.io/openchoreo/ci-tools:1.0
│   └── Copies jq, yq, curl binaries + libs to /tools volume
└── Container: quay.io/podman/stable:v5.8.2
    ├── privileged: true (inside user namespace = safe)
    ├── Overlay storage on /storage emptyDir (10Gi)
    ├── podman build -t <image> -f <dockerfile> ...
    └── podman save -o /mnt/vol/app-image.tar
```

### Buildpack Builds (paketo/gcp/ballerina-buildpacks-build.yaml)

```
Pod (hostUsers: false)
└── Container: quay.io/podman/stable:v5.8.2
    ├── privileged: true (inside user namespace = safe)
    ├── dnf install jq
    ├── Overlay storage on /storage emptyDir (10Gi)
    ├── podman system service --time=0 (daemon on unix socket)
    ├── Downloads pack CLI v0.40.1 at runtime
    ├── pack build --docker-host inherit --builder <builder> --run-image <run-img>
    ├── Waits for image to exist in local Podman storage
    └── podman save -o /mnt/vol/app-image.tar
```

Ballerina variant additionally mounts an emptyDir at `/app` and passes
`--volume "/mnt/vol:/app/generated-artifacts:rw"` to pack build.

### Image Publish (publish-image.yaml / publish-image-k3d.yaml)

```
Pod (hostUsers: false)
└── Container: quay.io/podman/stable:v5.8.2
    ├── privileged: true (inside user namespace = safe)
    ├── Overlay storage on /storage emptyDir (10Gi)
    ├── podman load -i /mnt/vol/app-image.tar
    ├── podman tag <src> <registry>/<src>
    └── podman push (with optional --authfile from registry-push-secret)
```

### Generate Workload (generate-workload.yaml / generate-workload-k3d.yaml)

```
Pod (no privileged, runAsNonRoot)
├── Init 1: ghcr.io/openchoreo/ci-tools:1.0
│   └── Copies jq, yq, curl binaries + libs to /tools volume
├── Init 2: ghcr.io/openchoreo/openchoreo-cli:latest-dev
│   └── Copies occ binary to /tools/bin
└── Container: alpine:3.23.4
    ├── runAsUser: 1000, runAsNonRoot: true, drop: [ALL]
    ├── occ workload create (generates workload CR)
    ├── yq + jq (YAML to JSON conversion)
    ├── curl → OAuth token
    └── curl → API server (create/update workload + annotate WorkflowRun)
```

---

## Design Decisions

### 1. User Namespaces + Overlay Storage

All templates that build or manipulate container images use `podSpecPatch: '{"hostUsers": false}'`
to enable Kubernetes user namespaces. This is critical for two reasons:

1. **Security**: Container UIDs map to unprivileged host UIDs. Even `privileged: true` only
   grants capabilities within the user namespace, not on the host.
2. **Performance**: Enables the **overlay** storage driver for Podman (instead of VFS, which
   copies all layers and is much slower for large images).

Storage configuration uses a dedicated emptyDir volume at `/storage` (10Gi limit):
```ini
[storage]
driver = "overlay"
runroot = "/storage/run"
graphroot = "/storage/graph"
[storage.options.overlay]
```

Templates that don't perform container-in-container operations (generate-workload) do not need
`hostUsers: false`.

### 2. Podman in User Namespace (not BuildKit or Buildah)

All build and publish templates use `quay.io/podman/stable:v5.8.2` with `privileged: true` +
`hostUsers: false`. This keeps the existing Podman-based workflow while confining privileges
to user namespaces.

**Why Podman over BuildKit/Buildah:**
- Minimal migration delta — `podman build` / `podman save` / `podman push` commands unchanged
- Single image (`quay.io/podman/stable`) for build, save, and publish steps
- `hostUsers: false` maps UID 0 inside to an unprivileged host UID — `privileged: true` grants
  full capabilities **within the user namespace only**, not on the host
- No sidecar containers or daemon coordination needed for containerfile builds
- Buildpack builds use `pack build --docker-host inherit` against `podman system service` running
  in the same container — same proven pattern as before

### 3. Pack CLI for Buildpack Builds

Buildpack templates download `pack` CLI v0.40.1 at runtime and use `pack build` against the
local Podman daemon (`podman system service` on a Unix socket).

**Why pack CLI over direct CNB lifecycle:**
- `pack build` handles builder/run image pulling, lifecycle orchestration, and daemon export
  in a single command
- No sidecar container coordination required
- Same approach as the original templates, just with rootless Podman underneath

**Builder images pinned:**
| Buildpack | Builder | Run Image |
|---|---|---|
| Paketo | `docker.io/paketobuildpacks/builder-jammy-full:0.3.603` | `docker.io/paketobuildpacks/run-jammy-full:0.1.130` |
| GCP | `gcr.io/buildpacks/builder@sha256:5977b...` | `gcr.io/buildpacks/google-22/run@sha256:a8ccb...` |
| Ballerina | `ghcr.io/openchoreo/buildpack/ballerina:18` | `ghcr.io/openchoreo/buildpack/ballerina:18-run` |

### 4. ci-tools Init Container

A custom `ghcr.io/openchoreo/ci-tools:1.0` image bundles statically-linked jq, yq, curl
binaries. Init containers copy these to an emptyDir volume at `/tools`, which the main
container mounts read-only. This replaces `podman run` calls for CLI tools.

Used by: `containerfile-build`, `generate-workload`, `generate-workload-k3d`.

Buildpack templates instead install jq via `dnf install` since they already run on Fedora-based
`quay.io/podman/stable`.

### 5. No Custom Tooling Image for generate-workload

`generate-workload` uses plain `alpine:3.23.4` with tools injected via init containers.
No custom `ci-tools` runner image needed. The occ binary is injected from a configurable
image parameter (`ghcr.io/openchoreo/openchoreo-cli:latest-dev` by default).

This means:
- occ version can be changed via workflow parameter without rebuilding anything
- Alpine base stays up to date automatically

### 6. Ballerina `/app` Fix

The Ballerina buildpack's "Preparing Choreo Files" step runs `mkdir /app`, which fails as
UID 1000. An `emptyDir` volume mounted at `/app` on the main container provides a writable
directory. Pack build passes `--volume "/mnt/vol:/app/generated-artifacts:rw"`.

---

## Security Context Summary

| Template | Before | After |
|---|---|---|
| `containerfile-build` | `privileged: true` (host root) | `hostUsers: false`, `privileged: true` (user NS) |
| `*-buildpacks-build` | `privileged: true` (host root) | `hostUsers: false`, `privileged: true` (user NS) |
| `publish-image` | `privileged: true` (host root) | `hostUsers: false`, `privileged: true` (user NS) |
| `generate-workload` | `privileged: true` (host root) | `runAsUser: 1000, runAsNonRoot: true, drop: [ALL]` |

---

## Build Cache (Removed — Future PR)

Registry-based layer caching code has been removed from all workflow templates in this PR.
The `build-cache` parameter, cache probe logic, and registry-based cache import/export have
all been stripped out to keep this PR focused on the rootless migration.

Layer caching will be re-added in a follow-up PR. The design is documented in:
- `samples/try-out-2/4-root-podman/PLAN-Caching.md`
- `samples/try-out-2/4-root-podman/TASK-Caching.md`

---

## Remaining Work (from original TASK.md)

The original plan had 5 steps. Current status:

| Step | What | Status |
|---|---|---|
| 1 | Bundle jq/yq/occ — retire podman-runner | Done (ci-tools init container + quay.io/podman/stable) |
| 2 | Switch Dockerfile builds to BuildKit or Buildah | Not done — using Podman with user namespaces instead |
| 3 | Switch buildpack builds to CNB lifecycle | Not done — using pack CLI with Podman daemon instead |
| 4 | Rootless image publish | Partial — `hostUsers: false` but still `privileged: true` |
| 5 | Drop privileged from generate-workload | Done |

The current approach prioritizes a smaller migration delta (keep Podman everywhere, add user
namespaces) over the originally planned tool replacement (BuildKit, CNB lifecycle, crane/skopeo).
`privileged: true` inside user namespaces is functionally safe — capabilities only apply within
the namespace, not on the host.

---

## Files NOT Changed

- `workflow-templates/checkout-source.yaml` — no privileged mode, unchanged
- `component-types/*.yaml` — names preserved, no changes needed
- `from-source/**/greeting-service.yaml` etc. — original sample components unchanged
- `test/e2e/e2e_test.go` — hardcoded names preserved
- `make/lint.mk` — file lists reference same original filenames

---

## Reference

- Task description: `samples/try-out-2/4-root-podman/TASK.md`
- Original runner image: `install/base-images/Dockerfile`
- K8s user namespaces blog: https://kubernetes.web.cern.ch/blog/2025/06/19/rootless-container-builds-on-kubernetes/
