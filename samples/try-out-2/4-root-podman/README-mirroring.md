# Transparent Build Mirroring with Zot + registries.conf Mount

> **Status: Implemented.** The approach described here has been applied to `README-build-cache.md`. This file is kept as a design reference.

Replace the explicit image ref rewriting in build templates with a mounted `registries.conf` ConfigMap. Podman reads the config automatically, and Pack CLI delegates pulls to Podman via `--docker-host inherit`, so mirroring becomes transparent to both.

## Current Approach (README-build-cache.md)

Each buildpack template manually rewrites builder, run, and lifecycle image refs:

```bash
if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
  BUILDER="${CACHE_REGISTRY}/mirror/gcr.io/buildpacks/builder@sha256:..."
  RUN_IMG="${CACHE_REGISTRY}/mirror/gcr.io/buildpacks/google-22/run@sha256:..."
  pack config lifecycle-image "${CACHE_REGISTRY}/mirror/docker.io/buildpacksio/lifecycle"
fi
```

Problems:
- Every template needs ~30 lines of mirror logic duplicated per builder type
- Image refs are hardcoded in the script — updating a builder version means updating both the upstream ref and the mirror ref
- Each new upstream registry requires editing every template
- Pack CLI lifecycle-image config is a separate workaround

## Proposed Approach

Adopt the pattern from `install/k3d/registry-cache/` — mount a `registries.conf` as a ConfigMap into build pods. Podman handles mirroring transparently for all image pulls, including those delegated by Pack CLI.

### How It Works

```
ClusterWorkflow creates registries-conf ConfigMap per WorkflowRun
  → ClusterWorkflowTemplate mounts it at /etc/containers/registries.conf.d/mirrors.conf
    → Podman reads it on every pull
      → Pack CLI uses --docker-host inherit → Podman → mirror applies
      → podman build → mirror applies to FROM images
```

No explicit ref rewriting needed. Builder, run, lifecycle, and Dockerfile base images all route through the mirror automatically.

### What Changes

#### 1. Zot sync config — preserve upstream paths

Current Zot sync config rewrites paths with a `destination` prefix:

```json
{
  "prefix": "/buildpacks/**",
  "destination": "/mirror/gcr.io",
  "stripPrefix": false
}
```

This stores `gcr.io/buildpacks/builder` as `mirror/gcr.io/buildpacks/builder` in Zot, which forces the `prefix`/`location` rewriting in `registries.conf`. With that format, the `registries.conf` entry maps `gcr.io` → `${CACHE_REGISTRY}/mirror/gcr.io`.

Two options:

**Option A — Keep current Zot path layout, use `prefix`/`location` in registries.conf:**

```toml
[[registry]]
prefix = "gcr.io"
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/gcr.io"
insecure = true

[[registry]]
prefix = "docker.io"
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/docker.io"
insecure = true

[[registry]]
prefix = "ghcr.io"
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/ghcr.io"
insecure = true

[[registry]]
prefix = "quay.io"
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/quay.io"
insecure = true
```

This works with the current Zot sync configuration — no changes to Zot needed. Podman rewrites the pull path at runtime.

**Option B — Change Zot sync to preserve upstream paths, use `[[registry.mirror]]`:**

Remove the `destination` prefix from Zot sync config so images are stored at their original path. Then use the `[[registry.mirror]]` syntax:

```toml
[[registry]]
location = "gcr.io"
[[registry.mirror]]
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
insecure = true

[[registry]]
location = "docker.io"
[[registry.mirror]]
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
insecure = true
```

Cleaner, but requires changing the Zot sync config and is incompatible with the existing layer cache paths (which use `build-cache/podman/` and `build-cache/cnb/` prefixes in the same registry).

**Recommendation: Option A.** No Zot config changes, no path conflicts with layer cache repos, and the `prefix`/`location` format is what the templates already generate dynamically.

#### 2. ClusterWorkflow — create registries-conf ConfigMap

Add a `registries-conf` resource to each ClusterWorkflow that creates a ConfigMap per WorkflowRun:

```yaml
resources:
  - id: build-registries-conf
    template:
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: "${metadata.workflowRunName}-registries-conf"
        namespace: "${metadata.namespace}"
      data:
        mirrors.conf: |
          [[registry]]
          location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
          insecure = true

          [[registry]]
          prefix = "docker.io"
          location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/docker.io"
          insecure = true

          [[registry]]
          prefix = "gcr.io"
          location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/gcr.io"
          insecure = true

          [[registry]]
          prefix = "ghcr.io"
          location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/ghcr.io"
          insecure = true

          [[registry]]
          prefix = "quay.io"
          location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/quay.io"
          insecure = true
```

