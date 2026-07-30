# Improve Workflow Template Logs

## Goal

Improve the CI workflow template shell scripts so that failures produce clear, actionable log messages instead of cryptic tool errors. Add upfront parameter validation and structured logging to each step.

## Relevant Files

- **Workflow templates (shell scripts to modify):** `samples/getting-started/workflow-templates/*`
- **CI workflows (compose the templates):** `samples/getting-started/ci-workflows/*`

## Pipeline Flow

All CI workflows follow the same 4-step pipeline:

```
checkout-source  -->  build-image  -->  publish-image  -->  generate-workload
```

## Analysis: Current Gaps Per Template

### 1. `checkout-source.yaml` (checkout step)

**Parameters:** `git-repo`, `branch`, `commit`, `git-secret`

| Parameter  | Current Behavior on Bad Input | Improvement |
|------------|-------------------------------|-------------|
| `git-repo` | Empty string causes `git clone ""` -> cryptic git error | Validate non-empty before cloning; log the repo URL being used |
| `branch`   | Empty string causes `git clone --branch ""` -> cryptic error | Validate non-empty (when `commit` is also empty); log branch name |
| `commit`   | Invalid SHA causes `git fetch origin <bad-sha>` -> opaque fetch error | Log the commit SHA; wrap fetch failure with "Commit SHA not found in repository" |

**Additional logging to add:**
- Log all resolved parameters at the start (repo URL with credentials masked, branch, commit)
- Log whether using commit-based or branch-based checkout
- Log the cloned directory listing (`ls` of source root) to help users confirm app structure

### 2. `containerfile-build.yaml` (Dockerfile build step)

**Parameters:** `image-name`, `image-tag`, `docker-context`, `dockerfile-path`, `build-env`, `build-args`

| Parameter | Current Behavior on Bad Input | Improvement |
|-----------|-------------------------------|-------------|
| `dockerfile-path` | `podman build -f` on missing file -> "Error: no Containerfile or Dockerfile" | Check `$WORKDIR/$DOCKERFILE_PATH` exists; log clear message: "Dockerfile not found at path: X. Check the 'docker.filePath' parameter." |
| `docker-context` | Missing dir -> "Error: no context directory" | Check `$WORKDIR/$DOCKER_CONTEXT` is a valid directory |

**Additional logging to add:**
- Log resolved image name, Dockerfile path, build context at the start
- Log number of build-env and build-args being applied
- Log the `podman build` command being executed (for debugging)

### 3. `ballerina-buildpack-build.yaml` / `gcp-buildpacks-build.yaml` / `paketo-buildpacks-build.yaml` (buildpack build steps)

**Parameters:** `image-name`, `image-tag`, `app-path`, `build-env`

| Parameter | Current Behavior on Bad Input | Improvement |
|-----------|-------------------------------|-------------|
| `app-path` | `pack build --path` on missing dir -> opaque buildpack error | Check `$WORKDIR/$APP_PATH` exists; log: "Application path not found: X. Check the 'repository.appPath' parameter." |

**Additional logging to add:**
- Log resolved image name and app path at the start
- Log the builder image version being used
- Log number of build-env vars being applied
- List contents of the app path directory to help diagnose missing source files

### 4. `publish-image.yaml` / `publish-image-k3d.yaml` (image push step)

**Parameters:** `image-name`, `image-tag`, `git-revision`, `registry-push-secret`

| Parameter | Current Behavior on Bad Input | Improvement |
|-----------|-------------------------------|-------------|
| Image tar | If build step produced no tar -> `podman load` fails with cryptic I/O error | Check `/mnt/vol/app-image.tar` exists before `podman load` |

**Additional logging to add:**
- Log the source and destination image tags
- Log whether registry auth is being used
- Log the registry push endpoint

### 5. `generate-workload.yaml` / `generate-workload-k3d.yaml` (workload generation step)

**Parameters:** `image`, `run-name`, `project-name`, `component-name`, `namespace-name`, `app-path`, OAuth params

This template already has reasonable logging. Minor improvements:

- Validate `DESCRIPTOR_PATH` directory exists before attempting occ workload create
- Log the full API URL being used for each HTTP call (already partially done)
- On OAuth failure, suggest checking IDP configuration

## Implementation Approach

**Where to validate: in each template's own script, before the main operation.** Add a validation block after variable assignment and before the main logic. Pattern:

```sh
# --- Parameter Validation ---
echo "=== Step: <step-name> ==="
echo "Parameter: value"
...

if [ -z "$REQUIRED_PARAM" ]; then
  echo "Error: <parameter-name> is required but was not provided."
  echo "Hint: Check the '<schema-field-name>' field in your component's workflow parameters."
  exit 1
fi

if [ ! -d "$EXPECTED_PATH" ]; then
  echo "Error: Directory not found: $EXPECTED_PATH"
  echo "Hint: Verify the '<schema-field-name>' parameter points to a valid directory in your repository."
  echo "Repository contents:"
  ls -la /mnt/vol/source/
  exit 1
fi
```

## Priority

1. **checkout-source.yaml** — Most failures start here (wrong repo URL, bad branch, bad credentials). Highest impact.
2. **Build templates** (containerfile, ballerina, gcp, paketo) — App path validation catches the most common misconfiguration.
3. **publish-image** — Lower priority, failures here are rarer.
4. **generate-workload** — Already has decent logging.
