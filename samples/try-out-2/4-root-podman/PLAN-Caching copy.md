# Plan: Layer Caching for Rootless Build Templates

## Overview

Deploy an optional in-cluster cache registry in the workflow plane namespace. When present,
all build templates automatically use registry-based layer caching. When absent, builds work
normally without caching. No new external dependencies — uses the standard `registry:2` image.

---

## 1. Cache Registry Deployment

### 1.1 Helm Chart Addition

Add a cache registry to the **workflow plane Helm chart** (`install/helm/openchoreo-workflow-plane/`).
This is separate from the existing image registry (deployed via `twuni/docker-registry` as `registry`
on port 10082). The cache registry stores only intermediate build cache — not final images.

**New values in `values.yaml`:**

```yaml
cacheRegistry:
  enabled: false
  name: cache-registry
  image: registry:2
  port: 5100
  persistence:
    enabled: true
    size: 10Gi
    storageClass: ""    # use cluster default
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 200m
      memory: 256Mi
```

**New template files:**

| File | Resource | Purpose |
|---|---|---|
| `templates/cache-registry/deployment.yaml` | Deployment | Runs `registry:2` with emptyDir or PVC |
| `templates/cache-registry/service.yaml` | Service (ClusterIP) | Exposes cache registry within cluster |
| `templates/cache-registry/pvc.yaml` | PersistentVolumeClaim | Persistent cache storage (when `persistence.enabled`) |
| `templates/cache-registry/configmap.yaml` | ConfigMap | Registry config (storage limits, GC settings) |

All templates wrapped with `{{- if .Values.cacheRegistry.enabled }}`.

**Service DNS (accessible from any pod in the cluster):**

```
cache-registry.<workflow-plane-namespace>.svc.cluster.local:5100
```

For workflows running in the same namespace (e.g., `argo-build` namespace within the workflow plane),
the short form works:

```
cache-registry.openchoreo-workflow-plane.svc.cluster.local:5100
```

**Registry configuration (ConfigMap):**

```yaml
version: 0.1
log:
  level: warn
storage:
  filesystem:
    rootdirectory: /var/lib/registry
  delete:
    enabled: true
  maintenance:
    uploadpurging:
      enabled: true
      age: 168h       # purge incomplete uploads after 7 days
      interval: 24h
http:
  addr: :5100
```

### 1.2 Makefile / Install Script Changes

Update `make/e2e.mk` and the k3d install flow to deploy the cache registry alongside the
existing image registry when enabled. The cache registry uses the same `twuni/docker-registry`
Helm chart (which wraps `registry:2`) but with different values:

```yaml
# install/k3d/single-cluster/values-cache-registry.yaml
fullnameOverride: cache-registry

persistence:
  enabled: true

service:
  type: ClusterIP
  port: 5100
```

OR define it as a sub-template within the workflow plane chart itself (preferred — keeps
it as a single Helm install).

### 1.3 k3d Registry Mirror Config

The cache registry is **internal only** (ClusterIP). No k3d port mapping needed.
Workflow pods access it via Kubernetes service DNS within the cluster.

For k3d nodes to trust HTTP access (if needed for containerd pulls), add to the k3d
registries config:

```yaml
mirrors:
  "cache-registry.openchoreo-workflow-plane.svc.cluster.local:5100":
    endpoint:
      - http://cache-registry.openchoreo-workflow-plane.svc.cluster.local:5100
```

This is only needed if kubelet itself needs to pull from the cache registry (unlikely —
only build pods access it directly).

---

## 2. Disabling Mechanism

Caching must be optional on **both sides**: the infrastructure (Helm) and the consumers
(workflow templates). The two sides must work independently — templates are `kubectl apply`'d
separately from Helm installs.

### 2.1 Helm Side

```yaml
# values.yaml
cacheRegistry:
  enabled: false   # default: off
```

- `enabled: true` → deploy cache-registry Deployment + Service + PVC
- `enabled: false` → deploy nothing (all templates gated by `{{- if .Values.cacheRegistry.enabled }}`)

### 2.2 Workflow Template Side (Runtime Detection)

Each build template accepts a `cache-registry` input parameter with a default pointing to
the expected in-cluster service:

