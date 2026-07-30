# Build Layer Caching with a Zot-Based OCI Cache Registry

Speed up repeat builds by deploying a cluster-internal [Zot](https://zotregistry.dev/) OCI registry as a build layer cache. Once configured, Dockerfile builds (Podman) and Buildpack builds (Pack CLI) push and pull cached layers from this registry, skipping redundant downloads of base images, runtimes, and dependencies.

**What you get:**

- Dockerfile builds use Podman `--cache-from` / `--cache-to` for registry-backed layer caching
- Buildpack builds (Paketo, GCP, Ballerina) use Pack CLI `--publish` + `--cache-image` for CNB layer caching, then pull the built image back for the workflow tarball
- Automatic cache cleanup via Zot's built-in retention policies and online GC
- Graceful fallback -- if the cache registry is not deployed or unreachable, builds proceed normally

---

## Prerequisites

- A running OpenChoreo cluster with the workflow plane installed
- `kubectl` configured to access the cluster
- `helm` v3 installed (for the Zot registry)

---

## Step 1: Deploy the Zot Cache Registry

Create the Zot configuration, deployment, service, and PVC in the `openchoreo-workflow-plane` namespace.

### 1.1 Create the namespace (if it doesn't already exist)

```bash
kubectl create namespace openchoreo-workflow-plane --dry-run=client -o yaml | kubectl apply -f -
```

### 1.2 Deploy the Zot registry

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
          image: ghcr.io/project-zot/zot-minimal-linux-amd64:v2.1.3
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
              cpu: 200m
              memory: 256Mi
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

The `docker2s2` compatibility mode is required if Pack CLI is used with `--publish`.
Pack exports Docker Manifest v2 Schema 2 (`application/vnd.docker.distribution.manifest.v2+json`),
and Zot rejects that media type unless this compatibility mode is enabled.

### 1.3 Verify the registry is running

```bash
kubectl -n openchoreo-workflow-plane rollout status deployment/build-cache
```

Then port-forward and test the API:

```bash
kubectl -n openchoreo-workflow-plane port-forward svc/build-cache 5100:5100 &
sleep 2
curl -v http://localhost:5100/v2/
kill %1
```

You should see `HTTP/1.1 200 OK` with the header `Docker-Distribution-Api-Version: registry/2.0`, confirming the registry is serving the OCI distribution API.

### Retention policy summary

| Repository pattern | TTL | Keep count | Purpose |
|---|---|---|---|
| `build-cache/podman/**` | 7 days | 3 most recent | Dockerfile layer cache |
| `build-cache/cnb/**` | 7 days | 3 most recent | Buildpack layer cache |
| `**` (catch-all) | 3 days | 1 most recent | Everything else |

A tag is only deleted when **all** retention conditions are false (TTL expired AND not recently pulled AND outside the keep count).

### k3d clusters

For local k3d clusters, you can use a smaller 5Gi PVC to save disk space. To do this, change `storage: 10Gi` to `storage: 5Gi` in the PVC section of step 1.2 **before** applying it for the first time. PVC size cannot be reduced after creation.

---

## Step 2: Update Workflow Templates

Apply the updated ClusterWorkflowTemplates that add `build-cache` and `enable-cache` parameters and cache-aware build logic.

### 2.1 containerfile-build (Dockerfile / Podman)

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
          - name: enable-cache
            default: "true"
      volumes:
        - name: storage
          emptyDir:
            sizeLimit: 10Gi
        - name: tools
          emptyDir: {}
      initContainers:
        - name: install-tools
          image: ghcr.io/openchoreo/ci-tools:1.0
          command:
            - sh
            - -c
            - |
              mkdir -p /tools/bin /tools/lib
              cp /ci-tools/bin/* /tools/bin/
              cp /ci-tools/lib/* /tools/lib/
          volumeMounts:
            - name: tools
              mountPath: /tools
      container:
        image: quay.io/podman/stable:v5.8.2
        command:
          - sh
          - -c
        args:
          - |-
            set -e

            export PATH="/tools/bin:${PATH}"
            export LD_LIBRARY_PATH="/tools/lib"

            WORKDIR="/mnt/vol/source"
            IMAGE="{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-{{inputs.parameters.git-revision}}"
            DOCKER_CONTEXT="{{workflow.parameters.docker-context}}"
            DOCKERFILE_PATH="{{workflow.parameters.dockerfile-path}}"
            BUILD_ENV_JSON='{{inputs.parameters.build-env}}'
            BUILD_ARGS_JSON='{{inputs.parameters.build-args}}'

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

            mkdir -p /storage/run /storage/graph
            cat > /etc/containers/storage.conf <<STEOF
            [storage]
            driver = "overlay"
            runroot = "/storage/run"
            graphroot = "/storage/graph"
            [storage.options.overlay]
            STEOF

            # Build --env and --build-arg flags
            ENV_ARGS=""
            if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
              ENV_ARGS=$(echo "$BUILD_ENV_JSON" | \
                jq -r '.[] | "--env \(.name)=\(.value)"' | \
                tr '\n' ' ')
            fi

            BUILD_ARG_ARGS=""
            if [ -n "$BUILD_ARGS_JSON" ] && [ "$BUILD_ARGS_JSON" != "[]" ]; then
              BUILD_ARG_ARGS=$(echo "$BUILD_ARGS_JSON" | \
                jq -r '.[] | "--build-arg \(.name)=\(.value)"' | \
                tr '\n' ' ')
            fi

            # Build cache configuration
            CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
            ENABLE_CACHE="{{inputs.parameters.enable-cache}}"
            IMAGE_NAME="{{workflow.parameters.image-name}}"
            CACHE_ARGS=""

            if [ "$ENABLE_CACHE" = "true" ] && [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
              if env -u LD_LIBRARY_PATH /usr/bin/curl -sf --connect-timeout 3 "http://${CACHE_REGISTRY}/v2/" >/dev/null 2>&1; then
                CACHE_REF="${CACHE_REGISTRY}/build-cache/podman/${IMAGE_NAME}"
                CACHE_ARGS="--layers --cache-from=${CACHE_REF} --cache-to=${CACHE_REF} --cache-ttl=168h"

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
            else
              echo ">> Cache not enabled, building without cache"
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
          - mountPath: /tools
            name: tools
            readOnly: true
EOF
```

### 2.2 paketo-buildpacks-build

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
          - name: enable-cache
            default: "false"
      volumes:
        - name: storage
          emptyDir:
            sizeLimit: 10Gi
      container:
        image: quay.io/podman/stable:v5.8.2
        command:
          - sh
          - -c
        args:
          - |-
            set -e

            dnf install -y --setopt=install_weak_deps=False jq > /dev/null 2>&1

            WORKDIR=/mnt/vol/source
            GIT_REVISION="{{inputs.parameters.git-revision}}"

            IMAGE="{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-${GIT_REVISION}"
            APP_PATH="{{workflow.parameters.app-path}}"
            BUILD_ENV_JSON='{{workflow.parameters.build-env}}'

            echo ">> Image: $IMAGE"
            echo ">> App path: $APP_PATH"

            if [ ! -d "$WORKDIR/$APP_PATH" ]; then
              echo ">> Error: The specified application path '$APP_PATH' does not exist in the repository"
              echo ">> Hint: Verify that the application path points to a valid directory in your repository."
              echo ">> Repository contents:"
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

            echo ">> Initializing build environment"
            mkdir -p /run/podman
            podman system service --time=0 unix:///run/podman/podman.sock &
            until podman info --format '{{.Host.RemoteSocket.Exists}}' 2>/dev/null | grep -q true; do sleep 1; done
            export DOCKER_HOST=unix:///run/podman/podman.sock

            BUILDER="docker.io/paketobuildpacks/builder-jammy-full:0.3.603"
            RUN_IMG="docker.io/paketobuildpacks/run-jammy-full:0.1.130"

            # Install pack CLI
            echo ">> Installing pack CLI"
            PACK_VERSION="v0.40.1"
            curl -fsSL "https://github.com/buildpacks/pack/releases/download/${PACK_VERSION}/pack-${PACK_VERSION}-linux.tgz" | tar xz -C /tmp
            chmod +x /tmp/pack

            # Build --env flags
            ENV_ARGS=""
            if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
              ENV_ARGS=$(echo "$BUILD_ENV_JSON" | \
                jq -r '.[] | "--env \(.name)=\(.value)"' | \
                tr '\n' ' ')
            fi

            # Build cache configuration
            CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
            ENABLE_CACHE="{{inputs.parameters.enable-cache}}"
            IMAGE_NAME="{{workflow.parameters.image-name}}"
            IMAGE_TAG="{{workflow.parameters.image-tag}}"

            if [ "$ENABLE_CACHE" = "true" ] && [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
              CACHE_PROBE="false"
              if command -v wget >/dev/null 2>&1; then
                wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null && CACHE_PROBE="true"
              elif command -v curl >/dev/null 2>&1; then
                curl -sf --max-time 3 -o /dev/null "http://${CACHE_REGISTRY}/v2/" 2>/dev/null && CACHE_PROBE="true"
              else
                echo ">> No probe tool (wget/curl), assuming cache registry available"
                CACHE_PROBE="true"
              fi

              if [ "$CACHE_PROBE" = "true" ]; then
                CACHE_IMAGE="${CACHE_REGISTRY}/build-cache/cnb/${IMAGE_NAME}:cnb-cache"
                PUBLISH_REF="${CACHE_REGISTRY}/build-cache/cnb/${IMAGE_NAME}:${IMAGE_TAG}-${GIT_REVISION}"

                mkdir -p /etc/containers/registries.conf.d
                cat > /etc/containers/registries.conf.d/cache.conf <<REGEOF
            [[registry]]
            location = "${CACHE_REGISTRY}"
            insecure = true
            REGEOF

                echo ">> CNB layer cache: ${CACHE_IMAGE}"
                echo ">> Publishing build image to Zot: ${PUBLISH_REF}"
                /tmp/pack build "$PUBLISH_REF" \
                  --builder "$BUILDER" \
                  --run-image "$RUN_IMG" \
                  --path "$WORKDIR/$APP_PATH" \
                  --pull-policy always \
                  --publish \
                  --insecure-registry "$CACHE_REGISTRY" \
                  --cache-image "$CACHE_IMAGE" \
                  $ENV_ARGS

                echo ">> Pulling published image back from Zot"
                podman pull --tls-verify=false "$PUBLISH_REF"
                podman tag "$PUBLISH_REF" "$IMAGE"
                podman save -o /mnt/vol/app-image.tar "$IMAGE"
                echo ">> Image built and saved successfully"
                exit 0
              else
                echo ">> Cache registry not reachable, building without cache"
              fi
            else
              echo ">> Cache not enabled, building without cache"
            fi

            echo ">> Building image with Paketo buildpacks"
            /tmp/pack build "$IMAGE" \
              --builder "$BUILDER" \
              --run-image "$RUN_IMG" \
              --docker-host inherit \
              --path "$WORKDIR/$APP_PATH" \
              --pull-policy always \
              $ENV_ARGS

            echo ">> Image built successfully"
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

### 2.3 gcp-buildpacks-build

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
          - name: enable-cache
            default: "false"
      volumes:
        - name: storage
          emptyDir:
            sizeLimit: 10Gi
      container:
        image: quay.io/podman/stable:v5.8.2
        command:
          - sh
          - -c
        args:
          - |-
            set -e

            dnf install -y --setopt=install_weak_deps=False jq > /dev/null 2>&1

            WORKDIR=/mnt/vol/source
            GIT_REVISION="{{inputs.parameters.git-revision}}"

            IMAGE="{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-${GIT_REVISION}"
            APP_PATH="{{workflow.parameters.app-path}}"
            BUILD_ENV_JSON='{{workflow.parameters.build-env}}'

            echo ">> Image: $IMAGE"
            echo ">> App path: $APP_PATH"

            if [ ! -d "$WORKDIR/$APP_PATH" ]; then
              echo ">> Error: The specified application path '$APP_PATH' does not exist in the repository"
              echo ">> Hint: Verify that the application path points to a valid directory in your repository."
              echo ">> Repository contents:"
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

            echo ">> Initializing build environment"
            mkdir -p /run/podman
            podman system service --time=0 unix:///run/podman/podman.sock &
            until podman info --format '{{.Host.RemoteSocket.Exists}}' 2>/dev/null | grep -q true; do sleep 1; done
            export DOCKER_HOST=unix:///run/podman/podman.sock

            BUILDER="gcr.io/buildpacks/builder@sha256:5977b4bd47d3e9ff729eefe9eb99d321d4bba7aa3b14986323133f40b622aef1"
            RUN_IMG="gcr.io/buildpacks/google-22/run@sha256:a8ccb6641b4d98b0adf6397f954e7194611d1ae61310f0561f1c00fdf7f9ba96"

            # Install pack CLI
            echo ">> Installing pack CLI"
            PACK_VERSION="v0.40.1"
            curl -fsSL "https://github.com/buildpacks/pack/releases/download/${PACK_VERSION}/pack-${PACK_VERSION}-linux.tgz" | tar xz -C /tmp
            chmod +x /tmp/pack

            # Build --env flags
            ENV_ARGS=""
            if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
              ENV_ARGS=$(echo "$BUILD_ENV_JSON" | \
                jq -r '.[] | "--env \(.name)=\(.value)"' | \
                tr '\n' ' ')
            fi

            # Build cache configuration
            CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
            ENABLE_CACHE="{{inputs.parameters.enable-cache}}"
            IMAGE_NAME="{{workflow.parameters.image-name}}"
            IMAGE_TAG="{{workflow.parameters.image-tag}}"

            if [ "$ENABLE_CACHE" = "true" ] && [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
              CACHE_PROBE="false"
              if command -v wget >/dev/null 2>&1; then
                wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null && CACHE_PROBE="true"
              elif command -v curl >/dev/null 2>&1; then
                curl -sf --max-time 3 -o /dev/null "http://${CACHE_REGISTRY}/v2/" 2>/dev/null && CACHE_PROBE="true"
              else
                echo ">> No probe tool (wget/curl), assuming cache registry available"
                CACHE_PROBE="true"
              fi

              if [ "$CACHE_PROBE" = "true" ]; then
                CACHE_IMAGE="${CACHE_REGISTRY}/build-cache/cnb/${IMAGE_NAME}:cnb-cache"
                PUBLISH_REF="${CACHE_REGISTRY}/build-cache/cnb/${IMAGE_NAME}:${IMAGE_TAG}-${GIT_REVISION}"

                mkdir -p /etc/containers/registries.conf.d
                cat > /etc/containers/registries.conf.d/cache.conf <<REGEOF
            [[registry]]
            location = "${CACHE_REGISTRY}"
            insecure = true
            REGEOF

                echo ">> CNB layer cache: ${CACHE_IMAGE}"
                echo ">> Publishing build image to Zot: ${PUBLISH_REF}"
                /tmp/pack build "$PUBLISH_REF" \
                  --builder "$BUILDER" \
                  --run-image "$RUN_IMG" \
                  --path "$WORKDIR/$APP_PATH" \
                  --pull-policy always \
                  --publish \
                  --insecure-registry "$CACHE_REGISTRY" \
                  --cache-image "$CACHE_IMAGE" \
                  $ENV_ARGS

                echo ">> Pulling published image back from Zot"
                podman pull --tls-verify=false "$PUBLISH_REF"
                podman tag "$PUBLISH_REF" "$IMAGE"
                podman save -o /mnt/vol/app-image.tar "$IMAGE"
                echo ">> Image built and saved successfully"
                exit 0
              else
                echo ">> Cache registry not reachable, building without cache"
              fi
            else
              echo ">> Cache not enabled, building without cache"
            fi

            echo ">> Building image with GCP buildpacks"
            /tmp/pack build "$IMAGE" \
              --builder "$BUILDER" \
              --run-image "$RUN_IMG" \
              --docker-host inherit \
              --path "$WORKDIR/$APP_PATH" \
              --pull-policy always \
              $ENV_ARGS

            echo ">> Image built successfully"
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

### 2.4 ballerina-buildpack-build

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
          - name: enable-cache
            default: "false"
      volumes:
        - name: storage
          emptyDir:
            sizeLimit: 10Gi
        - name: app-dir
          emptyDir: {}
      container:
        image: quay.io/podman/stable:v5.8.2
        command:
          - sh
          - -c
        args:
          - |-
            set -e

            dnf install -y --setopt=install_weak_deps=False jq > /dev/null 2>&1

            WORKDIR=/mnt/vol/source
            GIT_REVISION="{{inputs.parameters.git-revision}}"

            IMAGE="{{workflow.parameters.image-name}}:{{workflow.parameters.image-tag}}-${GIT_REVISION}"
            APP_PATH="{{workflow.parameters.app-path}}"
            BUILD_ENV_JSON='{{inputs.parameters.build-env}}'

            echo ">> Image: $IMAGE"
            echo ">> App path: $APP_PATH"

            if [ ! -d "$WORKDIR/$APP_PATH" ]; then
              echo ">> Error: The specified application path '$APP_PATH' does not exist in the repository"
              echo ">> Hint: Verify that the application path points to a valid directory in your repository."
              echo ">> Repository contents:"
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

            echo ">> Initializing build environment"
            mkdir -p /run/podman
            podman system service --time=0 unix:///run/podman/podman.sock &
            until podman info --format '{{.Host.RemoteSocket.Exists}}' 2>/dev/null | grep -q true; do sleep 1; done
            export DOCKER_HOST=unix:///run/podman/podman.sock

            BUILDER="ghcr.io/openchoreo/buildpack/ballerina:18"
            RUN_IMAGE="ghcr.io/openchoreo/buildpack/ballerina:18-run"

            # Install pack CLI
            echo ">> Installing pack CLI"
            PACK_VERSION="v0.40.1"
            curl -fsSL "https://github.com/buildpacks/pack/releases/download/${PACK_VERSION}/pack-${PACK_VERSION}-linux.tgz" | tar xz -C /tmp
            chmod +x /tmp/pack

            # Build --env flags
            ENV_ARGS=""
            if [ -n "$BUILD_ENV_JSON" ] && [ "$BUILD_ENV_JSON" != "[]" ]; then
              ENV_ARGS=$(echo "$BUILD_ENV_JSON" | \
                jq -r '.[] | "--env \(.name)=\(.value)"' | \
                tr '\n' ' ')
            fi

            # Build cache configuration
            CACHE_REGISTRY="{{inputs.parameters.build-cache}}"
            ENABLE_CACHE="{{inputs.parameters.enable-cache}}"
            IMAGE_NAME="{{workflow.parameters.image-name}}"
            IMAGE_TAG="{{workflow.parameters.image-tag}}"

            if [ "$ENABLE_CACHE" = "true" ] && [ -n "$CACHE_REGISTRY" ] && [ "$CACHE_REGISTRY" != "none" ]; then
              CACHE_PROBE="false"
              if command -v wget >/dev/null 2>&1; then
                wget -q --spider --timeout=3 "http://${CACHE_REGISTRY}/v2/" 2>/dev/null && CACHE_PROBE="true"
              elif command -v curl >/dev/null 2>&1; then
                curl -sf --max-time 3 -o /dev/null "http://${CACHE_REGISTRY}/v2/" 2>/dev/null && CACHE_PROBE="true"
              else
                echo ">> No probe tool (wget/curl), assuming cache registry available"
                CACHE_PROBE="true"
              fi

              if [ "$CACHE_PROBE" = "true" ]; then
                CACHE_IMAGE="${CACHE_REGISTRY}/build-cache/cnb/${IMAGE_NAME}:cnb-cache"
                PUBLISH_REF="${CACHE_REGISTRY}/build-cache/cnb/${IMAGE_NAME}:${IMAGE_TAG}-${GIT_REVISION}"

                mkdir -p /etc/containers/registries.conf.d
                cat > /etc/containers/registries.conf.d/cache.conf <<REGEOF
            [[registry]]
            location = "${CACHE_REGISTRY}"
            insecure = true
            REGEOF

                echo ">> CNB layer cache: ${CACHE_IMAGE}"
                echo ">> Publishing build image to Zot: ${PUBLISH_REF}"
                /tmp/pack build "$PUBLISH_REF" \
                  --builder "$BUILDER" \
                  --run-image "$RUN_IMAGE" \
                  --path "$WORKDIR/$APP_PATH" \
                  --volume "/mnt/vol:/app/generated-artifacts:rw" \
                  --pull-policy always \
                  --publish \
                  --insecure-registry "$CACHE_REGISTRY" \
                  --cache-image "$CACHE_IMAGE" \
                  $ENV_ARGS

                echo ">> Pulling published image back from Zot"
                podman pull --tls-verify=false "$PUBLISH_REF"
                podman tag "$PUBLISH_REF" "$IMAGE"
                podman save -o /mnt/vol/app-image.tar "$IMAGE"
                echo ">> Image built and saved successfully"
                exit 0
              else
                echo ">> Cache registry not reachable, building without cache"
              fi
            else
              echo ">> Cache not enabled, building without cache"
            fi

            echo ">> Building image with Ballerina buildpack"
            /tmp/pack build "$IMAGE" \
              --builder "$BUILDER" \
              --run-image "$RUN_IMAGE" \
              --docker-host inherit \
              --path "$WORKDIR/$APP_PATH" \
              --volume "/mnt/vol:/app/generated-artifacts:rw" \
              --pull-policy always \
              $ENV_ARGS

            echo ">> Image built successfully"
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

---

## Step 3: Update CI Workflows

Patch the existing ClusterWorkflows to add the `enableCache` parameter and wire it through to the build templates. Each workflow needs three changes:

1. Add `enableCache` boolean property to the developer-facing schema
2. Add `enable-cache` to the Argo workflow arguments (bridges OpenChoreo parameter to Argo)
3. Pass `enable-cache` to the build step

> **Requires:** [yq v4](https://github.com/mikefarah/yq) (`brew install yq` or `go install github.com/mikefarah/yq/v4@latest`)

### 3.1 Patch all four CI workflows

The same three mutations apply to every workflow. Run this from the OpenChoreo repository root:

```bash
CACHE_DESC="Enable registry-based build layer caching. Requires a build-cache registry deployed in the workflow plane namespace."

for workflow in dockerfile-builder paketo-buildpacks-builder gcp-buildpacks-builder ballerina-buildpack-builder; do
  kubectl get clusterworkflow "$workflow" -o yaml | \
    yq '
      # Add enableCache schema property
      .spec.parameters.openAPIV3Schema.properties.enableCache = {
        "type": "boolean",
        "default": false,
        "description": "'"$CACHE_DESC"'"
      } |

      # Remove old enable-cache entries (idempotent) then add fresh ones
      .spec.runTemplate.spec.arguments.parameters |=
        [.[] | select(.name != "enable-cache")] + [{"name": "enable-cache", "value": "${parameters.enableCache}"}] |
      .spec.runTemplate.spec.templates[0].steps[1][0].arguments.parameters |=
        [.[] | select(.name != "enable-cache")] + [{"name": "enable-cache", "value": "{{workflow.parameters.enable-cache}}"}]
    ' | kubectl apply -f -
  echo "Patched $workflow"
done
```

### 3.2 What the patch does

For each ClusterWorkflow, the `yq` expression makes three additions:

**1. Schema property** -- adds `enableCache` to `spec.parameters.openAPIV3Schema.properties`:

```yaml
enableCache:
  type: boolean
  default: false
  description: "Enable registry-based build layer caching. ..."
```

**2. Argo workflow argument** -- adds to `spec.runTemplate.spec.arguments.parameters`:

```yaml
- name: enable-cache
  value: ${parameters.enableCache}
```

**3. Build step argument** -- adds to the `build-image` step's `arguments.parameters`:

```yaml
- name: enable-cache
  value: "{{workflow.parameters.enable-cache}}"
```

### 3.3 Verify the patches

```bash
for workflow in dockerfile-builder paketo-buildpacks-builder gcp-buildpacks-builder ballerina-buildpack-builder; do
  echo "=== $workflow ==="
  kubectl get clusterworkflow "$workflow" -o yaml | yq '.spec.parameters.openAPIV3Schema.properties.enableCache'
  echo ""
done
```

Each should show:

```yaml
type: boolean
default: false
description: "Enable registry-based build layer caching. ..."
```

---

## Step 4: Enable Caching on a Component

By default, `enableCache` is `false` (caching disabled). To enable caching for a component, set `enableCache: true` in the component's workflow parameters:

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Component
metadata:
  name: my-service
  namespace: default
spec:
  # ... other fields ...
  workflow:
    kind: ClusterWorkflow
    name: dockerfile-builder
  parameters:
    enableCache: true
```

Or update an existing component:

```bash
kubectl patch component my-service -n default --type merge -p '{"spec":{"parameters":{"enableCache":true}}}'
```

---

## Step 5: Verify Caching

### 5.1 Check cache registry health

```bash
kubectl -n openchoreo-workflow-plane port-forward svc/build-cache 5100:5100 &
sleep 2
curl -s http://localhost:5100/v2/_catalog
kill %1
```

After a cached build completes, you should see repositories like:

```json
{"repositories":["build-cache/podman/default-myproject-myservice"]}
```

### 5.2 Compare build times

1. Trigger a build with `enableCache: true` -- this first build populates the cache.
2. Trigger a second build of the same component without code changes.
3. The second build should be noticeably faster (cached layers are reused).

### 5.3 Check build logs

Look for these log lines in the build step output:

**Cache enabled:**
```
>> Layer cache: build-cache.openchoreo-workflow-plane.svc.cluster.local:5100/build-cache/podman/<component>
```

**Cache not enabled (enableCache: false / default):**
```
>> Cache not enabled, building without cache
```

**Cache registry not deployed:**
```
>> Cache registry not reachable, building without cache
```

---

## Disable / Uninstall

### Skip cache for a single build

Set `enableCache: false` (or omit it -- defaults to `false`) on the component. The existing cache stays in the registry for the next build.

### Remove the cache registry

```bash
kubectl delete deployment build-cache -n openchoreo-workflow-plane
kubectl delete service build-cache -n openchoreo-workflow-plane
kubectl delete configmap build-cache-config -n openchoreo-workflow-plane
kubectl delete pvc build-cache-data -n openchoreo-workflow-plane
```

Builds continue to work normally -- the templates probe the registry at startup and fall back to building without cache when it is unreachable.

### Restore original workflow templates and CI workflows

To fully revert to the original templates (removing the `build-cache` and `enable-cache` parameters), reapply the originals from the OpenChoreo repository:

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

```
Developer sets enableCache: true on component
  -> ClusterWorkflow schema validates it as boolean
    -> runTemplate maps ${parameters.enableCache} -> Argo param "enable-cache"
      -> build step passes {{workflow.parameters.enable-cache}} -> build template
        -> template script: if ENABLE_CACHE = "true" -> probe cache registry -> use cache
```

### Cache reference paths

| Build type | Cache ref pattern |
|---|---|
| Dockerfile (Podman) | `build-cache/podman/<namespace-project-component>` |
| Buildpacks (Pack CLI) | `build-cache/cnb/<namespace-project-component>:cnb-cache` |

Each component gets its own isolated cache namespace based on its identity (`<namespace>-<project>-<component>`).

### Caching mechanisms

| Builder | Cache flags (when `enableCache: true`) |
|---|---|
| Podman (Dockerfile) | `--layers --cache-from=<ref> --cache-to=<ref> --cache-ttl=168h` |
| Pack CLI (Buildpacks) | `--publish --insecure-registry=<registry> --cache-image=<ref>`, then `podman pull/tag/save` |

For Podman, the cache ref is intentionally an untagged repository. Podman writes
content-addressed cache tags under that repository and rejects a tagged `--cache-to`
reference such as `:buildcache`.

For Pack CLI, registry cache requires `--publish`. The template publishes the built
image to Zot under `build-cache/cnb/<component>:<image-tag>-<git-revision>`, pulls it
back into Podman with `--tls-verify=false`, tags it as the expected workflow image,
and saves `/mnt/vol/app-image.tar` for the rest of the pipeline.

### Retention and cleanup

Zot handles cleanup automatically via online garbage collection:
- GC runs every 6 hours (configurable)
- Tags older than the TTL with no recent pulls and outside the keep count are deleted
- GC runs online -- no downtime, no CronJob required
