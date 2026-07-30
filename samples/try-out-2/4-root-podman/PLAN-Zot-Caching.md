# Plan: Zot-Based Build Cache Registry for OpenChoreo

## Overview

Deploy a Zot-based OCI cache registry alongside OpenChoreo's workflow plane. Build templates
push/pull layer cache to this registry, speeding up repeat builds. Zot's built-in retention
policies and online GC handle cleanup automatically — no CronJob required.

The cache registry is **not** part of the OpenChoreo Helm charts. It lives in
`chalindukodikara/openchoreo-build-cache` as a standalone Helm chart + updated workflow
templates that reference it.

---

## Repository Structure

**GitHub:** `chalindukodikara/openchoreo-build-cache`

```
openchoreo-build-cache/
├── README.md
├── .github/
│   └── workflows/
│       └── release-helm-chart.yaml       # CI: package + push Helm chart to GHCR on tag
├── helm/
│   └── build-cache/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── values-k3d.yaml              # k3d overrides (smaller PVC, shorter TTL)
│       ├── values-production.yaml        # production overrides
│       └── templates/
│           ├── deployment.yaml
│           ├── service.yaml
│           ├── pvc.yaml
│           ├── configmap.yaml
│           └── _helpers.tpl
├── workflow-templates/                   # ClusterWorkflowTemplates (build steps)
│   ├── containerfile-build.yaml          # updated: adds build-cache + no-cache params + podman cache flags
│   ├── paketo-buildpacks-build.yaml      # updated: adds build-cache + no-cache params + pack cache flags
│   ├── gcp-buildpacks-build.yaml         # updated: adds build-cache + no-cache params + pack cache flags
│   └── ballerina-buildpack-build.yaml    # updated: adds build-cache + no-cache params + pack cache flags
└── ci-workflows/                         # ClusterWorkflows (developer-facing, exposes noCache param)
    ├── dockerfile-builder.yaml           # updated: adds noCache schema property + wires to build step
    ├── paketo-buildpacks-builder.yaml    # updated: adds noCache schema property + wires to build step
    ├── gcp-buildpacks-builder.yaml       # updated: adds noCache schema property + wires to build step
    └── ballerina-buildpack-builder.yaml  # updated: adds noCache schema property + wires to build step
```

The `ci-workflows/` files are copies of the originals from
`openchoreo/samples/getting-started/ci-workflows/` with the `noCache` parameter added.
Apply these instead of the originals to enable the cache feature.

---

## 1. Zot Cache Registry — Helm Chart

### 1.1 Chart.yaml

```yaml
apiVersion: v2
name: openchoreo-build-cache
description: Zot-based OCI cache registry for OpenChoreo build layer caching
version: 0.1.0
appVersion: "2.1.3"
```

### 1.2 values.yaml (defaults)

```yaml
replicaCount: 1

image:
  repository: ghcr.io/project-zot/zot-minimal-linux-amd64
  tag: v2.1.3
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 5100

persistence:
  enabled: true
  size: 10Gi
  storageClass: ""          # empty = cluster default

resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 200m
    memory: 256Mi

retention:
  buildCacheTTL: "168h"       # 7 days
  buildCacheKeepCount: 3
  catchAllTTL: "72h"          # 3 days
  catchAllKeepCount: 1

gc:
  enabled: true
  delay: "2h"
  interval: "6h"
```

### 1.3 values-k3d.yaml

```yaml
persistence:
  enabled: true
  size: 5Gi

retention:
  buildCacheTTL: "48h"
  buildCacheKeepCount: 1
  catchAllTTL: "24h"
  catchAllKeepCount: 1
```

### 1.4 values-production.yaml

