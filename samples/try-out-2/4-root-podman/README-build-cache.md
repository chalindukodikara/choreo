# Build Mirror and Layer Caching with Zot

Deploy a cluster-internal [Zot](https://zotregistry.dev/) OCI registry in the workflow plane and use it for two independent cache paths:

- **Mirror cache**: pull-through cache for upstream builder, run, lifecycle, and Dockerfile base images. Implemented as a mounted `registries.conf` ConfigMap — Podman handles mirroring transparently for all image pulls, including those delegated by Pack CLI via `--docker-host inherit`.
- **Layer cache**: registry-backed build layer cache for Podman Dockerfile builds and Pack CLI buildpack builds.

Component authors control caching with:

```yaml
parameters:
  cache:
    mirror:
      enabled: true
    layers:
      mode: reuse # disabled | reuse | rebuild
```

Layer modes:

| Mode | Read layer cache | Write layer cache | Use case |
|---|---:|---:|---|
| `disabled` | no | no | Fully uncached build |
| `reuse` | yes | yes | Normal fast build |
| `rebuild` | no | yes | Clean build that refreshes the cache for later builds |

Mirror caching and layer caching are intentionally independent. You can disable the mirror while still using layer cache, or disable layer cache while still pulling upstream images through Zot.

### How mirroring works

```
ClusterWorkflow creates registries-conf ConfigMap per WorkflowRun (when mirror.enabled)
  → ClusterWorkflowTemplate mounts it at /etc/containers/registries.conf.d/mirrors.conf
    → Podman reads it on every pull
      → Pack CLI uses --docker-host inherit → Podman → mirror applies
      → podman build → mirror applies to FROM images
```

No explicit ref rewriting needed. Builder, run, lifecycle, and Dockerfile base images all route through the mirror automatically. Podman tries the mirror first and falls back to upstream if the mirror is unavailable. When the ConfigMap is absent (mirror disabled), Podman pulls directly from upstream — no script changes needed.

---

## Prerequisites

- A running OpenChoreo cluster with the workflow plane installed
- `kubectl` configured to access the cluster
- [yq v4](https://github.com/mikefarah/yq) for the patch commands

---

## Step 1: Deploy Zot

Install Zot via its official Helm chart using the OpenChoreo build-cache values file. This creates a `build-cache` Service on port `5100`, a 10Gi PVC, and the Zot configuration in the `openchoreo-workflow-plane` namespace.

### 1.1 Add the Zot Helm repository

```bash
helm repo add project-zot http://zotregistry.dev/helm-charts/
helm repo update project-zot
```

### 1.2 Install the registry

Install Zot with the OpenChoreo build-cache values file. It uses the full Zot image (not `zot-minimal`, because mirroring relies on Zot's `sync` extension), enables `docker2s2` compatibility for Pack CLI `--publish`, and configures on-demand mirroring with retention-based garbage collection. The chart deploys a single-replica StatefulSet backed by a 10Gi `ReadWriteOnce` PVC.

```bash
helm install build-cache project-zot/zot \
  -n openchoreo-workflow-plane --create-namespace \
  -f https://raw.githubusercontent.com/openchoreo/openchoreo/refs/heads/main/install/k3d/common/values-zot.yaml
```

The values file lives at [`install/k3d/common/values-zot.yaml`](../../../install/k3d/common/values-zot.yaml). To customize the image tag, storage size, or storage class, download it, edit your copy, and pass it with `-f`.

The `docker2s2` compatibility mode is required when Pack CLI uses `--publish`. Pack can export Docker Manifest v2 Schema 2, and Zot rejects that media type unless this compatibility mode is enabled.

For Docker Hub, the values configure only `onDemand: true`; polled mirroring is not used because Docker Hub is rate-limited and does not support catalog listing.

### 1.3 Verify Zot

```bash
kubectl -n openchoreo-workflow-plane rollout status statefulset/build-cache

kubectl -n openchoreo-workflow-plane port-forward svc/build-cache 5100:5100 &
sleep 2
curl -v http://localhost:5100/v2/
kill %1
```

You should see `HTTP/1.1 200 OK` and `Docker-Distribution-Api-Version: registry/2.0`.

---

## Step 2: Patch Workflow Templates

Patch the in-cluster `ClusterWorkflowTemplate` resources so they:

1. Accept `build-cache` and `cache-layers-mode` input parameters for layer caching
2. Mount an optional `registries-conf` ConfigMap for transparent mirroring

Mirroring is handled entirely by the mounted `registries.conf` — build scripts only contain layer cache logic.

### 2.1 Apply the updated ClusterWorkflowTemplates

Apply the four patched `ClusterWorkflowTemplate` resources from [`install/k3d/workflow-templates/`](../../../install/k3d/workflow-templates/). They add the `build-cache` and `cache-layers-mode` input parameters and the optional `registries-conf` mirror mount; see [2.2](#22-what-changed-from-the-original-templates) for the full diff.

```bash
BASE=https://raw.githubusercontent.com/openchoreo/openchoreo/refs/heads/main/install/k3d/workflow-templates

for tmpl in containerfile-build paketo-buildpacks-build gcp-buildpacks-build ballerina-buildpack-build; do
  kubectl apply -f "$BASE/$tmpl.yaml"
done
```

### 2.2 What changed from the original templates

Each template adds:

1. **`registries-conf` volume** — an optional ConfigMap named after the WorkflowRun. When present, Podman transparently routes upstream image pulls through Zot. When absent, Podman pulls directly from upstream.

```yaml
volumes:
  - name: registries-conf
    configMap:
      name: "{{workflow.parameters.workflowrun-name}}-registries-conf"
      optional: true
```

2. **`registries-conf` volumeMount** — mounted as a drop-in file in `registries.conf.d/` to avoid replacing any existing container configuration.

```yaml
volumeMounts:
  - mountPath: /etc/containers/registries.conf.d/mirrors.conf
    subPath: mirrors.conf
    name: registries-conf
    readOnly: true
```

3. **Simplified input parameters** — only `build-cache` and `cache-layers-mode`. The `cache-mirror-enabled` parameter is no longer needed because mirroring is controlled by ConfigMap presence.

4. **No mirror ref rewriting in scripts** — builder, run, and lifecycle image refs stay as upstream refs. No `pack config lifecycle-image` workaround needed. Build scripts only handle layer cache logic.

### 2.3 Cache probe and layer cache

The cache probe checks if Zot is reachable and registers it as an insecure registry for layer cache push/pull:

```bash
CACHE_AVAILABLE="false"
if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
  if curl -sf --max-time 3 -o /dev/null "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
    CACHE_AVAILABLE="true"
    mkdir -p /etc/containers/registries.conf.d
    cat > /etc/containers/registries.conf.d/build-cache.conf <<REGEOF
[[registry]]
location = "${CACHE_REGISTRY}"
insecure = true
REGEOF
  fi
fi
```

Podman layer cache uses `--cache-from`/`--cache-to` with a `--cache-ttl`:

```bash
CACHE_ARGS="--layers --cache-from=${CACHE_REF} --cache-to=${CACHE_REF} --cache-ttl=168h"
```

Pack CLI layer cache uses `--publish` with `--cache-image`. When using `--publish`, the built image is pushed to Zot, then pulled back and saved as a tar:

```bash
pack build "$PUBLISH_REF" \
  --builder "$BUILDER" \
  --run-image "$RUN_IMG" \
  --publish \
  --cache-image "$CACHE_IMAGE" \
  ...

podman pull --tls-verify=false "$PUBLISH_REF"
podman tag "$PUBLISH_REF" "$IMAGE"
podman save -o /mnt/vol/app-image.tar "$IMAGE"
```

### 2.4 Template patch checklist

For each `ClusterWorkflowTemplate`:

1. Add input parameters:

```yaml
- name: build-cache
  default: "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
- name: cache-layers-mode
  default: "reuse"
```

2. Add `registries-conf` volume (`optional: true`) and volumeMount (`registries.conf.d/mirrors.conf`).
3. Remove mirror ref rewriting from the build script.
4. Keep the cache probe, insecure registry entry, and layer cache logic.

---

## Step 3: Patch CI Workflows

Patch the existing `ClusterWorkflow` resources so developers configure cache with a structured parameter. The workflow creates a `registries-conf` ConfigMap per WorkflowRun when mirroring is enabled, and passes layer cache parameters to the build templates.

### 3.1 Patch all four CI workflows

```bash
CACHE_DESC="Build cache configuration. mirror.enabled controls upstream image pull-through caching via a mounted registries.conf. layers.mode controls layer cache behavior: disabled, reuse, or rebuild."

REGISTRIES_CONF='[[registry]]
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
insecure = true

[[registry]]
location = "docker.io"
[[registry.mirror]]
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/docker.io"
insecure = true

[[registry]]
location = "gcr.io"
[[registry.mirror]]
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/gcr.io"
insecure = true

[[registry]]
location = "ghcr.io"
[[registry.mirror]]
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/ghcr.io"
insecure = true

[[registry]]
location = "quay.io"
[[registry.mirror]]
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/quay.io"
insecure = true'

for workflow in dockerfile-builder paketo-buildpacks-builder gcp-buildpacks-builder ballerina-buildpack-builder; do
  kubectl get clusterworkflow "$workflow" -o yaml | \
    yq '
      del(
        .metadata.creationTimestamp,
        .metadata.generation,
        .metadata.managedFields,
        .metadata.resourceVersion,
        .metadata.uid,
        .status
      ) |

      .spec.parameters.openAPIV3Schema.properties.cache = {
        "type": "object",
        "default": {
          "mirror": {"enabled": true},
          "layers": {"mode": "reuse"}
        },
        "description": "'"$CACHE_DESC"'",
        "properties": {
          "mirror": {
            "type": "object",
            "default": {"enabled": true},
            "properties": {
              "enabled": {
                "type": "boolean",
                "default": true,
                "description": "Pull upstream builder, run, lifecycle, and base images through the workflow-plane cache registry when available"
              }
            }
          },
          "layers": {
            "type": "object",
            "default": {"mode": "reuse"},
            "properties": {
              "mode": {
                "type": "string",
                "default": "reuse",
                "enum": ["disabled", "reuse", "rebuild"],
                "description": "Layer cache mode: disabled skips cache, reuse reads and writes cache, rebuild ignores existing cache and writes a fresh cache"
              }
            }
          }
        }
      } |

      .spec.runTemplate.spec.arguments.parameters |=
        [.[] | select(.name != "build-cache" and .name != "cache-layers-mode")] +
        [
          {"name": "build-cache", "value": "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"},
          {"name": "cache-layers-mode", "value": "${parameters.cache.layers.mode}"}
        ] |

      .spec.runTemplate.spec.templates[0].steps[1][0].arguments.parameters |=
        [.[] | select(.name != "build-cache" and .name != "cache-layers-mode")] +
        [
          {"name": "build-cache", "value": "{{workflow.parameters.build-cache}}"},
          {"name": "cache-layers-mode", "value": "{{workflow.parameters.cache-layers-mode}}"}
        ] |

      .spec.resources = (.spec.resources // []) |
      del(.spec.resources[] | select(.id == "build-registries-conf")) |
      .spec.resources += [
        {
          "id": "build-registries-conf",
          "includeWhen": "${parameters.cache.mirror.enabled}",
          "template": {
            "apiVersion": "v1",
            "kind": "ConfigMap",
            "metadata": {
              "name": "${metadata.workflowRunName}-registries-conf",
              "namespace": "${metadata.namespace}"
            },
            "data": {
              "mirrors.conf": ""
            }
          }
        }
      ]
    ' | \
    REGISTRIES_CONF="$REGISTRIES_CONF" yq '
      (.spec.resources[] | select(.id == "build-registries-conf")).template.data."mirrors.conf" = strenv(REGISTRIES_CONF) |
      (.spec.resources[] | select(.id == "build-registries-conf")).template.data."mirrors.conf" style="literal"
    ' | kubectl apply -f -
  echo "Patched $workflow"
done
```

The `build-registries-conf` resource uses `includeWhen: ${parameters.cache.mirror.enabled}` so the ConfigMap is only created when mirroring is enabled. When it's not created, the `optional: true` volume mount in the ClusterWorkflowTemplate means the build pod starts normally — Podman just pulls from upstream.

### 3.2 Verify the workflow schema

```bash
for workflow in dockerfile-builder paketo-buildpacks-builder gcp-buildpacks-builder ballerina-buildpack-builder; do
  echo "=== $workflow ==="
  kubectl get clusterworkflow "$workflow" -o yaml | yq '.spec.parameters.openAPIV3Schema.properties.cache'
  echo ""
done
```

Each workflow should show `cache.mirror.enabled` and `cache.layers.mode` with enum values:

```yaml
enum:
  - disabled
  - reuse
  - rebuild
```

### 3.3 Verify the registries-conf resource

```bash
for workflow in dockerfile-builder paketo-buildpacks-builder gcp-buildpacks-builder ballerina-buildpack-builder; do
  echo "=== $workflow ==="
  kubectl get clusterworkflow "$workflow" -o yaml | yq '.spec.resources[] | select(.id == "build-registries-conf")'
  echo ""
done
```

Each workflow should show the `build-registries-conf` resource with `includeWhen` and the `mirrors.conf` data.

---

## Step 4: Configure a Component

Default cache behavior:

```yaml
parameters:
  cache:
    mirror:
      enabled: true
    layers:
      mode: reuse
```

Clean build that refreshes the layer cache:

```yaml
parameters:
  cache:
    mirror:
      enabled: true
    layers:
      mode: rebuild
```

Fully uncached build:

```yaml
parameters:
  cache:
    mirror:
      enabled: false
    layers:
      mode: disabled
```

Patch an existing component:

```bash
kubectl patch component my-service -n default --type merge -p '{"spec":{"parameters":{"cache":{"mirror":{"enabled":true},"layers":{"mode":"rebuild"}}}}}'
```

---

## Step 5: Verify Caching

### 5.1 Check registry health and catalog

```bash
kubectl -n openchoreo-workflow-plane port-forward svc/build-cache 5100:5100 &
sleep 2
curl -s http://localhost:5100/v2/_catalog
kill %1
```

After cached builds complete, the catalog should include repositories like:

```json
{
  "repositories": [
    "build-cache/containerfile/default-myproject-myservice",
    "build-cache/cnb/default-myproject-myservice",
    "mirror/docker.io/paketobuildpacks/builder-jammy-full",
    "mirror/ghcr.io/openchoreo/buildpack/ballerina"
  ]
}
```

### 5.2 Check build logs

Mirror enabled (ConfigMap mounted):

```text
>> Upstream image mirror: mounted via registries.conf
```

Podman `Trying to pull` lines should show the Zot mirror URL first. If Zot has the image (or fetches it on-demand), the pull succeeds from the mirror. If not, Podman falls back to the upstream registry automatically.

Layer cache reuse:

```text
>> Layer cache: reuse build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/build-cache/containerfile/<component>
```

Layer cache rebuild:

```text
>> Layer cache: rebuild build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/build-cache/containerfile/<component>
```

Registry unavailable:

```text
>> Cache registry not reachable, building without layer cache
```

Mirror disabled (no ConfigMap, no mirror log line) — Podman `Trying to pull` lines show upstream registry URLs directly.

---

## Disable / Uninstall

### Disable cache for one component

```yaml
parameters:
  cache:
    mirror:
      enabled: false
    layers:
      mode: disabled
```

### Remove Zot

```bash
helm uninstall build-cache -n openchoreo-workflow-plane
# Helm leaves the StatefulSet PVC behind by design; delete it to reclaim storage:
kubectl delete pvc build-cache-pvc-build-cache-0 -n openchoreo-workflow-plane
```

Builds continue to work normally. The `optional: true` ConfigMap mount means build pods start without the mirror config, and the cache probe falls back when Zot is unreachable.

### Restore original workflow templates and CI workflows

If the live objects were previously patched from `kubectl get -o yaml`, their
`kubectl.kubernetes.io/last-applied-configuration` annotations might contain
server-owned metadata such as `resourceVersion`. Remove those annotations before
restoring with client-side apply:

```bash
kubectl annotate clusterworkflowtemplate \
  containerfile-build \
  paketo-buildpacks-build \
  gcp-buildpacks-build \
  ballerina-buildpack-build \
  kubectl.kubernetes.io/last-applied-configuration-

kubectl annotate clusterworkflow \
  dockerfile-builder \
  paketo-buildpacks-builder \
  gcp-buildpacks-builder \
  ballerina-buildpack-builder \
  kubectl.kubernetes.io/last-applied-configuration-
```

Then apply the original manifests:

```bash
kubectl apply -f samples/getting-started/workflow-templates/containerfile-build.yaml
kubectl apply -f samples/getting-started/workflow-templates/paketo-buildpacks-build.yaml
kubectl apply -f samples/getting-started/workflow-templates/gcp-buildpacks-build.yaml
kubectl apply -f samples/getting-started/workflow-templates/ballerina-buildpack-build.yaml

kubectl apply -f samples/getting-started/ci-workflows/dockerfile-builder.yaml
kubectl apply -f samples/getting-started/ci-workflows/paketo-buildpacks-builder.yaml
kubectl apply -f samples/getting-started/ci-workflows/gcp-buildpacks-builder.yaml
kubectl apply -f samples/getting-started/ci-workflows/ballerina-buildpack-builder.yaml
```

---

## How It Works

### Parameter flow

```text
Developer sets cache.mirror.enabled and cache.layers.mode
  -> ClusterWorkflow schema validates cache.layers.mode enum
    -> mirror.enabled=true: ClusterWorkflow creates registries-conf ConfigMap via includeWhen
    -> runTemplate maps cache.layers.mode to Argo parameter
      -> build step passes build-cache and cache-layers-mode to ClusterWorkflowTemplate
        -> template mounts registries-conf ConfigMap (optional: true)
          -> Podman reads registries.conf.d/mirrors.conf on every pull
            -> All image pulls (FROM, builder, run, lifecycle) try Zot first, fall back to upstream
          -> template probes Zot for layer cache
            -> layers.mode reuse: read and write layer cache
            -> layers.mode rebuild: skip read, write fresh cache
```

### Cache reference paths

| Cache type | Reference pattern |
|---|---|
| Containerfile layer cache | `build-cache/containerfile/<namespace-project-component>` |
| CNB layer cache | `build-cache/cnb/<namespace-project-component>:cnb-cache` |
| CNB handoff image (temporary) | `build-tmp/cnb/<namespace-project-component>:<image-tag>-<git-revision>` |
| Docker Hub mirror | `mirror/docker.io/<repo>` |
| GCR mirror | `mirror/gcr.io/<repo>` |
| GHCR mirror | `mirror/ghcr.io/<repo>` |
| Quay mirror | `mirror/quay.io/<repo>` |

### Cache mechanisms

| Builder | `reuse` | `rebuild` |
|---|---|---|
| Podman Dockerfile | `--layers --cache-from=<ref> --cache-to=<ref>` | `--layers --cache-to=<ref>` |
| Pack CLI Buildpacks | `--publish --cache-image=<ref>` | `--publish --cache-image=<ref> --clear-cache` |

For Podman, the cache ref is intentionally an untagged repository. Podman writes content-addressed cache tags under that repository and rejects a tagged `--cache-to` reference such as `:buildcache`.

For Pack CLI, registry cache requires `--publish`. The template publishes the built image to Zot, pulls it back into Podman with `--tls-verify=false`, tags it as the expected workflow image, and saves `/mnt/vol/app-image.tar` for the rest of the pipeline.

Zot handles cleanup through online garbage collection and retention policies. GC runs without a separate CronJob.

### Mirroring mechanism

Mirroring uses a mounted `registries.conf` ConfigMap with `[[registry.mirror]]` entries. Podman tries the Zot mirror first and falls back to upstream if the mirror is unavailable or doesn't have the image.

```toml
[[registry]]
location = "docker.io"
[[registry.mirror]]
location = "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror/docker.io"
insecure = true
```

Zot's sync config uses `onDemand: true` with a catch-all `"prefix": "/**"` content filter per registry. Any image from docker.io, gcr.io, ghcr.io, or quay.io is fetched and cached on first pull. Images are stored under `mirror/<registry>/` paths in Zot.

Graceful degradation:
- Mirror disabled (`cache.mirror.enabled: false`): ConfigMap is not created, `optional: true` volume mount is a no-op, Podman pulls from upstream
- Zot unreachable: Podman tries the mirror, gets a connection error, falls back to upstream automatically
- Image not in Zot cache: Zot fetches it on-demand from upstream, caches it, and serves it