```yaml
inputs:
  parameters:
    - name: cache-registry
      default: "cache-registry.openchoreo-workflow-plane.svc.cluster.local:5100"
```

The script **probes the registry at startup** before using any cache flags:

```bash
CACHE_REGISTRY="{{inputs.parameters.cache-registry}}"
CACHE_ENABLED="false"

if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
  if wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
    CACHE_ENABLED="true"
    echo ">> Cache registry available at ${CACHE_REGISTRY}"
  else
    echo ">> Cache registry not reachable at ${CACHE_REGISTRY}, building without cache"
  fi
fi
```

This way:
- **Cache registry deployed** → probe succeeds → caching enabled automatically
- **Cache registry not deployed** → probe fails → builds proceed without cache, no error
- **Explicit disable** → set parameter to `"none"` in CI workflow → skips probe entirely
- **Custom cache registry** → override parameter with any registry endpoint

### 2.3 CI Workflow Side

Each CI workflow (ClusterWorkflow) passes `cache-registry` to the build template. The default
matches the in-cluster service. To disable, override with `"none"`:

```yaml
# In the CI workflow runTemplate
arguments:
  parameters:
    - name: cache-registry
      value: "cache-registry.openchoreo-workflow-plane.svc.cluster.local:5100"
```

For clusters without cache registry:

```yaml
    - name: cache-registry
      value: "none"
```

---

## 3. BuildKit Layer Caching (Dockerfile Builds)

### Template: `rootless-buildkit/containerfile-build.yaml`

**Changes to the build script:**

```bash
CACHE_REGISTRY="{{inputs.parameters.cache-registry}}"
CACHE_ENABLED="false"
CACHE_ARGS=""

# Probe cache registry
if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
  if wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
    CACHE_ENABLED="true"
    CACHE_REF="${CACHE_REGISTRY}/${IMAGE_NAME}:buildcache"
    CACHE_ARGS="--import-cache type=registry,ref=${CACHE_REF} --export-cache type=registry,ref=${CACHE_REF},mode=max"
    echo ">> Layer cache: ${CACHE_REF}"
  else
    echo ">> Cache registry not reachable, building without cache"
  fi
fi

buildctl-daemonless.sh build \
  --frontend dockerfile.v0 \
  --local context="$WORKDIR/$DOCKER_CONTEXT" \
  --local dockerfile="$DOCKERFILE_DIR" \
  --opt filename="$DOCKERFILE_NAME" \
  $BUILD_ARG_OPTS \
  $CACHE_ARGS \
  --output "type=docker,name=$IMAGE,dest=/mnt/vol/app-image.tar"
```

**Key details:**
- `mode=max` exports cache for all layers (not just final image layers) — maximizes cache reuse
- `IMAGE_NAME` already includes `namespace-project-component`, so cache is scoped per component
- BuildKit handles TLS/HTTP automatically; in-cluster HTTP works without extra config
- The `BUILDKITD_FLAGS` env var can include `--registry-service-name` if DNS resolution
  needs help, but standard K8s service DNS should work

**Auth:** BuildKit reads `$DOCKER_CONFIG/config.json` for registry auth. For the in-cluster
cache registry with no auth, this just works (anonymous push/pull). If the cache registry
requires auth, mount the `registry-push-secret` in the build container and set
`DOCKER_CONFIG=/etc/secrets/registry-push-secret`.

### Template input changes

```yaml
inputs:
  parameters:
    - name: git-revision
    - name: build-env
    - name: build-args
    - name: cache-registry          # NEW
      default: "cache-registry.openchoreo-workflow-plane.svc.cluster.local:5100"
```

---

## 4. Buildah Layer Caching (Dockerfile Builds)

### Template: `rootless-buildah/containerfile-build.yaml`

Buildah v1.43.0 supports `--cache-from` and `--cache-to` for registry-based layer caching.
Unlike BuildKit's manifest-based cache, Buildah pushes intermediate layer images to the
registry directly — each cached step is a separate image tag.

**Changes to the build script:**