The ConfigMap name is per-WorkflowRun so multiple concurrent builds don't collide.

#### 3. ClusterWorkflowTemplate — mount the ConfigMap

Add a volume and volumeMount to each build template:

```yaml
volumes:
  - name: registries-conf
    configMap:
      name: "{{workflow.parameters.workflowrun-name}}-registries-conf"
      optional: true
container:
  volumeMounts:
    - mountPath: /etc/containers/registries.conf.d/mirrors.conf
      subPath: mirrors.conf
      name: registries-conf
      readOnly: true
```

Using `registries.conf.d/` drop-in directory instead of replacing the main `registries.conf` file. This preserves any existing container config and adds the mirror on top.

The `optional: true` on the ConfigMap means builds still work if the ConfigMap doesn't exist (mirror disabled or Zot not deployed) — Podman just pulls directly from upstream.

#### 4. Simplify build scripts

**Before (per-template mirror logic):**

```bash
# ~30 lines per buildpack template
UPSTREAM_BUILDER="gcr.io/buildpacks/builder@sha256:..."
UPSTREAM_RUN_IMG="gcr.io/buildpacks/google-22/run@sha256:..."
BUILDER="$UPSTREAM_BUILDER"
RUN_IMG="$UPSTREAM_RUN_IMG"

if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
  BUILDER="${CACHE_REGISTRY}/mirror/gcr.io/buildpacks/builder@sha256:..."
  RUN_IMG="${CACHE_REGISTRY}/mirror/gcr.io/buildpacks/google-22/run@sha256:..."
  pack config lifecycle-image "${CACHE_REGISTRY}/mirror/docker.io/buildpacksio/lifecycle"
  echo ">> Upstream image mirror: ${CACHE_REGISTRY}/mirror/gcr.io"
elif [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
  echo ">> Cache registry not reachable, pulling from upstream registries"
else
  echo ">> Upstream image mirror disabled"
fi
```

**After (no mirror logic needed):**

```bash
BUILDER="gcr.io/buildpacks/builder@sha256:..."
RUN_IMG="gcr.io/buildpacks/google-22/run@sha256:..."

# Mirroring is handled transparently by the mounted registries.conf.
# Podman routes pulls through Zot when the ConfigMap is present.
```

The builder and run image refs stay as upstream refs. Podman handles the remapping at pull time. No `pack config lifecycle-image` workaround needed either — the lifecycle pull also goes through the daemon.

For **containerfile-build**, the same applies — the `[[registry]] prefix/location` block currently written dynamically in the script is replaced by the mounted ConfigMap. The `podman build` `FROM` image pulls go through the mirror automatically.

#### 5. What stays the same

**Layer caching is unaffected.** The `--cache-from`/`--cache-to` (Podman) and `--publish`/`--cache-image` (Pack CLI) logic remains in the build scripts. Layer cache refs point directly at Zot and bypass `registries.conf` — this is correct because they're pushing/pulling build cache artifacts, not upstream images.

**Cache probe stays.** The `curl` probe to check if Zot is reachable is still useful for the layer cache path. For mirroring, the `optional: true` ConfigMap mount handles graceful degradation.

**`cache.mirror.enabled` parameter becomes simpler.** Instead of controlling script-level ref rewriting, it could control whether the ConfigMap resource is created in the ClusterWorkflow (using `includeWhen`):

```yaml
resources:
  - id: build-registries-conf
    includeWhen: "${parameters.cache.mirror.enabled}"
    template:
      ...
```

## Migration Path

1. Update Zot deployment — no changes needed (Option A preserves current sync config)
2. Patch ClusterWorkflows — add `build-registries-conf` ConfigMap resource
3. Patch ClusterWorkflowTemplates — add volume + volumeMount for the ConfigMap
4. Simplify build scripts — remove mirror ref rewriting, keep layer cache logic
5. Test — verify `>> Trying to pull` logs show pulls routed through Zot

Steps 2-3 can follow the same script pattern as `install/k3d/registry-cache/patch-build-workflows.sh` and `patch-build-templates.sh`.

## Comparison Summary

| | Current (ref rewriting) | Proposed (registries.conf mount) |
|---|---|---|
| Mirror config location | Duplicated in each template script | Single ConfigMap, mounted into all pods |
| Adding a new upstream registry | Edit every template | Edit one ConfigMap |
| Updating builder/run image versions | Update both upstream and mirror refs | Update upstream ref only |
| Pack CLI lifecycle image | Explicit `pack config lifecycle-image` | Transparent via Podman daemon |
| Graceful degradation | Script probes Zot, falls back | `optional: true` mount, Podman falls back |
| Layer cache | Script logic (unchanged) | Script logic (unchanged) |