```yaml
persistence:
  enabled: true
  size: 50Gi
  storageClass: ""            # set per cluster (gp3, pd-ssd, etc.)

retention:
  buildCacheTTL: "336h"       # 14 days
  buildCacheKeepCount: 5
  catchAllTTL: "168h"
  catchAllKeepCount: 3

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

### 1.5 ConfigMap — Zot Configuration

```json
{
  "storage": {
    "rootDirectory": "/var/lib/registry",
    "gc": {{ .Values.gc.enabled }},
    "gcDelay": "{{ .Values.gc.delay }}",
    "gcInterval": "{{ .Values.gc.interval }}",
    "retention": {
      "policies": [
        {
          "repositories": ["build-cache/buildkit/**"],
          "keepTags": {
            "pushedWithin": "{{ .Values.retention.buildCacheTTL }}",
            "pulledWithin": "{{ .Values.retention.buildCacheTTL }}",
            "mostRecentlyPushedCount": {{ .Values.retention.buildCacheKeepCount }},
            "mostRecentlyPulledCount": {{ .Values.retention.buildCacheKeepCount }}
          }
        },
        {
          "repositories": ["build-cache/cnb/**"],
          "keepTags": {
            "pushedWithin": "{{ .Values.retention.buildCacheTTL }}",
            "pulledWithin": "{{ .Values.retention.buildCacheTTL }}",
            "mostRecentlyPushedCount": {{ .Values.retention.buildCacheKeepCount }},
            "mostRecentlyPulledCount": {{ .Values.retention.buildCacheKeepCount }}
          }
        },
        {
          "repositories": ["build-cache/podman/**"],
          "keepTags": {
            "pushedWithin": "{{ .Values.retention.buildCacheTTL }}",
            "pulledWithin": "{{ .Values.retention.buildCacheTTL }}",
            "mostRecentlyPushedCount": {{ .Values.retention.buildCacheKeepCount }},
            "mostRecentlyPulledCount": {{ .Values.retention.buildCacheKeepCount }}
          }
        },
        {
          "repositories": ["**"],
          "keepTags": {
            "pushedWithin": "{{ .Values.retention.catchAllTTL }}",
            "pulledWithin": "{{ .Values.retention.catchAllTTL }}",
            "mostRecentlyPulledCount": {{ .Values.retention.catchAllKeepCount }}
          }
        }
      ]
    }
  },
  "http": {
    "address": "0.0.0.0",
    "port": "5100"
  },
  "log": {
    "level": "warn"
  }
}
```

**Retention logic** (all conditions are OR — tag kept if *any* match):

| Condition | Purpose |
|---|---|
| `pushedWithin` | Keep recently written cache |
| `pulledWithin` | Keep heavily-used cache even if pushed long ago (e.g., shared base layer cache) |
| `mostRecentlyPushedCount` | Floor — always keep N newest, protects against burst builds all expiring at once |
| `mostRecentlyPulledCount` | Floor — always keep N most accessed, protects hot cache from TTL eviction |

A tag is deleted only when **all four conditions are false**.

### 1.6 Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "build-cache.fullname" . }}
  namespace: {{ .Release.Namespace }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ include "build-cache.fullname" . }}
  template:
    metadata:
      labels:
        app: {{ include "build-cache.fullname" . }}
    spec:
      containers:
      - name: zot
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        imagePullPolicy: {{ .Values.image.pullPolicy }}
        ports:
        - containerPort: 5100
          name: http
        args: ["/etc/zot/config.json"]
        volumeMounts:
        - name: data
          mountPath: /var/lib/registry
        - name: config
          mountPath: /etc/zot/config.json
          subPath: config.json
          readOnly: true
        resources:
          {{- toYaml .Values.resources | nindent 10 }}
        readinessProbe:
          httpGet:
            path: /v2/
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /v2/
            port: http
          initialDelaySeconds: 10
          periodSeconds: 30
      volumes:
      - name: config
        configMap:
          name: {{ include "build-cache.fullname" . }}-config
      - name: data
        {{- if .Values.persistence.enabled }}
        persistentVolumeClaim:
          claimName: {{ include "build-cache.fullname" . }}-data
        {{- else }}
        emptyDir:
          sizeLimit: {{ .Values.persistence.size }}
        {{- end }}
```