```bash
CACHE_REGISTRY="{{inputs.parameters.cache-registry}}"
CACHE_ENABLED="false"
CACHE_ARGS=""

# Probe cache registry
if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
  if wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
    CACHE_ENABLED="true"
    CACHE_REF="${CACHE_REGISTRY}/${IMAGE_NAME}:buildcache"
    CACHE_ARGS="--layers --cache-from=${CACHE_REF} --cache-to=${CACHE_REF}"
    echo ">> Layer cache: ${CACHE_REF}"
  else
    echo ">> Cache registry not reachable, building without cache"
  fi
fi

buildah build \
  --isolation chroot \
  -t "$IMAGE" \
  -f "$WORKDIR/$DOCKERFILE_PATH" \
  $ENV_ARGS \
  $BUILD_ARG_FLAGS \
  $CACHE_ARGS \
  "$WORKDIR/$DOCKER_CONTEXT"
```

**Key details:**
- `--layers` enables intermediate layer caching (required for `--cache-to` to work)
- `--cache-from` pulls cached layers from the registry on build start
- `--cache-to` pushes new/updated cached layers to the registry after build
- Buildah requires `--tls-verify=false` for HTTP registries:
  `--cache-from=${CACHE_REF} --cache-to=${CACHE_REF} --tls-verify=false` (for cache operations)
  OR configure `/etc/containers/registries.conf` to mark the cache registry as insecure:
  ```ini
  [[registry]]
  location = "cache-registry.openchoreo-workflow-plane.svc.cluster.local:5100"
  insecure = true
  ```

**Auth:** Buildah reads `$REGISTRY_AUTH_FILE` or `${XDG_RUNTIME_DIR}/containers/auth.json`.
For anonymous in-cluster access, no config needed.

### Template input changes

Same as BuildKit — add `cache-registry` input parameter with default.

---

## 5. CNB Buildpack Layer Caching (Podman Rootless)

This section covers the rootless CNB templates that use a Podman sidecar:
- `rootless-cnb/paketo-buildpacks-build.yaml`
- `rootless-cnb/gcp-buildpacks-build.yaml`
- `rootless-cnb/ballerina-buildpack-build.yaml`

### 5.1 Container Layout — Keep `main` + `save-image`

**Constraint:** The OpenChoreo log reader only reads the **main** container's logs. The main
container must contain only the CNB lifecycle build — nothing else. Image saving must happen
in a separate container.

**Structure (unchanged layout, renamed):**

```
sidecars:
  - podman-service              # Podman daemon (rootless) — unchanged
containerSet:
  containers:
    - name: main                # CNB lifecycle creator (build only — logs read from here)
    - name: save-image          # podman save (runs after main completes)
      dependencies: [main]
```

This is the same as the current structure with `build` renamed to `main`. The `main`
container runs only the CNB lifecycle. The `save-image` container runs `podman save`
after `main` completes. The Podman sidecar runs the rootless daemon.

**Why not merge into one container:** The platform reads build logs from the container
named `main`. Mixing build output with `podman save` output would pollute the build log.
Keeping them separate gives clean build logs to the user and isolates the save step.

### 5.2 Add `-cache-image` for Layer Caching

Replace `-cache-dir` (emptyDir, ephemeral) and `-skip-restore` (cache disabled) with
`-cache-image` (registry-backed, persistent).

**Changes to the `main` container script:**

```bash
CACHE_REGISTRY="{{inputs.parameters.cache-registry}}"
CACHE_ARGS="-skip-restore"

# Probe cache registry
if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
  if wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
    CACHE_IMAGE="${CACHE_REGISTRY}/${IMAGE_NAME}:cnb-cache"
    CACHE_ARGS="-cache-image=${CACHE_IMAGE}"
    echo ">> CNB layer cache: ${CACHE_IMAGE}"
  else
    echo ">> Cache registry not reachable, building without cache"
  fi
fi

/cnb/lifecycle/creator \
  -daemon \
  -app="$WORKDIR/$APP_PATH" \
  -run-image="$RUN_IMG" \
  $CACHE_ARGS \
  -platform=/mnt/vol/cnb-platform \
  "$SRC_IMAGE"
```

**`save-image` container** — unchanged, still runs:
```bash
podman --remote --url unix:///podman-run/podman.sock save -o /mnt/vol/app-image.tar "$SRC_IMAGE"
```

