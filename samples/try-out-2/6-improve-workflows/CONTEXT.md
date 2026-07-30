# Workflow Template Improvements: podman-runner Migration

## What Changed

Migrated all 8 workflow templates from using `quay.io/podman/stable:v5.8.2` (Fedora-based) and `alpine:3.23.4` to `ghcr.io/openchoreo/podman-runner:v1.2` (Alpine-based, pre-built with podman, pack, jq, yq, curl).

### Files Changed

```
samples/getting-started/workflow-templates/
  containerfile-build.yaml
  paketo-buildpacks-build.yaml
  gcp-buildpacks-build.yaml
  ballerina-buildpack-build.yaml
  publish-image-k3d.yaml
  publish-image.yaml
  generate-workload-k3d.yaml
  generate-workload.yaml
```

### Per-Template Changes

| Template | Old Image | Changes |
|---|---|---|
| containerfile-build | `quay.io/podman/stable:v5.8.2` + `ci-tools` init | Removed ci-tools init container, tools volume, PATH/LD_LIBRARY_PATH exports. jq is now in the base image. |
| paketo-buildpacks-build | `quay.io/podman/stable:v5.8.2` | Removed `dnf install jq`, removed runtime pack CLI download (`curl | tar`), `/tmp/pack` -> `pack`. Added runtime `containers.conf` for nested container support. |
| gcp-buildpacks-build | `quay.io/podman/stable:v5.8.2` | Same as paketo. |
| ballerina-buildpack-build | `quay.io/podman/stable:v5.8.2` | Same as paketo. |
| publish-image-k3d | `quay.io/podman/stable:v5.8.2` | Image swap only. |
| publish-image | `quay.io/podman/stable:v5.8.2` | Image swap only. |
| generate-workload-k3d | `alpine:3.23.4` + `ci-tools` init | Removed ci-tools init container, removed LD_LIBRARY_PATH. Kept tools volume + copy-occ init (occ binary comes from openchoreo-cli image). Added `mkdir -p /tools/bin` to copy-occ command. |
| generate-workload | `alpine:3.23.4` + `ci-tools` init | Same as generate-workload-k3d. |

## Why

1. **Eliminate `dnf install jq`** in buildpack templates -- this was downloading and installing jq from Fedora repos on every build, adding 30-60s of silent delay with output redirected to `/dev/null`.
2. **Eliminate runtime `pack` CLI download** -- each buildpack build was downloading pack v0.40.1 from GitHub on every run (~5s). The podman-runner image has pack v0.40.6 pre-installed at `/usr/local/bin/pack`.
3. **Remove ci-tools init container** from containerfile-build and generate-workload templates -- the ci-tools init existed to copy Alpine musl-linked jq/yq/curl binaries into a shared volume for use in the Fedora podman container (which couldn't run them natively without LD_LIBRARY_PATH). With the Alpine-based podman-runner, jq/yq/curl are native and no cross-libc workaround is needed.
4. **Consistent base image** across all workflow steps.

## Nested Container Support (Buildpack Templates Only)

The 3 buildpack templates (paketo, gcp, ballerina) run `podman system service` and then `pack build`, which creates containers inside podman (nested containers). The `quay.io/podman/stable` image ships with a `/etc/containers/containers.conf` that enables this:

```toml
[containers]
netns="host"
userns="host"
ipcns="host"
utsns="host"
cgroupns="host"
cgroups="disabled"
log_driver = "k8s-file"
[engine]
cgroup_manager = "cgroupfs"
events_logger="file"
runtime="crun"
```

The podman-runner image does not include this config (keeping it generic). Instead, the 3 buildpack templates write this config at runtime before starting `podman system service`. This was discovered when the GCP buildpack build failed with:

```
crun: create '/sys/fs/cgroup/libpod_parent': Permission denied: OCI permission denied
ERROR: failed to build: executing lifecycle: unable to upgrade to tcp, received 500
```

The key setting is `cgroups="disabled"` which prevents podman from trying to create cgroup directories for sub-containers.

This config is NOT added to:
- `containerfile-build` -- uses `podman build` directly, no nested containers
- `publish-image` / `publish-image-k3d` -- uses `podman load/push`, no nested containers
- `generate-workload` / `generate-workload-k3d` -- doesn't use podman at all

## The podman-runner Base Image

Source: `install/base-images/Dockerfile`
Published as: `ghcr.io/openchoreo/podman-runner:v1.2`

Pre-installed tools:
- `podman` 5.7.0
- `pack` v0.40.6 (buildpacks CLI)
- `jq` 1.8.1
- `yq` 4.49.2
- `curl` 8.19.0
- `fuse-overlayfs` 1.16
- `iptables` 1.8.11

Alpine-based, no `containers.conf` baked in (applied at runtime by templates that need it).

## generate-workload: copy-occ Init Container

The generate-workload templates still need an init container to copy the `occ` binary from the `ghcr.io/openchoreo/openchoreo-cli` image into a shared volume. This is because `occ` lives in a separate image and the only Kubernetes-native way to share a binary between images is via an init container + emptyDir volume.

The ci-tools init container previously created `/tools/bin` before copy-occ ran. After removing ci-tools, the copy-occ command was updated from:

```yaml
command: [cp, /usr/local/bin/occ, /tools/bin/occ]
```

to:

```yaml
command: [sh, -c, "mkdir -p /tools/bin && cp /usr/local/bin/occ /tools/bin/occ"]
```

The main container still sets `PATH="/tools/bin:${PATH}"` so `occ` is available on the path. `LD_LIBRARY_PATH` was removed since occ doesn't need Alpine musl library overrides in the podman-runner image.