### 1.7 Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ include "build-cache.fullname" . }}
  namespace: {{ .Release.Namespace }}
spec:
  type: {{ .Values.service.type }}
  ports:
  - port: {{ .Values.service.port }}
    targetPort: http
    protocol: TCP
    name: http
  selector:
    app: {{ include "build-cache.fullname" . }}
```

### 1.8 PVC

```yaml
{{- if .Values.persistence.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ include "build-cache.fullname" . }}-data
  namespace: {{ .Release.Namespace }}
spec:
  accessModes: [ReadWriteOnce]
  {{- if .Values.persistence.storageClass }}
  storageClassName: {{ .Values.persistence.storageClass }}
  {{- end }}
  resources:
    requests:
      storage: {{ .Values.persistence.size }}
{{- end }}
```

---

## 2. Service DNS

The cache registry is accessible within the cluster at:

```
build-cache.<namespace>.svc.cluster.local:5100
```

Default namespace is `openchoreo-workflow-plane` (where Argo runs builds):

```
build-cache.openchoreo-workflow-plane.svc.cluster.local:5100
```

The registry uses HTTP (no TLS) since it's cluster-internal only.

---

## 3. Workflow Template Changes

Each build template gets two cache-related input parameters:
- `build-cache` — the cache registry endpoint (default: in-cluster Zot service)
- `no-cache` — boolean flag (`"true"` / `"false"`) to skip cache for a single build

The script probes the registry at startup — if reachable and `no-cache` is not set,
caching is enabled; otherwise builds proceed without cache.

### 3.1 Cache Registry Probe (shared pattern)

```bash
CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
NO_CACHE="{{inputs.parameters.no-cache}}"
CACHE_ENABLED="false"

if [ "$NO_CACHE" = "true" ]; then
  echo ">> Cache disabled (no-cache=true)"