**Key details:**
- `-cache-image` and `-daemon` are independent: app image → Podman daemon, cache → registry
- When cache registry is unavailable, falls back to `-skip-restore` (no cache, same as current)
- Removes `-cache-dir=/mnt/vol/cnb-cache` (no longer needed — cache is in registry)
- Can drop the `cnb-cache` emptyDir volume from the template

**Auth for `-cache-image`:**

The CNB lifecycle resolves auth in order:
1. `CNB_REGISTRY_AUTH` env var (JSON: `{"registry:port": "Basic xxx"}`)
2. Docker `config.json` at `$DOCKER_CONFIG` or `~/.docker/config.json`
3. Anonymous (if no credentials match)

For an in-cluster cache registry without auth, anonymous access works. If auth is needed:

```yaml
env:
  - name: DOCKER_HOST
    value: unix:///podman-run/podman.sock
  - name: DOCKER_CONFIG
    value: /etc/secrets/cache-registry-auth
  - name: CNB_PLATFORM_API
    value: "0.14"
```

**Important:** `DOCKER_HOST` (daemon socket for app image export) and `DOCKER_CONFIG`
(registry credentials for cache image) are independent — setting `DOCKER_CONFIG` does not
affect the daemon export.

### 5.3 Main Container Script (Paketo Example)

```bash
set -e

WORKDIR=/mnt/vol/source
APP_PATH="{{workflow.parameters.app-path}}"
BUILD_ENV_JSON='{{workflow.parameters.build-env}}'
GIT_REVISION="{{inputs.parameters.git-revision}}"
CACHE_REGISTRY="{{inputs.parameters.cache-registry}}"

IMAGE_NAME="{{workflow.parameters.image-name}}"
IMAGE_TAG="{{workflow.parameters.image-tag}}"
SRC_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}-${GIT_REVISION}"
RUN_IMG="docker.io/paketobuildpacks/run-jammy-full:0.1.130"

echo ">> [cnb-rootless] Source image: $SRC_IMAGE"
echo ">> App path: $APP_PATH"

# --- Validate app path ---
if [ ! -d "$WORKDIR/$APP_PATH" ]; then
  echo ">> Error: Application path '$APP_PATH' does not exist"
  ls -la "$WORKDIR/"
  exit 1
fi

# --- Wait for Podman sidecar ---
for i in $(seq 1 300); do
  [ -S /podman-run/podman.sock ] && break
  sleep 1
done
if [ ! -S /podman-run/podman.sock ]; then
  echo ">> Error: Podman socket not available"
  exit 1
fi

# --- Setup ---
mkdir -p /mnt/vol/cnb-platform/env
chmod -R a+rwX /mnt/vol/cnb-platform
export HOME=/tmp/cnb-home
mkdir -p "$HOME"
git config --global --add safe.directory "$WORKDIR" 2>/dev/null || true
git config --global --add safe.directory "$WORKDIR/$APP_PATH" 2>/dev/null || true
printf '%s' '-buildvcs=false' > /mnt/vol/cnb-platform/env/GOFLAGS

# --- Build env vars ---
if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
  echo "$BUILD_ENV_JSON" | python3 -c "
import json, sys
for e in json.loads(sys.stdin.read()):
    with open('/mnt/vol/cnb-platform/env/' + e['name'], 'w') as f:
        f.write(e['value'])
" 2>/dev/null || echo ">> Warning: Could not parse build env JSON"
fi

chmod -R a+rX "$WORKDIR/$APP_PATH" 2>/dev/null || true

# --- Determine cache args ---
CACHE_ARGS="-skip-restore"
if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
  if wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
    CACHE_IMAGE="${CACHE_REGISTRY}/${IMAGE_NAME}:cnb-cache"
    CACHE_ARGS="-cache-image=${CACHE_IMAGE}"
    echo ">> CNB layer cache: ${CACHE_IMAGE}"
  else
    echo ">> Cache registry not reachable, building without cache"
  fi
fi

# --- Build ---
echo ">> Building with CNB lifecycle creator"
/cnb/lifecycle/creator \
  -daemon \
  -app="$WORKDIR/$APP_PATH" \
  -run-image="$RUN_IMG" \
  $CACHE_ARGS \
  -platform=/mnt/vol/cnb-platform \
  "$SRC_IMAGE"

echo ">> Build complete"
```

The `save-image` container (separate, runs after `main`):

