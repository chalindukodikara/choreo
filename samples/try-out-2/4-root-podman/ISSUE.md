## Why workflows needed root access

All CI workflow templates ran with `privileged: true` because Podman requires root for container-in-container operations — kernel namespace manipulation, fuse-overlayfs mounts, and running a Podman daemon (for `pack build`).

## Approach: Rootful Podman in User Namespaces

Instead of switching to a different build tool, we keep Podman but add `hostUsers: false` (Kubernetes user namespaces). This maps UID 0 inside the container to an unprivileged host UID — so `privileged: true` only grants capabilities within the user namespace, not on the host.

We evaluated three options for Dockerfile builds:

| Option | Pros | Cons |
|---|---|---|
| **Buildah** | Daemonless, no privileged needed | No multi-stage parallelism, slower for complex Dockerfiles |
| **BuildKit** | Parallel multi-stage builds, superior caching | Still 0.x, adds complexity (sidecar or daemonless script) |
| **Podman + `hostUsers: false`** | Minimal diff from existing templates, same CLI | Still says `privileged: true` (safe within user NS) |

We chose **Podman with `hostUsers: false`** to minimize changes — the scripts stay nearly identical to the old ones, just with a different base image (`quay.io/podman/stable:v5.8.2`) and overlay storage on an emptyDir volume. This also removes the dependency on the custom `ghcr.io/openchoreo/podman-runner:v1.1` image — one fewer image to build and maintain.

## Buildpack Builds

`pack build` requires a running Docker/Podman daemon, which was the main reason for root access. With `hostUsers: false`, Podman runs its daemon rootfully inside the user namespace — `pack` connects via `DOCKER_HOST` as before. No changes to the build logic.

## Workload Generation

`generate-workload` doesn't do container-in-container operations — it only needs CLI tools (jq, yq, occ, curl). It now runs on `alpine:3.23.4` with tools injected via init containers, fully unprivileged (`runAsNonRoot: true`, `drop: [ALL]`). No `hostUsers: false` needed.

## Kubernetes Requirement

User namespaces (`hostUsers: false`) are GA in Kubernetes 1.33+. The k3d setup uses **k3s v1.35**.
