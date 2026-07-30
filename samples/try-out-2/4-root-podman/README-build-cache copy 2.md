# Build Mirror and Layer Caching with Zot

Deploy a cluster-internal [Zot](https://zotregistry.dev/) OCI registry in the workflow plane and use it for two independent cache paths:

- **Mirror cache**: pull-through cache for upstream builder, run, lifecycle, and Dockerfile base images.
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

---

## Prerequisites

- A running OpenChoreo cluster with the workflow plane installed
- `kubectl` configured to access the cluster
- [yq v4](https://github.com/mikefarah/yq) for the patch commands

---

## Step 1: Deploy Zot

Create the Zot configuration, deployment, service, and PVC in the `openchoreo-workflow-plane` namespace.

### 1.1 Create the namespace

```bash
kubectl create namespace openchoreo-workflow-plane --dry-run=client -o yaml | kubectl apply -f -
```

### 1.2 Deploy the registry

Use the full Zot image, not `zot-minimal`, because mirroring uses Zot's `sync` extension.

```bash
kubectl apply -f - <<'EOF'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: build-cache-config
  namespace: openchoreo-workflow-plane
data:
  config.json: |
    {
      "distSpecVersion": "1.1.0",
      "storage": {
        "rootDirectory": "/var/lib/registry",
        "gc": true,
        "gcDelay": "2h",
        "gcInterval": "6h",
        "retention": {
          "policies": [
            {
              "repositories": ["build-cache/podman/**"],
              "keepTags": {
                "pushedWithin": "168h",
                "pulledWithin": "168h",
                "mostRecentlyPushedCount": 3,
                "mostRecentlyPulledCount": 3
              }
            },
            {
              "repositories": ["build-cache/cnb/**"],
              "keepTags": {
                "pushedWithin": "168h",
                "pulledWithin": "168h",
                "mostRecentlyPushedCount": 3,
                "mostRecentlyPulledCount": 3
              }
            },
            {
              "repositories": ["mirror/**"],
              "keepTags": {
                "pushedWithin": "336h",
                "pulledWithin": "336h",
                "mostRecentlyPulledCount": 2
              }
            },
            {
              "repositories": ["**"],
              "keepTags": {
                "pushedWithin": "72h",
                "pulledWithin": "72h",
                "mostRecentlyPulledCount": 1
              }
            }
          ]
        }
      },
      "http": {
        "address": "0.0.0.0",
        "port": "5100",
        "compat": ["docker2s2"]
      },
      "log": {
        "level": "warn"
      },
      "extensions": {
        "sync": {
          "enable": true,
          "registries": [
            {
              "urls": ["https://docker.io"],
              "onDemand": true,
              "tlsVerify": true,
              "maxRetries": 3,
              "retryDelay": "30s",
              "content": [
                {
                  "prefix": "/paketobuildpacks/**",
                  "destination": "/mirror/docker.io",
                  "stripPrefix": false
                },
                {
                  "prefix": "/buildpacksio/**",
                  "destination": "/mirror/docker.io",
                  "stripPrefix": false
                },
                {
                  "prefix": "/library/**",
                  "destination": "/mirror/docker.io",
                  "stripPrefix": false
                }
              ]
            },
            {
              "urls": ["https://gcr.io"],
              "onDemand": true,
              "tlsVerify": true,
              "maxRetries": 3,
              "retryDelay": "30s",
              "content": [
                {
                  "prefix": "/buildpacks/**",
                  "destination": "/mirror/gcr.io",
                  "stripPrefix": false
                }
              ]
            },
            {
              "urls": ["https://ghcr.io"],
              "onDemand": true,
              "tlsVerify": true,
              "maxRetries": 3,
              "retryDelay": "30s",
              "content": [
                {
                  "prefix": "/openchoreo/buildpack/**",
                  "destination": "/mirror/ghcr.io",
                  "stripPrefix": false
                }
              ]
            },
            {
              "urls": ["https://quay.io"],
              "onDemand": true,
              "tlsVerify": true,
              "maxRetries": 3,
              "retryDelay": "30s",
              "content": [
                {
                  "prefix": "/podman/**",
                  "destination": "/mirror/quay.io",
                  "stripPrefix": false
                }
              ]
            }
          ]
        }
      }
    }
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: build-cache-data
  namespace: openchoreo-workflow-plane
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: build-cache
  namespace: openchoreo-workflow-plane
spec:
  replicas: 1
  selector:
    matchLabels:
      app: build-cache
  template:
    metadata:
      labels:
        app: build-cache
    spec:
      containers:
        - name: zot
          image: ghcr.io/project-zot/zot-linux-amd64:v2.1.3
          imagePullPolicy: IfNotPresent
          args: ["serve", "/etc/zot/config.json"]
          ports:
            - containerPort: 5100
              name: http
          volumeMounts:
            - name: data
              mountPath: /var/lib/registry
            - name: config
              mountPath: /etc/zot/config.json
              subPath: config.json
              readOnly: true
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 512Mi
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
            name: build-cache-config
        - name: data
          persistentVolumeClaim:
            claimName: build-cache-data
---
apiVersion: v1
kind: Service
metadata:
  name: build-cache
  namespace: openchoreo-workflow-plane
spec:
  type: ClusterIP
  ports:
    - port: 5100
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: build-cache
EOF
```

The `docker2s2` compatibility mode is required when Pack CLI uses `--publish`. Pack can export Docker Manifest v2 Schema 2, and Zot rejects that media type unless this compatibility mode is enabled.

For Docker Hub, use only `onDemand: true`; do not configure polled mirroring because Docker Hub is rate-limited and does not support catalog listing.

### 1.3 Verify Zot

```bash
kubectl -n openchoreo-workflow-plane rollout status deployment/build-cache

kubectl -n openchoreo-workflow-plane port-forward svc/build-cache 5100:5100 &
sleep 2
curl -v http://localhost:5100/v2/
kill %1
```

You should see `HTTP/1.1 200 OK` and `Docker-Distribution-Api-Version: registry/2.0`.

---

## Step 2: Patch Workflow Templates

Patch the in-cluster `ClusterWorkflowTemplate` resources so they accept:

```yaml
- name: build-cache
- name: cache-mirror-enabled
- name: cache-layers-mode
```

### 2.1 Apply the updated ClusterWorkflowTemplates

Run the following `kubectl apply` commands from the OpenChoreo repository root.

#### 2.1.1 containerfile-build

```bash
kubectl apply -f - <<'EOF'
apiVersion: argoproj.io/v1alpha1
kind: ClusterWorkflowTemplate
metadata:
  name: containerfile-build
spec:
  templates:
    - name: build-image
      podSpecPatch: '{"hostUsers": false}'
      inputs:
        parameters:
          - name: git-revision
          - name: build-env
          - name: build-args
          - name: build-cache
            default: "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
          - name: cache-mirror-enabled
            default: "true"
          - name: cache-layers-mode
            default: "reuse"
      volumes:
        - name: storage
          emptyDir:
            sizeLimit: 10Gi
      container:
        image: ghcr.io/openchoreo/podman-runner:v1.2
        command:
          - sh
          - -c
        args:
          - |-
            set -e

            WORKDIR="/mnt/vol/source"
            IMAGE="{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-{{inputs.parameters.git-revision}}"
            DOCKER_CONTEXT="{{workflow.parameters.docker-context}}"
            DOCKERFILE_PATH="{{workflow.parameters.dockerfile-path}}"
            BUILD_ENV_JSON='{{inputs.parameters.build-env}}'
            BUILD_ARGS_JSON='{{inputs.parameters.build-args}}'
            CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
            CACHE_MIRROR_ENABLED="{{inputs.parameters.cache-mirror-enabled}}"
            CACHE_LAYERS_MODE="{{inputs.parameters.cache-layers-mode}}"

            echo ">> Image: $IMAGE"
            echo ">> Dockerfile: $DOCKERFILE_PATH"
            echo ">> Docker context: $DOCKER_CONTEXT"

            if [ ! -f "$WORKDIR/$DOCKERFILE_PATH" ]; then
              echo ">> Error: Dockerfile not found at: '$DOCKERFILE_PATH'"
              echo ">> Hint: Verify that the Dockerfile path is correct and relative to the repository root."
              echo ">> Repository contents:"
              ls -la "$WORKDIR/"
              exit 1
            fi

            if [ ! -d "$WORKDIR/$DOCKER_CONTEXT" ]; then
              echo ">> Error: Docker build context directory not found: '$DOCKER_CONTEXT'"
              echo ">> Hint: Verify that the Docker build context points to a valid directory relative to the repository root."
              echo ">> Repository contents:"
              ls -la "$WORKDIR/"
              exit 1
            fi

            case "$CACHE_LAYERS_MODE" in
              disabled|reuse|rebuild) ;;
              *)
                echo ">> Error: invalid cache layers mode '$CACHE_LAYERS_MODE' (expected disabled, reuse, or rebuild)"
                exit 1
                ;;
            esac

            mkdir -p /storage/run /storage/graph
            cat > /etc/containers/storage.conf <<STEOF
            [storage]
            driver = "overlay"
            runroot = "/storage/run"
            graphroot = "/storage/graph"
            [storage.options.overlay]
            STEOF

            ENV_ARGS=""
            if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
              ENV_ARGS=$(echo "$BUILD_ENV_JSON" | jq -r '.[] | "--env \(.name)=\(.value)"' | tr '\n' ' ')
            fi

            BUILD_ARG_ARGS=""
            if [ -n "$BUILD_ARGS_JSON" ] && [ "$BUILD_ARGS_JSON" != "[]" ]; then
              BUILD_ARG_ARGS=$(echo "$BUILD_ARGS_JSON" | jq -r '.[] | "--build-arg \(.name)=\(.value)"' | tr '\n' ' ')
            fi

            CACHE_AVAILABLE="false"
            CACHE_ARGS=""
            if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
              if curl -sf --connect-timeout 3 "http://${CACHE_REGISTRY}/v2/" >/dev/null 2>&1; then
                CACHE_AVAILABLE="true"
                mkdir -p /etc/containers/registries.conf.d
                cat > /etc/containers/registries.conf.d/build-cache.conf <<REGEOF
            [[registry]]
            location = "${CACHE_REGISTRY}"
            insecure = true
            REGEOF
              fi
            fi

            if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
              cat >> /etc/containers/registries.conf.d/build-cache.conf <<REGEOF

            [[registry]]
            prefix = "docker.io"
            location = "${CACHE_REGISTRY}/mirror/docker.io"
            insecure = true

            [[registry]]
            prefix = "gcr.io"
            location = "${CACHE_REGISTRY}/mirror/gcr.io"
            insecure = true

            [[registry]]
            prefix = "ghcr.io"
            location = "${CACHE_REGISTRY}/mirror/ghcr.io"
            insecure = true

            [[registry]]
            prefix = "quay.io"
            location = "${CACHE_REGISTRY}/mirror/quay.io"
            insecure = true
            REGEOF
              echo ">> Upstream image mirror: ${CACHE_REGISTRY}/mirror"
            elif [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
              echo ">> Cache registry not reachable, pulling base images from upstream registries"
            else
              echo ">> Upstream image mirror disabled"
            fi

            if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_LAYERS_MODE" != "disabled" ]; then
              CACHE_REF="${CACHE_REGISTRY}/build-cache/podman/{{workflow.parameters.image-name}}"
              case "$CACHE_LAYERS_MODE" in
                reuse)
                  CACHE_ARGS="--layers --cache-from=${CACHE_REF} --cache-to=${CACHE_REF} --cache-ttl=168h"
                  echo ">> Layer cache: reuse ${CACHE_REF}"
                  ;;
                rebuild)
                  CACHE_ARGS="--layers --cache-to=${CACHE_REF} --cache-ttl=168h"
                  echo ">> Layer cache: rebuild ${CACHE_REF}"
                  ;;
              esac
            elif [ "$CACHE_LAYERS_MODE" != "disabled" ]; then
              echo ">> Cache registry not reachable, building without layer cache"
            else
              echo ">> Layer cache disabled"
            fi

            echo ">> Building image"
            podman build -t $IMAGE -f $WORKDIR/$DOCKERFILE_PATH $ENV_ARGS $BUILD_ARG_ARGS $CACHE_ARGS $WORKDIR/$DOCKER_CONTEXT
            echo ">> Image built successfully"
            podman save -o /mnt/vol/app-image.tar $IMAGE
        securityContext:
          privileged: true
        volumeMounts:
          - mountPath: /mnt/vol
            name: workspace
          - mountPath: /storage
            name: storage
EOF
```

#### 2.1.2 Buildpacks templates

The three Buildpacks templates share the same cache flow. Apply each full template with the following commands.

```bash
kubectl apply -f - <<'EOF'
apiVersion: argoproj.io/v1alpha1
kind: ClusterWorkflowTemplate
metadata:
  name: paketo-buildpacks-build
spec:
  templates:
    - name: build-image
      podSpecPatch: '{"hostUsers": false}'
      inputs:
        parameters:
          - name: git-revision
          - name: build-cache
            default: "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
          - name: cache-mirror-enabled
            default: "true"
          - name: cache-layers-mode
            default: "reuse"
      volumes:
        - name: storage
          emptyDir:
            sizeLimit: 10Gi
      container:
        image: ghcr.io/openchoreo/podman-runner:v1.2
        command: [sh, -c]
        args:
          - |-
            set -e

            WORKDIR=/mnt/vol/source
            GIT_REVISION="{{inputs.parameters.git-revision}}"
            IMAGE="{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-${GIT_REVISION}"
            APP_PATH="{{workflow.parameters.app-path}}"
            BUILD_ENV_JSON='{{workflow.parameters.build-env}}'
            CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
            CACHE_MIRROR_ENABLED="{{inputs.parameters.cache-mirror-enabled}}"
            CACHE_LAYERS_MODE="{{inputs.parameters.cache-layers-mode}}"

            case "$CACHE_LAYERS_MODE" in disabled|reuse|rebuild) ;; *) echo ">> Error: invalid cache layers mode '$CACHE_LAYERS_MODE'"; exit 1 ;; esac

            if [ ! -d "$WORKDIR/$APP_PATH" ]; then
              echo ">> Error: The specified application path '$APP_PATH' does not exist in the repository"
              ls -la "$WORKDIR/"
              exit 1
            fi

            mkdir -p /storage/run /storage/graph
            cat > /etc/containers/storage.conf <<STEOF
            [storage]
            driver = "overlay"
            runroot = "/storage/run"
            graphroot = "/storage/graph"
            [storage.options.overlay]
            STEOF

            cat > /etc/containers/containers.conf <<CEOF
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
            CEOF

            mkdir -p /run/podman
            podman system service --time=0 unix:///run/podman/podman.sock &
            until podman info --format '{{.Host.RemoteSocket.Exists}}' 2>/dev/null | grep -q true; do sleep 1; done
            export DOCKER_HOST=unix:///run/podman/podman.sock

            UPSTREAM_BUILDER="docker.io/paketobuildpacks/builder-jammy-full:0.3.603"
            UPSTREAM_RUN_IMG="docker.io/paketobuildpacks/run-jammy-full:0.1.130"
            BUILDER="$UPSTREAM_BUILDER"
            RUN_IMG="$UPSTREAM_RUN_IMG"

            ENV_ARGS=""
            if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
              ENV_ARGS=$(echo "$BUILD_ENV_JSON" | jq -r '.[] | "--env \(.name)=\(.value)"' | tr '\n' ' ')
            fi

            CACHE_AVAILABLE="false"
            PACK_REGISTRY_ARGS=""
            if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
              if curl -sf --max-time 3 -o /dev/null "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
                CACHE_AVAILABLE="true"
                PACK_REGISTRY_ARGS="--insecure-registry ${CACHE_REGISTRY}"
                mkdir -p /etc/containers/registries.conf.d
                cat > /etc/containers/registries.conf.d/build-cache.conf <<REGEOF
            [[registry]]
            location = "${CACHE_REGISTRY}"
            insecure = true
            REGEOF
              fi
            fi

            if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
              BUILDER="${CACHE_REGISTRY}/mirror/docker.io/paketobuildpacks/builder-jammy-full:0.3.603"
              RUN_IMG="${CACHE_REGISTRY}/mirror/docker.io/paketobuildpacks/run-jammy-full:0.1.130"
              pack config lifecycle-image "${CACHE_REGISTRY}/mirror/docker.io/buildpacksio/lifecycle"
              echo ">> Upstream image mirror: ${CACHE_REGISTRY}/mirror/docker.io"
            elif [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
              echo ">> Cache registry not reachable, pulling builder and run image from upstream registries"
            else
              echo ">> Upstream image mirror disabled"
            fi

            if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_LAYERS_MODE" != "disabled" ]; then
              CACHE_IMAGE="${CACHE_REGISTRY}/build-cache/cnb/{{workflow.parameters.image-name}}:cnb-cache"
              PUBLISH_REF="${CACHE_REGISTRY}/build-cache/cnb/{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-${GIT_REVISION}"
              CLEAR_CACHE_ARG=""
              if [ "$CACHE_LAYERS_MODE" = "rebuild" ]; then CLEAR_CACHE_ARG="--clear-cache"; fi

              pack build "$PUBLISH_REF" \
                --builder "$BUILDER" \
                --run-image "$RUN_IMG" \
                --path "$WORKDIR/$APP_PATH" \
                --pull-policy always \
                --publish \
                $PACK_REGISTRY_ARGS \
                --cache-image "$CACHE_IMAGE" \
                $CLEAR_CACHE_ARG \
                $ENV_ARGS

              podman pull --tls-verify=false "$PUBLISH_REF"
              podman tag "$PUBLISH_REF" "$IMAGE"
              podman save -o /mnt/vol/app-image.tar "$IMAGE"
              exit 0
            fi

            pack build "$IMAGE" \
              --builder "$BUILDER" \
              --run-image "$RUN_IMG" \
              --docker-host inherit \
              --path "$WORKDIR/$APP_PATH" \
              --pull-policy always \
              $PACK_REGISTRY_ARGS \
              $ENV_ARGS

            until podman image exists "$IMAGE" 2>/dev/null; do sleep 1; done
            podman save -o /mnt/vol/app-image.tar "$IMAGE"
        securityContext:
          privileged: true
        volumeMounts:
          - mountPath: /mnt/vol
            name: workspace
          - mountPath: /storage
            name: storage
EOF
```

```bash
kubectl apply -f - <<'EOF'
apiVersion: argoproj.io/v1alpha1
kind: ClusterWorkflowTemplate
metadata:
  name: gcp-buildpacks-build
spec:
  templates:
    - name: build-image
      podSpecPatch: '{"hostUsers": false}'
      inputs:
        parameters:
          - name: git-revision
          - name: build-cache
            default: "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
          - name: cache-mirror-enabled
            default: "true"
          - name: cache-layers-mode
            default: "reuse"
      volumes:
        - name: storage
          emptyDir:
            sizeLimit: 10Gi
      container:
        image: ghcr.io/openchoreo/podman-runner:v1.2
        command: [sh, -c]
        args:
          - |-
            set -e

            WORKDIR=/mnt/vol/source
            GIT_REVISION="{{inputs.parameters.git-revision}}"
            IMAGE="{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-${GIT_REVISION}"
            APP_PATH="{{workflow.parameters.app-path}}"
            BUILD_ENV_JSON='{{workflow.parameters.build-env}}'
            CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
            CACHE_MIRROR_ENABLED="{{inputs.parameters.cache-mirror-enabled}}"
            CACHE_LAYERS_MODE="{{inputs.parameters.cache-layers-mode}}"

            case "$CACHE_LAYERS_MODE" in disabled|reuse|rebuild) ;; *) echo ">> Error: invalid cache layers mode '$CACHE_LAYERS_MODE'"; exit 1 ;; esac

            if [ ! -d "$WORKDIR/$APP_PATH" ]; then
              echo ">> Error: The specified application path '$APP_PATH' does not exist in the repository"
              ls -la "$WORKDIR/"
              exit 1
            fi

            mkdir -p /storage/run /storage/graph
            cat > /etc/containers/storage.conf <<STEOF
            [storage]
            driver = "overlay"
            runroot = "/storage/run"
            graphroot = "/storage/graph"
            [storage.options.overlay]
            STEOF

            cat > /etc/containers/containers.conf <<CEOF
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
            CEOF

            mkdir -p /run/podman
            podman system service --time=0 unix:///run/podman/podman.sock &
            until podman info --format '{{.Host.RemoteSocket.Exists}}' 2>/dev/null | grep -q true; do sleep 1; done
            export DOCKER_HOST=unix:///run/podman/podman.sock

            UPSTREAM_BUILDER="gcr.io/buildpacks/builder@sha256:5977b4bd47d3e9ff729eefe9eb99d321d4bba7aa3b14986323133f40b622aef1"
            UPSTREAM_RUN_IMG="gcr.io/buildpacks/google-22/run@sha256:a8ccb6641b4d98b0adf6397f954e7194611d1ae61310f0561f1c00fdf7f9ba96"
            BUILDER="$UPSTREAM_BUILDER"
            RUN_IMG="$UPSTREAM_RUN_IMG"

            ENV_ARGS=""
            if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
              ENV_ARGS=$(echo "$BUILD_ENV_JSON" | jq -r '.[] | "--env \(.name)=\(.value)"' | tr '\n' ' ')
            fi

            CACHE_AVAILABLE="false"
            PACK_REGISTRY_ARGS=""
            if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
              if curl -sf --max-time 3 -o /dev/null "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
                CACHE_AVAILABLE="true"
                PACK_REGISTRY_ARGS="--insecure-registry ${CACHE_REGISTRY}"
                mkdir -p /etc/containers/registries.conf.d
                cat > /etc/containers/registries.conf.d/build-cache.conf <<REGEOF
            [[registry]]
            location = "${CACHE_REGISTRY}"
            insecure = true
            REGEOF
              fi
            fi

            if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
              BUILDER="${CACHE_REGISTRY}/mirror/gcr.io/buildpacks/builder@sha256:5977b4bd47d3e9ff729eefe9eb99d321d4bba7aa3b14986323133f40b622aef1"
              RUN_IMG="${CACHE_REGISTRY}/mirror/gcr.io/buildpacks/google-22/run@sha256:a8ccb6641b4d98b0adf6397f954e7194611d1ae61310f0561f1c00fdf7f9ba96"
              pack config lifecycle-image "${CACHE_REGISTRY}/mirror/docker.io/buildpacksio/lifecycle"
              echo ">> Upstream image mirror: ${CACHE_REGISTRY}/mirror/gcr.io"
            elif [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
              echo ">> Cache registry not reachable, pulling builder and run image from upstream registries"
            else
              echo ">> Upstream image mirror disabled"
            fi

            if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_LAYERS_MODE" != "disabled" ]; then
              CACHE_IMAGE="${CACHE_REGISTRY}/build-cache/cnb/{{workflow.parameters.image-name}}:cnb-cache"
              PUBLISH_REF="${CACHE_REGISTRY}/build-cache/cnb/{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-${GIT_REVISION}"
              CLEAR_CACHE_ARG=""
              if [ "$CACHE_LAYERS_MODE" = "rebuild" ]; then CLEAR_CACHE_ARG="--clear-cache"; fi

              pack build "$PUBLISH_REF" \
                --builder "$BUILDER" \
                --run-image "$RUN_IMG" \
                --path "$WORKDIR/$APP_PATH" \
                --pull-policy always \
                --publish \
                $PACK_REGISTRY_ARGS \
                --cache-image "$CACHE_IMAGE" \
                $CLEAR_CACHE_ARG \
                $ENV_ARGS

              podman pull --tls-verify=false "$PUBLISH_REF"
              podman tag "$PUBLISH_REF" "$IMAGE"
              podman save -o /mnt/vol/app-image.tar "$IMAGE"
              exit 0
            fi

            pack build "$IMAGE" \
              --builder "$BUILDER" \
              --run-image "$RUN_IMG" \
              --docker-host inherit \
              --path "$WORKDIR/$APP_PATH" \
              --pull-policy always \
              $PACK_REGISTRY_ARGS \
              $ENV_ARGS

            until podman image exists "$IMAGE" 2>/dev/null; do sleep 1; done
            podman save -o /mnt/vol/app-image.tar "$IMAGE"
        securityContext:
          privileged: true
        volumeMounts:
          - mountPath: /mnt/vol
            name: workspace
          - mountPath: /storage
            name: storage
EOF
```

```bash
kubectl apply -f - <<'EOF'
apiVersion: argoproj.io/v1alpha1
kind: ClusterWorkflowTemplate
metadata:
  name: ballerina-buildpack-build
spec:
  templates:
    - name: build-image
      podSpecPatch: '{"hostUsers": false}'
      inputs:
        parameters:
          - name: git-revision
          - name: build-env
          - name: build-cache
            default: "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
          - name: cache-mirror-enabled
            default: "true"
          - name: cache-layers-mode
            default: "reuse"
      volumes:
        - name: storage
          emptyDir:
            sizeLimit: 10Gi
        - name: app-dir
          emptyDir: {}
      container:
        image: ghcr.io/openchoreo/podman-runner:v1.2
        command: [sh, -c]
        args:
          - |-
            set -e

            WORKDIR=/mnt/vol/source
            GIT_REVISION="{{inputs.parameters.git-revision}}"
            IMAGE="{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-${GIT_REVISION}"
            APP_PATH="{{workflow.parameters.app-path}}"
            BUILD_ENV_JSON='{{inputs.parameters.build-env}}'
            CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
            CACHE_MIRROR_ENABLED="{{inputs.parameters.cache-mirror-enabled}}"
            CACHE_LAYERS_MODE="{{inputs.parameters.cache-layers-mode}}"

            case "$CACHE_LAYERS_MODE" in disabled|reuse|rebuild) ;; *) echo ">> Error: invalid cache layers mode '$CACHE_LAYERS_MODE'"; exit 1 ;; esac

            if [ ! -d "$WORKDIR/$APP_PATH" ]; then
              echo ">> Error: The specified application path '$APP_PATH' does not exist in the repository"
              ls -la "$WORKDIR/"
              exit 1
            fi

            mkdir -p /storage/run /storage/graph
            cat > /etc/containers/storage.conf <<STEOF
            [storage]
            driver = "overlay"
            runroot = "/storage/run"
            graphroot = "/storage/graph"
            [storage.options.overlay]
            STEOF

            cat > /etc/containers/containers.conf <<CEOF
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
            CEOF

            mkdir -p /run/podman
            podman system service --time=0 unix:///run/podman/podman.sock &
            until podman info --format '{{.Host.RemoteSocket.Exists}}' 2>/dev/null | grep -q true; do sleep 1; done
            export DOCKER_HOST=unix:///run/podman/podman.sock

            UPSTREAM_BUILDER="ghcr.io/openchoreo/buildpack/ballerina:18"
            UPSTREAM_RUN_IMAGE="ghcr.io/openchoreo/buildpack/ballerina:18-run"
            BUILDER="$UPSTREAM_BUILDER"
            RUN_IMAGE="$UPSTREAM_RUN_IMAGE"

            ENV_ARGS=""
            if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
              ENV_ARGS=$(echo "$BUILD_ENV_JSON" | jq -r '.[] | "--env \(.name)=\(.value)"' | tr '\n' ' ')
            fi

            CACHE_AVAILABLE="false"
            PACK_REGISTRY_ARGS=""
            if [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
              if curl -sf --max-time 3 -o /dev/null "http://${CACHE_REGISTRY}/v2/" 2>/dev/null; then
                CACHE_AVAILABLE="true"
                PACK_REGISTRY_ARGS="--insecure-registry ${CACHE_REGISTRY}"
                mkdir -p /etc/containers/registries.conf.d
                cat > /etc/containers/registries.conf.d/build-cache.conf <<REGEOF
            [[registry]]
            location = "${CACHE_REGISTRY}"
            insecure = true
            REGEOF
              fi
            fi

            if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
              BUILDER="${CACHE_REGISTRY}/mirror/ghcr.io/openchoreo/buildpack/ballerina:18"
              RUN_IMAGE="${CACHE_REGISTRY}/mirror/ghcr.io/openchoreo/buildpack/ballerina:18-run"
              pack config lifecycle-image "${CACHE_REGISTRY}/mirror/docker.io/buildpacksio/lifecycle"
              echo ">> Upstream image mirror: ${CACHE_REGISTRY}/mirror/ghcr.io"
            elif [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
              echo ">> Cache registry not reachable, pulling builder and run image from upstream registries"
            else
              echo ">> Upstream image mirror disabled"
            fi

            if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_LAYERS_MODE" != "disabled" ]; then
              CACHE_IMAGE="${CACHE_REGISTRY}/build-cache/cnb/{{workflow.parameters.image-name}}:cnb-cache"
              PUBLISH_REF="${CACHE_REGISTRY}/build-cache/cnb/{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-${GIT_REVISION}"
              CLEAR_CACHE_ARG=""
              if [ "$CACHE_LAYERS_MODE" = "rebuild" ]; then CLEAR_CACHE_ARG="--clear-cache"; fi

              pack build "$PUBLISH_REF" \
                --builder "$BUILDER" \
                --run-image "$RUN_IMAGE" \
                --path "$WORKDIR/$APP_PATH" \
                --volume "/mnt/vol:/app/generated-artifacts:rw" \
                --pull-policy always \
                --publish \
                $PACK_REGISTRY_ARGS \
                --cache-image "$CACHE_IMAGE" \
                $CLEAR_CACHE_ARG \
                $ENV_ARGS

              podman pull --tls-verify=false "$PUBLISH_REF"
              podman tag "$PUBLISH_REF" "$IMAGE"
              podman save -o /mnt/vol/app-image.tar "$IMAGE"
              exit 0
            fi

            pack build "$IMAGE" \
              --builder "$BUILDER" \
              --run-image "$RUN_IMAGE" \
              --docker-host inherit \
              --path "$WORKDIR/$APP_PATH" \
              --volume "/mnt/vol:/app/generated-artifacts:rw" \
              --pull-policy always \
              $PACK_REGISTRY_ARGS \
              $ENV_ARGS

            until podman image exists "$IMAGE" 2>/dev/null; do sleep 1; done
            podman save -o /mnt/vol/app-image.tar "$IMAGE"
        securityContext:
          privileged: true
        volumeMounts:
          - mountPath: /mnt/vol
            name: workspace
          - mountPath: /storage
            name: storage
          - mountPath: /app
            name: app-dir
EOF
```

Use these values in build scripts:

```bash
CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
CACHE_MIRROR_ENABLED="{{inputs.parameters.cache-mirror-enabled}}"
CACHE_LAYERS_MODE="{{inputs.parameters.cache-layers-mode}}"
```

Validate layer mode early:

```bash
case "$CACHE_LAYERS_MODE" in
  disabled|reuse|rebuild) ;;
  *)
    echo ">> Error: invalid cache layers mode '$CACHE_LAYERS_MODE' (expected disabled, reuse, or rebuild)"
    exit 1
    ;;
esac
```

### 2.2 Common cache probe

Add this after storage setup in every build template:

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


### 2.3 Podman Dockerfile layer cache

Patch `containerfile-build` so `podman build` receives `CACHE_ARGS`:

```bash
CACHE_ARGS=""

if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
  cat >> /etc/containers/registries.conf.d/build-cache.conf <<REGEOF

[[registry]]
prefix = "docker.io"
location = "${CACHE_REGISTRY}/mirror/docker.io"
insecure = true

[[registry]]
prefix = "gcr.io"
location = "${CACHE_REGISTRY}/mirror/gcr.io"
insecure = true

[[registry]]
prefix = "ghcr.io"
location = "${CACHE_REGISTRY}/mirror/ghcr.io"
insecure = true

[[registry]]
prefix = "quay.io"
location = "${CACHE_REGISTRY}/mirror/quay.io"
insecure = true
REGEOF
  echo ">> Upstream image mirror: ${CACHE_REGISTRY}/mirror"
elif [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
  echo ">> Cache registry not reachable, pulling base images from upstream registries"
else
  echo ">> Upstream image mirror disabled"
fi

if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_LAYERS_MODE" != "disabled" ]; then
  CACHE_REF="${CACHE_REGISTRY}/build-cache/podman/${IMAGE_NAME}"
  case "$CACHE_LAYERS_MODE" in
    reuse)
      CACHE_ARGS="--layers --cache-from=${CACHE_REF} --cache-to=${CACHE_REF} --cache-ttl=168h"
      echo ">> Layer cache: reuse ${CACHE_REF}"
      ;;
    rebuild)
      CACHE_ARGS="--layers --cache-to=${CACHE_REF} --cache-ttl=168h"
      echo ">> Layer cache: rebuild ${CACHE_REF}"
      ;;
  esac
elif [ "$CACHE_LAYERS_MODE" != "disabled" ]; then
  echo ">> Cache registry not reachable, building without layer cache"
else
  echo ">> Layer cache disabled"
fi
```

Then include `$CACHE_ARGS`:

```bash
podman build -t $IMAGE -f $WORKDIR/$DOCKERFILE_PATH $ENV_ARGS $BUILD_ARG_ARGS $CACHE_ARGS $WORKDIR/$DOCKER_CONTEXT
```

### 2.4 Buildpacks mirror and layer cache

Patch each Buildpacks template (`paketo-buildpacks-build`, `gcp-buildpacks-build`, `ballerina-buildpack-build`) to keep upstream refs and switch to mirror refs only when mirror cache is enabled and Zot is reachable.

Paketo:

```bash
UPSTREAM_BUILDER="docker.io/paketobuildpacks/builder-jammy-full:0.3.603"
UPSTREAM_RUN_IMG="docker.io/paketobuildpacks/run-jammy-full:0.1.130"
BUILDER="$UPSTREAM_BUILDER"
RUN_IMG="$UPSTREAM_RUN_IMG"

if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
  BUILDER="${CACHE_REGISTRY}/mirror/docker.io/paketobuildpacks/builder-jammy-full:0.3.603"
  RUN_IMG="${CACHE_REGISTRY}/mirror/docker.io/paketobuildpacks/run-jammy-full:0.1.130"
fi
```

GCP:

```bash
UPSTREAM_BUILDER="gcr.io/buildpacks/builder@sha256:5977b4bd47d3e9ff729eefe9eb99d321d4bba7aa3b14986323133f40b622aef1"
UPSTREAM_RUN_IMG="gcr.io/buildpacks/google-22/run@sha256:a8ccb6641b4d98b0adf6397f954e7194611d1ae61310f0561f1c00fdf7f9ba96"
BUILDER="$UPSTREAM_BUILDER"
RUN_IMG="$UPSTREAM_RUN_IMG"

if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
  BUILDER="${CACHE_REGISTRY}/mirror/gcr.io/buildpacks/builder@sha256:5977b4bd47d3e9ff729eefe9eb99d321d4bba7aa3b14986323133f40b622aef1"
  RUN_IMG="${CACHE_REGISTRY}/mirror/gcr.io/buildpacks/google-22/run@sha256:a8ccb6641b4d98b0adf6397f954e7194611d1ae61310f0561f1c00fdf7f9ba96"
fi
```

Ballerina:

```bash
UPSTREAM_BUILDER="ghcr.io/openchoreo/buildpack/ballerina:18"
UPSTREAM_RUN_IMAGE="ghcr.io/openchoreo/buildpack/ballerina:18-run"
BUILDER="$UPSTREAM_BUILDER"
RUN_IMAGE="$UPSTREAM_RUN_IMAGE"

if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_MIRROR_ENABLED" = "true" ]; then
  BUILDER="${CACHE_REGISTRY}/mirror/ghcr.io/openchoreo/buildpack/ballerina:18"
  RUN_IMAGE="${CACHE_REGISTRY}/mirror/ghcr.io/openchoreo/buildpack/ballerina:18-run"
fi
```

When layer cache is enabled, Pack must use `--publish` with a registry cache image. `rebuild` adds `--clear-cache`, which ignores the previous cache and writes a new cache for later builds:

```bash
PACK_REGISTRY_ARGS=""
if [ "$CACHE_AVAILABLE" = "true" ]; then
  PACK_REGISTRY_ARGS="--insecure-registry ${CACHE_REGISTRY}"
fi

if [ "$CACHE_AVAILABLE" = "true" ] && [ "$CACHE_LAYERS_MODE" != "disabled" ]; then
  CACHE_IMAGE="${CACHE_REGISTRY}/build-cache/cnb/${IMAGE_NAME}:cnb-cache"
  PUBLISH_REF="${CACHE_REGISTRY}/build-cache/cnb/${IMAGE_NAME}:${IMAGE_TAG}-${GIT_REVISION}"
  CLEAR_CACHE_ARG=""

  if [ "$CACHE_LAYERS_MODE" = "rebuild" ]; then
    CLEAR_CACHE_ARG="--clear-cache"
    echo ">> CNB layer cache: rebuild ${CACHE_IMAGE}"
  else
    echo ">> CNB layer cache: reuse ${CACHE_IMAGE}"
  fi

  pack build "$PUBLISH_REF" \
    --builder "$BUILDER" \
    --run-image "$RUN_IMG" \
    --path "$WORKDIR/$APP_PATH" \
    --pull-policy always \
    --publish \
    $PACK_REGISTRY_ARGS \
    --cache-image "$CACHE_IMAGE" \
    $CLEAR_CACHE_ARG \
    $ENV_ARGS

  podman pull --tls-verify=false "$PUBLISH_REF"
  podman tag "$PUBLISH_REF" "$IMAGE"
  podman save -o /mnt/vol/app-image.tar "$IMAGE"
  exit 0
fi
```

For Ballerina, use `RUN_IMAGE` instead of `RUN_IMG` in the generic Pack snippet above.

For Ballerina, keep the existing generated-artifacts volume:

```bash
--volume "/mnt/vol:/app/generated-artifacts:rw"
```

If Pack uses an untrusted builder and pulls a separate lifecycle image, point the lifecycle image at the Docker Hub mirror:

```bash
pack config lifecycle-image "${CACHE_REGISTRY}/mirror/docker.io/buildpacksio/lifecycle"
```

If layer cache is disabled or Zot is unreachable, continue to use the existing local Podman build path:

```bash
pack build "$IMAGE" \
  --builder "$BUILDER" \
  --run-image "$RUN_IMG" \
  --docker-host inherit \
  --path "$WORKDIR/$APP_PATH" \
  --pull-policy always \
  $PACK_REGISTRY_ARGS \
  $ENV_ARGS
```

### 2.5 Template patch checklist

For each `ClusterWorkflowTemplate`:

1. Add input parameters:

```yaml
- name: build-cache
  default: "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"
- name: cache-mirror-enabled
  default: "true"
- name: cache-layers-mode
  default: "reuse"
```

2. Add the common probe and layer-mode validation.
3. Add mirror registry handling.
4. Add `reuse` and `rebuild` layer-cache behavior.

---

## Step 3: Patch CI Workflows

Patch the existing `ClusterWorkflow` resources so developers configure cache with a structured parameter and Argo passes flat scalar parameters to the build templates.

### 3.1 Patch all four CI workflows

```bash
CACHE_DESC="Build cache configuration. mirror.enabled controls upstream image pull-through caching. layers.mode controls layer cache behavior: disabled, reuse, or rebuild."

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
        [.[] | select(.name != "build-cache" and .name != "cache-mirror-enabled" and .name != "cache-layers-mode")] +
        [
          {"name": "build-cache", "value": "build-cache.openchoreo-workflow-plane.svc.cluster.local:5100"},
          {"name": "cache-mirror-enabled", "value": "${parameters.cache.mirror.enabled}"},
          {"name": "cache-layers-mode", "value": "${parameters.cache.layers.mode}"}
        ] |

      .spec.runTemplate.spec.templates[0].steps[1][0].arguments.parameters |=
        [.[] | select(.name != "build-cache" and .name != "cache-mirror-enabled" and .name != "cache-layers-mode")] +
        [
          {"name": "build-cache", "value": "{{workflow.parameters.build-cache}}"},
          {"name": "cache-mirror-enabled", "value": "{{workflow.parameters.cache-mirror-enabled}}"},
          {"name": "cache-layers-mode", "value": "{{workflow.parameters.cache-layers-mode}}"}
        ]
    ' | kubectl apply -f -
  echo "Patched $workflow"
done
```

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
    "build-cache/podman/default-myproject-myservice",
    "build-cache/cnb/default-myproject-myservice",
    "mirror/docker.io/paketobuildpacks/builder-jammy-full",
    "mirror/ghcr.io/openchoreo/buildpack/ballerina"
  ]
}
```

### 5.2 Check build logs

Mirror enabled:

```text
>> Upstream image mirror: build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/mirror
```

Layer cache reuse:

```text
>> Layer cache: reuse build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/build-cache/podman/<component>
```

Layer cache rebuild:

```text
>> Layer cache: rebuild build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/build-cache/podman/<component>
```

Registry unavailable:

```text
>> Cache registry not reachable, building without layer cache
```

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
kubectl delete deployment build-cache -n openchoreo-workflow-plane
kubectl delete service build-cache -n openchoreo-workflow-plane
kubectl delete configmap build-cache-config -n openchoreo-workflow-plane
kubectl delete pvc build-cache-data -n openchoreo-workflow-plane
```

Builds continue to work normally if templates probe the registry and fall back to upstream pulls/no layer cache when Zot is unreachable.

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
    -> runTemplate maps nested fields to Argo parameters
      -> build step passes flat parameters to ClusterWorkflowTemplate
        -> template probes Zot
          -> mirror enabled: use local mirror refs or registry remapping
          -> layers.mode reuse: read and write layer cache
          -> layers.mode rebuild: skip layer-cache read and write a fresh cache
```

### Cache reference paths

| Cache type | Reference pattern |
|---|---|
| Podman layer cache | `build-cache/podman/<namespace-project-component>` |
| CNB layer cache | `build-cache/cnb/<namespace-project-component>:cnb-cache` |
| Published CNB handoff image | `build-cache/cnb/<namespace-project-component>:<image-tag>-<git-revision>` |
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