```bash
set -e

IMAGE_NAME="{{workflow.parameters.image-name}}"
IMAGE_TAG="{{workflow.parameters.image-tag}}"
GIT_REVISION="{{inputs.parameters.git-revision}}"
SRC_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}-${GIT_REVISION}"

echo ">> Saving CNB image as docker-archive tarball"
podman --remote --url unix:///podman-run/podman.sock save -o /mnt/vol/app-image.tar "$SRC_IMAGE"
chmod a+r /mnt/vol/app-image.tar
echo ">> Image saved to /mnt/vol/app-image.tar"
```

### 5.4 Template Input Changes

Each CNB template gets the new `cache-registry` parameter:

```yaml
inputs:
  parameters:
    - name: git-revision
    - name: build-env             # (ballerina only — others use workflow.parameters)
    - name: cache-registry        # NEW
      default: "cache-registry.openchoreo-workflow-plane.svc.cluster.local:5100"
```

### 5.5 Volume Changes

Remove `cnb-cache` emptyDir (no longer needed — cache goes to registry):

```yaml
volumes:
  - name: storage
    emptyDir:
      sizeLimit: 10Gi
  - name: podman-run
    emptyDir: {}
  # REMOVED: cnb-layers (no longer needed)
```

---

## 6. CI Workflow Changes

Each CI workflow (ClusterWorkflow) needs to:
1. Add `cache-registry` as a workflow-level parameter
2. Pass it to the build template

### 6.1 Workflow Parameter

```yaml
spec:
  runTemplate:
    spec:
      arguments:
        parameters:
          # ... existing parameters ...
          - name: cache-registry
            value: "cache-registry.openchoreo-workflow-plane.svc.cluster.local:5100"
```

### 6.2 Pass to Build Template

```yaml
- - name: build-image
    templateRef:
      name: containerfile-build-buildkit   # or buildah, or cnb
      clusterScope: true
      template: build-image
    arguments:
      parameters:
        - name: git-revision
          value: '{{steps.checkout-source.outputs.parameters.git-revision}}'
        - name: cache-registry                                             # NEW
          value: '{{workflow.parameters.cache-registry}}'
```

### 6.3 Affected CI Workflows

| File | Build Template | Cache Type |
|---|---|---|
| `dockerfile-builder-buildkit.yaml` | `containerfile-build-buildkit` | BuildKit registry cache |
| `dockerfile-builder-buildah.yaml` | `containerfile-build-buildah` | Buildah registry cache |
| `paketo-buildpacks-builder-rootless.yaml` | `paketo-buildpacks-build-rootless` | CNB `-cache-image` |
| `gcp-buildpacks-builder-rootless.yaml` | `gcp-buildpacks-build-rootless` | CNB `-cache-image` |
| `ballerina-buildpack-builder-rootless.yaml` | `ballerina-buildpack-build-rootless` | CNB `-cache-image` |

---

## 7. Files Changed

### New Files

| File | Purpose |
|---|---|
| `install/helm/openchoreo-workflow-plane/templates/cache-registry/deployment.yaml` | Cache registry Deployment |
| `install/helm/openchoreo-workflow-plane/templates/cache-registry/service.yaml` | Cache registry Service (ClusterIP) |
| `install/helm/openchoreo-workflow-plane/templates/cache-registry/pvc.yaml` | Cache storage PVC |
| `install/helm/openchoreo-workflow-plane/templates/cache-registry/configmap.yaml` | Registry config |
| `install/k3d/single-cluster/values-cache-registry.yaml` | k3d-specific cache registry values (if using separate chart) |

### Modified Files