elif [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
  if wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
    CACHE_ENABLED="true"
    echo ">> Cache registry available at ${CACHE_REGISTRY}"
  else
    echo ">> Cache registry not reachable, building without cache"
  fi
fi
```

**Disabling cache:**
- **One-off skip**: Set `no-cache: "true"` when triggering the build. Cache stays intact for the next build.
- **Permanent disable**: Set `build-cache` to `"none"` in the CI workflow.

### 3.2 Why Skip Cache?

A user may need to trigger a cache-free build when:

| Reason | Example |
|---|---|
| **Corrupted cache** | A bad layer got cached, builds keep failing with cryptic errors |
| **Stale security patches** | `RUN apt-get update && apt-get install` is cached — unpatched CVEs persist even though upstream packages were updated |
| **Dependency drift** | `RUN npm install` cached from days ago, but a transitive dependency was yanked or updated (lockfile unchanged) |
| **Base image update** | Dockerfile uses `FROM node:20` but cached layers reference an older digest of that tag |
| **Buildpack version update** | Builder/run image was updated but CNB cache still holds old buildpack layers |
| **Build debugging** | Need full uncached output to diagnose where a step actually fails vs. silently passes from cache |
| **Pre-release verification** | Confirm the build reproduces from scratch before cutting a release |

### 3.3 containerfile-build.yaml — Podman Layer Cache

Podman supports `--cache-from` and `--cache-to` for registry-based layer caching.

**New input parameters:**

```yaml
inputs:
  parameters:
    - name: git-revision
    - name: build-env
    - name: build-args
    - name: build-cache
      default: "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
    - name: no-cache
      default: "false"
```

**Script additions** (after storage config, before `podman build`):

```bash
CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
NO_CACHE="{{inputs.parameters.no-cache}}"
IMAGE_NAME="{{workflow.parameters.image-name}}"
CACHE_ARGS=""

if [ "$NO_CACHE" = "true" ]; then
  CACHE_ARGS="--no-cache"
  echo ">> Cache disabled (no-cache=true), building from scratch"
elif [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
  if wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
    CACHE_REF="${CACHE_REGISTRY}/build-cache/podman/${IMAGE_NAME}:buildcache"
    CACHE_ARGS="--layers --cache-from=${CACHE_REF} --cache-to=${CACHE_REF} --cache-ttl=168h"

    # Configure insecure registry for HTTP access
    mkdir -p /etc/containers/registries.conf.d
    cat > /etc/containers/registries.conf.d/cache.conf <<REGEOF
[[registry]]
location = "${CACHE_REGISTRY}"
insecure = true
REGEOF

    echo ">> Layer cache: ${CACHE_REF}"
  else
    echo ">> Cache registry not reachable, building without cache"
  fi
fi

podman build -t $IMAGE -f $WORKDIR/$DOCKERFILE_PATH $ENV_ARGS $BUILD_ARG_ARGS $CACHE_ARGS $WORKDIR/$DOCKER_CONTEXT
```

When `no-cache=true`: passes `--no-cache` to `podman build` (ignores all cached layers,
rebuilds every step). Does **not** pass `--cache-to` — the old cache stays in the registry
untouched. The next normal build will still benefit from whatever valid cache existed before.

**Cache ref pattern:** `<build-cache>/build-cache/podman/<namespace-project-component>:buildcache`

### 3.4 Buildpack Templates — Pack CLI Cache

Pack CLI supports `--cache-image` for registry-based buildpack cache and `--clear-cache`
to ignore existing cache.

**New input parameters** (same for paketo, gcp, ballerina):

```yaml
inputs:
  parameters:
    - name: git-revision
    - name: build-cache
      default: "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
    - name: no-cache
      default: "false"
```

**Script additions** (after storage config, before `pack build`):

```bash
CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
NO_CACHE="{{inputs.parameters.no-cache}}"
IMAGE_NAME="{{workflow.parameters.image-name}}"
CACHE_ARGS=""

if [ "$NO_CACHE" = "true" ]; then
  CACHE_ARGS="--clear-cache"
  echo ">> Cache disabled (no-cache=true), building from scratch"
elif [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
  if wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
    CACHE_IMAGE="${CACHE_REGISTRY}/build-cache/cnb/${IMAGE_NAME}:cnb-cache"
    CACHE_ARGS="--cache-image=${CACHE_IMAGE}"

    # Configure insecure registry for HTTP access
    mkdir -p /etc/containers/registries.conf.d
    cat > /etc/containers/registries.conf.d/cache.conf <<REGEOF
[[registry]]
location = "${CACHE_REGISTRY}"
insecure = true
REGEOF

    echo ">> CNB layer cache: ${CACHE_IMAGE}"
  else
    echo ">> Cache registry not reachable, building without cache"
  fi
fi

/tmp/pack build "$IMAGE" \
  --builder "$BUILDER" \
  --run-image "$RUN_IMG" \
  --docker-host inherit \
  --path "$WORKDIR/$APP_PATH" \
  --pull-policy always \
  $CACHE_ARGS \
  $ENV_ARGS
```

When `no-cache=true`: passes `--clear-cache` to `pack build` (ignores existing cache,
rebuilds all buildpack layers). The next normal build writes fresh cache to the registry.

**Cache ref pattern:** `<build-cache>/build-cache/cnb/<namespace-project-component>:cnb-cache`

---

## 4. CI Workflow Changes

The CI workflows (ClusterWorkflow) in `openchoreo/samples/getting-started/ci-workflows/`
are **not modified** in the main OpenChoreo repo. Instead, updated copies live in
`chalindukodikara/openchoreo-build-cache/ci-workflows/` with the `noCache` parameter added.

Each updated CI workflow has three changes from the original:

### 4.1 Developer-Facing Schema Parameter

Add `noCache` to `spec.parameters.openAPIV3Schema.properties` so developers can see and
set it when triggering a build. Default is `true` (cache disabled) — developers opt-in to
caching by setting `noCache: false` once a build-cache registry is deployed.

```yaml
spec:
  parameters:
    openAPIV3Schema:
      type: object
      properties:
        # ... existing properties (repository, docker, buildEnv, buildArgs) ...
        noCache:
          type: boolean
          default: true
          description: "Skip build layer cache and rebuild from scratch. Set to false to enable registry-based layer caching (requires a build-cache registry in the workflow plane)."
```

**Why default `true`:** The cache registry is optional infrastructure deployed separately.
If a cluster doesn't have it, `noCache: true` skips the probe entirely — no wasted time,
no error messages. Developers on clusters with the cache registry set `noCache: false` in
their component's workflow parameters to opt in.

### 4.2 Argo Workflow Parameter

Add `no-cache` to `runTemplate.spec.arguments.parameters` to bridge the OpenChoreo
schema parameter (`noCache`) to the Argo Workflow parameter (`no-cache`):

```yaml
runTemplate:
  spec:
    arguments:
      parameters:
        # ... existing parameters ...
        - name: no-cache
          value: ${parameters.noCache}
```

### 4.3 Build Step Argument

Pass `no-cache` to the build template:

```yaml
- - name: build-image
    templateRef:
      name: containerfile-build
      clusterScope: true
      template: build-image
    arguments:
      parameters:
        - name: git-revision
          value: '{{steps.checkout-source.outputs.parameters.git-revision}}'
        - name: build-env
          value: '{{workflow.parameters.build-env}}'
        - name: build-args
          value: '{{workflow.parameters.build-args}}'
        - name: no-cache
          value: '{{workflow.parameters.no-cache}}'
```

The `build-cache` endpoint is **not** exposed as a developer parameter — it's an
infrastructure detail. The build templates have a default
(`build-cache.openchoreo-workflow-plane.svc.cluster.local:5100`) that works automatically
when the Helm chart is installed. Developers only interact with `noCache`.

### 4.4 Parameter Flow

```
Developer sets noCache: false on component
  → ClusterWorkflow schema validates it as boolean
    → runTemplate maps ${parameters.noCache} → Argo param "no-cache"
      → build step passes {{workflow.parameters.no-cache}} → build template
        → template script: if NO_CACHE != "true" → probe cache registry → use cache
```

### 4.5 Affected CI Workflows

**Affected CI workflows:**

| File | Build Template |
|---|---|
| `dockerfile-builder.yaml` | `containerfile-build` |
| `paketo-buildpacks-builder.yaml` | `paketo-buildpacks-build` |
| `gcp-buildpacks-builder.yaml` | `gcp-buildpacks-build` |
| `ballerina-buildpack-builder.yaml` | `ballerina-buildpack-build` |

---

## 5. Cache Scoping

Cache refs include the component identity (`<namespace>-<project>-<component>`) via
`workflow.parameters.image-name`, so each component gets its own cache namespace:

```
build-cache/podman/default-myproject-myservice:buildcache
build-cache/cnb/default-myproject-myservice:cnb-cache
```

This prevents cache collisions between components. Branch-scoped caching (to avoid
cache thrashing between main and feature branches) is not included in this plan —
add it later if cache thrashing becomes a problem.

---

## 6. GitHub CI — Helm Chart Release

A GitHub Actions workflow packages the Helm chart and pushes it as an OCI artifact to
GitHub Container Registry (GHCR) under `ghcr.io/chalindukodikara/openchoreo-build-cache`.

### 6.1 Workflow: `.github/workflows/release-helm-chart.yaml`

```yaml
name: Release Helm Chart

on:
  push:
    branches: [main]
    paths:
      - "helm/build-cache/Chart.yaml"

permissions:
  contents: read
  packages: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Helm
        uses: azure/setup-helm@v4

      - name: Read chart version
        id: chart
        run: |
          VERSION=$(grep '^version:' helm/build-cache/Chart.yaml | awk '{print $2}')
          echo "version=${VERSION}" >> "$GITHUB_OUTPUT"

      - name: Check if version already exists
        id: check
        run: |
          if helm pull oci://ghcr.io/chalindukodikara/build-cache --version ${{ steps.chart.outputs.version }} 2>/dev/null; then
            echo "exists=true" >> "$GITHUB_OUTPUT"
          else
            echo "exists=false" >> "$GITHUB_OUTPUT"
          fi

      - name: Log in to GHCR
        if: steps.check.outputs.exists == 'false'
        run: echo "${{ secrets.GITHUB_TOKEN }}" | helm registry login ghcr.io -u ${{ github.actor }} --password-stdin

      - name: Package chart
        if: steps.check.outputs.exists == 'false'
        run: helm package helm/build-cache

      - name: Push to GHCR
        if: steps.check.outputs.exists == 'false'
        run: helm push build-cache-${{ steps.chart.outputs.version }}.tgz oci://ghcr.io/chalindukodikara

      - name: Skip (version already published)
        if: steps.check.outputs.exists == 'true'
        run: echo "Chart version ${{ steps.chart.outputs.version }} already exists in GHCR, skipping."
```

### 6.2 How It Works

1. Bump `version` in `helm/build-cache/Chart.yaml` (e.g., `0.1.0` → `0.2.0`)
2. Merge to `main`
3. CI detects `Chart.yaml` changed, reads the version, checks if it already exists in GHCR
4. If new version → packages and pushes to `oci://ghcr.io/chalindukodikara/build-cache:0.2.0`
5. If same version → skips (idempotent, no duplicate pushes)

### 6.3 Chart Versioning

The **source of truth** is the `version` field in `helm/build-cache/Chart.yaml`. To release
a new version:

1. Update `version` in `Chart.yaml`
2. Commit and push to `main`
3. CI publishes automatically

No git tags required. The chart version in `Chart.yaml` is what gets published.

---

## 7. Installation

### 7.1 From GHCR (recommended)

```bash
# k3d
helm install build-cache oci://ghcr.io/chalindukodikara/build-cache \
  --version 0.1.0 \
  --namespace openchoreo-workflow-plane \
  --create-namespace \
  --values https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/helm/build-cache/values-k3d.yaml

# Production
helm install build-cache oci://ghcr.io/chalindukodikara/build-cache \
  --version 0.1.0 \
  --namespace openchoreo-workflow-plane \
  --values https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/helm/build-cache/values-production.yaml \
  --set persistence.storageClass=gp3
```

Then apply workflow templates and CI workflows:

```bash
kubectl apply -f https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/workflow-templates/containerfile-build.yaml
kubectl apply -f https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/workflow-templates/paketo-buildpacks-build.yaml
kubectl apply -f https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/workflow-templates/gcp-buildpacks-build.yaml
kubectl apply -f https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/workflow-templates/ballerina-buildpack-build.yaml
kubectl apply -f https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/ci-workflows/dockerfile-builder.yaml
kubectl apply -f https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/ci-workflows/paketo-buildpacks-builder.yaml
kubectl apply -f https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/ci-workflows/gcp-buildpacks-builder.yaml
kubectl apply -f https://raw.githubusercontent.com/chalindukodikara/openchoreo-build-cache/main/ci-workflows/ballerina-buildpack-builder.yaml
```

### 7.2 From Local Clone

```bash
git clone https://github.com/chalindukodikara/openchoreo-build-cache.git
cd openchoreo-build-cache

# k3d
helm install build-cache ./helm/build-cache \
  --namespace openchoreo-workflow-plane \
  --create-namespace \
  -f helm/build-cache/values-k3d.yaml

# Apply templates
kubectl apply -f workflow-templates/
kubectl apply -f ci-workflows/
```

### 7.3 Disable / Uninstall

```bash
# Remove cache registry
helm uninstall build-cache --namespace openchoreo-workflow-plane

# Optionally restore original CI workflows (from openchoreo repo)
kubectl apply -f <openchoreo-repo>/samples/getting-started/ci-workflows/

# Optionally restore original workflow templates
kubectl apply -f <openchoreo-repo>/samples/getting-started/workflow-templates/
```

After uninstall, if the updated CI workflows remain applied, `noCache` defaults to `true`
so builds skip caching entirely — no errors, no probe overhead. Restoring the originals
is optional.

---

## 7. Execution Order

| Step | What | Depends On |
|---|---|---|
| 1 | Create `chalindukodikara/openchoreo-build-cache` repo | — |
| 2 | Add Helm chart (`helm/build-cache/`) | — |
| 3 | Add GitHub Actions CI (`.github/workflows/release-helm-chart.yaml`) | Step 1 |
| 4 | Create updated `containerfile-build.yaml` with `build-cache` + `no-cache` params | — |
| 5 | Create updated `paketo-buildpacks-build.yaml` with `build-cache` + `no-cache` params | — |
| 6 | Create updated `gcp-buildpacks-build.yaml` with `build-cache` + `no-cache` params | — |
| 7 | Create updated `ballerina-buildpack-build.yaml` with `build-cache` + `no-cache` params | — |
| 8 | Create updated CI workflows with `noCache` schema property + wiring | Steps 4–7 |
| 9 | Bump `Chart.yaml` version → merge to main → CI publishes to GHCR | Steps 2–3 |
| 10 | Test on k3d: install from GHCR, apply templates + CI workflows, run each build type twice | Steps 8–9 |
| 11 | Test `noCache: true` (default): verify builds skip cache entirely | Step 10 |
| 12 | Test `noCache: false`: verify builds use cache, second build is faster | Step 10 |
| 13 | Test graceful fallback: uninstall chart, verify builds still work | Step 12 |
| 14 | Test production values on a real cluster | Step 12 |

Steps 4–7 can proceed in parallel.

---

## 8. Validation Checklist

- [ ] Zot starts and serves `/v2/` on port 5100
- [ ] PVC is mounted and persists across pod restarts
- [ ] Containerfile build: second build is faster (Podman cache hit on unchanged layers)
- [ ] Paketo buildpack build: second build is faster (CNB cache hit)
- [ ] GCP buildpack build: second build is faster (CNB cache hit)
- [ ] Ballerina buildpack build: second build is faster (CNB cache hit)
- [ ] Builds work when cache registry is not deployed (probe fails, no error)
- [ ] Builds work when `build-cache` parameter is set to `"none"`
- [ ] `no-cache=true` triggers a full rebuild (no cached layers used)
- [ ] `no-cache=true` does not destroy existing cache (next normal build still hits cache)
- [ ] Retention: tags older than TTL with no recent pulls are deleted by Zot GC
- [ ] Retention: heavily-pulled tags survive past TTL (pulledWithin rule)
- [ ] GC runs online without downtime (check Zot logs)
- [ ] No auth issues with anonymous push/pull to in-cluster Zot
- [ ] k3d: cache survives cluster restarts (PVC on local-path)
- [ ] Production: PVC binds to correct storage class

---

## 9. Open Questions

1. **Branch-scoped caching:** Currently one cache ref per component. If feature branch builds
   evict main branch cache, add branch to the cache tag:
   `build-cache/podman/<component>:<branch>-buildcache`. Defer until thrashing is observed.

2. **Cache warming:** First build after Zot deploy has no cache. For production, optionally
   run a one-time "warm" build of key components. Not automated — just documentation.

3. **Monitoring:** Zot exposes metrics at `/metrics` (Prometheus format). Consider scraping
   storage usage and GC activity if the observability plane is deployed.

4. **Multi-cluster:** Each cluster gets its own Zot instance + PVC. No cross-cluster cache
   sharing in this plan. Could add a registry mirror/replication layer later if needed.