| File | Change |
|---|---|
| `install/helm/openchoreo-workflow-plane/values.yaml` | Add `cacheRegistry` section |
| `install/helm/openchoreo-workflow-plane/values.schema.json` | Add `cacheRegistry` schema |
| `make/e2e.mk` | Deploy cache registry when enabled |
| **Workflow Templates:** | |
| `rootless-buildkit/containerfile-build.yaml` | Add `cache-registry` param + `--import-cache`/`--export-cache` |
| `rootless-buildah/containerfile-build.yaml` | Add `cache-registry` param + `--cache-from`/`--cache-to` |
| `rootless-cnb/paketo-buildpacks-build.yaml` | Rename `build` → `main`, add `-cache-image`, remove `-cache-dir`/`-skip-restore` |
| `rootless-cnb/gcp-buildpacks-build.yaml` | Same: rename `build` → `main`, add `-cache-image` |
| `rootless-cnb/ballerina-buildpack-build.yaml` | Same: rename `build` → `main`, add `-cache-image` |
| **CI Workflows:** | |
| `ci-workflows/dockerfile-builder-buildkit.yaml` | Add `cache-registry` param + pass to build |
| `ci-workflows/dockerfile-builder-buildah.yaml` | Add `cache-registry` param + pass to build |
| `ci-workflows/paketo-buildpacks-builder-rootless.yaml` | Add `cache-registry` param + pass to build |
| `ci-workflows/gcp-buildpacks-builder-rootless.yaml` | Add `cache-registry` param + pass to build |
| `ci-workflows/ballerina-buildpack-builder-rootless.yaml` | Add `cache-registry` param + pass to build |

---

## 8. Execution Order

| Step | What | Depends On |
|---|---|---|
| 1 | Add `cacheRegistry` to workflow plane Helm chart (values + templates) | — |
| 2 | Add `values-cache-registry.yaml` for k3d / update `e2e.mk` | Step 1 |
| 3 | Update BuildKit template with cache flags + `cache-registry` param | — |
| 4 | Update Buildah template with cache flags + `cache-registry` param | — |
| 5 | Consolidate CNB templates (single main container) + add `-cache-image` | — |
| 6 | Update all CI workflows to pass `cache-registry` param | Steps 3-5 |
| 7 | Test: deploy with cache registry enabled, run each build type twice | Steps 1-6 |
| 8 | Test: deploy without cache registry, verify graceful fallback | Steps 1-6 |

Steps 3, 4, 5 can proceed in parallel. Step 6 depends on all template changes.

---

## 9. Validation Checklist

- [ ] Cache registry deploys when `cacheRegistry.enabled: true`
- [ ] Cache registry does NOT deploy when `cacheRegistry.enabled: false`
- [ ] BuildKit: second build is faster (cache hit on unchanged layers)
- [ ] BuildKit: builds work when cache registry is absent
- [ ] Buildah: second build is faster (cache hit on unchanged layers)
- [ ] Buildah: builds work when cache registry is absent
- [ ] Buildah: `--cache-from`/`--cache-to` work with `--tls-verify=false` on HTTP registry
- [ ] CNB (Paketo): second build is faster (cache hit on buildpack layers)
- [ ] CNB (GCP): second build is faster
- [ ] CNB (Ballerina): second build is faster
- [ ] CNB: builds work when cache registry is absent (falls back to `-skip-restore`)
- [ ] CNB: `main` container runs only lifecycle (logs are clean for log reader)
- [ ] CNB: `save-image` container runs after `main` and produces tarball
- [ ] Cache data persists across workflow plane pod restarts (PVC)
- [ ] No auth issues with anonymous push/pull to in-cluster cache registry

---

## 10. Open Questions

1. **Cache garbage collection:** The cache registry will accumulate stale cache images over time.
   Should we configure automatic GC (registry:2 supports it) or rely on PVC size limits?
   Recommendation: Configure GC in the registry ConfigMap with a TTL-based policy.

2. **Cache scoping by branch:** For Dockerfile builds, cache from a feature branch can evict
   main branch cache. BuildKit supports multi-ref import (`--import-cache` from main + branch,
   `--export-cache` to branch only). Should we implement branch-scoped caching?
   Recommendation: Start with single cache ref per component. Add branch scoping later if
   cache thrashing becomes a problem in practice.

3. **Cache registry auth:** The plan assumes anonymous push/pull within the cluster. If the
   cluster enforces network policies or registry auth, the `registry-push-secret` needs to be
   mounted in build steps too. Should we mount it by default?
   Recommendation: Start with anonymous access. Add secret mounting as an option if needed.

4. **`wget`/`curl` in builder images:** The `main` container uses `wget` to probe the cache
   registry at startup. Need to verify `wget` or `curl` is available in all 3 builder images
   (Paketo, GCP, Ballerina). If missing, the probe can be skipped and caching assumed available
   (will fail gracefully on first cache pull if registry is absent).
