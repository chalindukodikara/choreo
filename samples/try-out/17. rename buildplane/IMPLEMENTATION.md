# Implementation: Rename BuildPlane to WorkflowPlane

## Overview

Mechanical rename of `BuildPlane` -> `WorkflowPlane` and `ClusterBuildPlane` -> `ClusterWorkflowPlane` across the entire OpenChoreo codebase. Zero remaining `BuildPlane` references in Go source files. All Go variable names (`bp` -> `wp`, `cbp` -> `cwp`, `bpClientMgr` -> `wpClientMgr`, `bpClient` -> `wpClient`) have been updated. All Makefile/script abbreviations (`BP` -> `WP`), test fixture data, error messages, CoreDNS config, and documentation have been updated. All binaries compile, `go fmt` and `go vet` pass cleanly.

## Files Changed

### Step 1: CRD Type Definitions (api/v1alpha1/)

**New files (created from old, with renames applied):**
- `api/v1alpha1/workflowplane_types.go` (was `buildplane_types.go`)
- `api/v1alpha1/clusterworkflowplane_types.go` (was `clusterbuildplane_types.go`)

**Deleted files:**
- `api/v1alpha1/buildplane_types.go`
- `api/v1alpha1/clusterbuildplane_types.go`

**Modified files:**
- `api/v1alpha1/types.go` — `BuildPlaneRef` -> `WorkflowPlaneRef`, `ClusterBuildPlaneRef` -> `ClusterWorkflowPlaneRef`, enum constants, `ClusterObservabilityPlaneRef` comment
- `api/v1alpha1/workflow_types.go` — `BuildPlaneRef` field -> `WorkflowPlaneRef`, JSON tag `buildPlaneRef` -> `workflowPlaneRef`
- `api/v1alpha1/clusterworkflow_types.go` — `BuildPlaneRef` field -> `WorkflowPlaneRef`, JSON tag updated

**Generated (by `make generate manifests`):**
- `api/v1alpha1/zz_generated.deepcopy.go` — auto-regenerated
- `config/crd/bases/openchoreo.dev_workflowplanes.yaml` (was `_buildplanes.yaml`)
- `config/crd/bases/openchoreo.dev_clusterworkflowplanes.yaml` (was `_clusterbuildplanes.yaml`)
- `config/crd/bases/openchoreo.dev_workflows.yaml` — updated `buildPlaneRef` -> `workflowPlaneRef`
- `config/crd/bases/openchoreo.dev_clusterworkflows.yaml` — same

### Step 2: Controllers

**Renamed directories:**
- `internal/controller/buildplane/` -> `internal/controller/workflowplane/`
- `internal/controller/clusterbuildplane/` -> `internal/controller/clusterworkflowplane/`

**Files updated in renamed dirs (all BuildPlane -> WorkflowPlane in package names, types, functions, comments):**
- `internal/controller/workflowplane/controller.go`
- `internal/controller/workflowplane/controller_test.go`
- `internal/controller/workflowplane/suite_test.go`
- `internal/controller/clusterworkflowplane/controller.go`
- `internal/controller/clusterworkflowplane/controller_test.go`
- `internal/controller/clusterworkflowplane/suite_test.go`

**Modified files:**
- `internal/controller/reference.go` — `BuildPlaneResult` -> `WorkflowPlaneResult`, `ResolveBuildPlane` -> `ResolveWorkflowPlane`, `GetObservabilityPlane*OfBuildPlane` -> `*OfWorkflowPlane`, all variable names and comments
- `internal/controller/reference_test.go` — matching renames, test fixture names `bp`/`cbp` -> `wp`/`cwp`
- `internal/controller/workflowrun/controller.go` — BuildPlane references
- `internal/controller/workflowrun/run_engine.go` — BuildPlane references, `bpClient` -> `wpClient`
- `internal/controller/workflowrun/controller_finalize.go` — `bpClient` -> `wpClient`
- `internal/controller/workflowrun/controller_conditions.go` — condition reason strings
- `internal/controller/workflowrun/controller_integration_test.go` — test name `no-bp` -> `no-wp`
- `cmd/main.go` — import paths, controller setup blocks

### Step 3: Webhooks

No webhook files existed for BuildPlane. No changes needed.

### Step 4: OpenAPI Spec and Generated API Code

**Modified:**
- `openapi/openchoreo-api.yaml` — all paths `/buildplanes` -> `/workflowplanes`, `/clusterbuildplanes` -> `/clusterworkflowplanes`, all schema definitions, parameter definitions, operation IDs, tag names, descriptions

**Generated (by `make openapi-codegen`):**
- `internal/openchoreo-api/api/gen/models.gen.go`
- `internal/openchoreo-api/api/gen/server.gen.go`
- `internal/openchoreo-api/api/gen/client.gen.go`

### Step 5: API Handlers

**Renamed files:**
- `internal/openchoreo-api/handlers/buildplanes.go` -> `workflowplanes.go`
- `internal/openchoreo-api/handlers/clusterbuildplanes.go` -> `clusterworkflowplanes.go`
- `internal/openchoreo-api/api/handlers/buildplanes.go` -> `workflowplanes.go`
- `internal/openchoreo-api/api/handlers/clusterbuildplanes.go` -> `clusterworkflowplanes.go`

**Modified:**
- `internal/openchoreo-api/handlers/handlers.go` — URL paths and handler names
- `internal/openchoreo-api/handlers/git_secrets.go` — error message "Build plane" -> "Workflow plane"
- `internal/openchoreo-api/api/handlers/clusterworkflowplanes.go` — `cbp` -> `cwp`, `genCBP` -> `genCWP`

### Step 6: Services

**Renamed directories:**
- `internal/openchoreo-api/services/buildplane/` -> `workflowplane/`
- `internal/openchoreo-api/services/clusterbuildplane/` -> `clusterworkflowplane/`

**Files updated in renamed dirs:**
- `service.go`, `interface.go`, `service_authz.go`, `errors.go` — all struct/method/error names, `bp` -> `wp` / `cbp` -> `cwp` variable names

**Modified:**
- `internal/openchoreo-api/services/handlerservices/services.go` — import paths, struct fields, constructors, `bpClientMgr` -> `wpClientMgr`
- `internal/openchoreo-api/services/gitsecret/service.go` — `bpClientMgr` -> `wpClientMgr`
- `internal/openchoreo-api/services/gitsecret/service_authz.go` — `bpClientMgr` -> `wpClientMgr`
- `internal/openchoreo-api/services/workflowrun/service.go` — `bpClientMgr` -> `wpClientMgr`, `bpClient` -> `wpClient`
- `internal/openchoreo-api/services/workflowrun/service_authz.go` — `bpClientMgr` -> `wpClientMgr`

### Step 7: Legacy Services

**Renamed files:**
- `internal/openchoreo-api/legacyservices/buildplane_service.go` -> `workflowplane_service.go`
- `internal/openchoreo-api/legacyservices/clusterbuildplane_service.go` -> `clusterworkflowplane_service.go`

**Modified:**
- `internal/openchoreo-api/legacyservices/services.go` — struct fields, constructors
- `internal/openchoreo-api/legacyservices/constants.go` — resource/action constants
- `internal/openchoreo-api/legacyservices/errors.go` — error definitions
- `internal/openchoreo-api/legacyservices/workflowrun_service.go` — `buildPlaneService` field -> `workflowPlaneService`, `bpClient` -> `wpClient`
- `internal/openchoreo-api/legacyservices/workflowrun_service_test.go` — matching renames
- `internal/openchoreo-api/legacyservices/gitsecret_service.go` — field and error reference, `bpClientMgr` -> `wpClientMgr`
- `internal/openchoreo-api/legacyservices/workflowplane_service.go` — `bpClientMgr` -> `wpClientMgr`, `bp` -> `wp` in `toWorkflowPlaneResponse` and `ArgoWorkflowExists`
- `internal/openchoreo-api/legacyservices/clusterworkflowplane_service.go` — `cbp` -> `cwp` in `toClusterWorkflowPlaneResponse`

### Step 8: MCP Handlers

**Renamed files:**
- `internal/openchoreo-api/legacymcphandlers/buildplanes.go` -> `workflowplanes.go`
- `internal/openchoreo-api/legacymcphandlers/clusterbuildplanes.go` -> `clusterworkflowplanes.go`

**Modified:**
- `internal/openchoreo-api/mcphandlers/infrastructure.go` — function names, service calls, `bp` -> `wp`
- `internal/openchoreo-api/mcphandlers/cluster_resources.go` — function names, service calls
- `internal/openchoreo-api/mcphandlers/transform_resources.go` — function names, `bp` -> `wp`, `cbp` -> `cwp` variable names
- `internal/openchoreo-api/legacymcphandlers/resources.go` — resource type map entries

### Step 9: MCP Tools (pkg/mcp/)

**Modified:**
- `pkg/mcp/tools/types.go` — interface methods
- `pkg/mcp/tools/pe.go` — registration functions
- `pkg/mcp/tools/register.go` — registration calls
- `pkg/mcp/tools/pe_specs_test.go` — test specs
- `pkg/mcp/tools/mock_test.go` — mock methods
- `pkg/mcp/legacytools/types.go`, `infrastructure.go`, `register.go`, `mock_test.go`, `infrastructure_specs_test.go`

### Step 10: CLI Commands

**Renamed directories:**
- `internal/occ/cmd/buildplane/` -> `internal/occ/cmd/workflowplane/`
- `pkg/cli/cmd/buildplane/` -> `pkg/cli/cmd/workflowplane/`

**Modified:**
- `internal/occ/validation/commands.go` — `ResourceBuildPlane` -> `ResourceWorkflowPlane`
- `internal/occ/validation/params.go` — `validateBuildPlaneParams` -> `validateWorkflowPlaneParams`
- `pkg/cli/common/constants/definitions.go` — command definitions, help text
- `internal/occ/cmd/workflowrun/logs.go` — gen type references, comments, `bp` -> `wp`, `cbp` -> `cwp`
- `internal/occ/cmd/workflowplane/workflowplane.go` — `bp` -> `wp` loop variable
- `internal/occ/cmd/apply/registry.go` — gen client method calls
- `internal/occ/resources/client/openapi_client.go` — wrapper function names, gen types

### Step 11: Authorization

**Modified:**
- `internal/authz/core/actions.go` — BuildPlane/ClusterBuildPlane action constants

### Step 12: Labels

**Modified:**
- `internal/labels/labels.go` — `LabelKeyBuildPlane` -> `LabelKeyWorkflowPlane`, `"openchoreo.dev/build-plane"` -> `"openchoreo.dev/workflow-plane"`

### Step 13: Cluster Gateway

**Modified:**
- `internal/cluster-gateway/plane_api.go` — plane type references
- `internal/cluster-gateway/plane_client_ca.go` — function names, type references, comments, `bp` -> `wp`, `cbp` -> `cwp`
- `internal/cluster-gateway/plane_client_ca_test.go` — type references, test fixtures, all `bp`/`cbp`/`shared-bp` -> `wp`/`cwp`/`shared-wp`
- `cmd/cluster-gateway/main.go` — CA loading, client setup
- `cmd/cluster-agent/main.go` — `"build-plane"` -> `"workflow-plane"`
- `internal/clients/gateway/client.go` — BuildPlane references
- `internal/clients/gateway/client_test.go` — `ci-bp` -> `ci-wp`, `build-ns` -> `workflow-ns`

### Step 14: Configuration & RBAC

**Renamed files in `config/rbac/`:**
- `buildplane_{admin,editor,viewer}_role.yaml` -> `workflowplane_*`
- `clusterbuildplane_{admin,editor,viewer}_role.yaml` -> `clusterworkflowplane_*`

**Renamed files in `config/samples/`:**
- `openchoreo_v1alpha1_buildplane.yaml` -> `openchoreo_v1alpha1_workflowplane.yaml`
- `openchoreo_v1alpha1_clusterbuildplane.yaml` -> `openchoreo_v1alpha1_clusterworkflowplane.yaml`

**Modified:**
- `config/rbac/kustomization.yaml`, `config/crd/kustomization.yaml`, `config/samples/kustomization.yaml`
- `config/rbac/role.yaml`

### Step 15: Helm Charts

**Renamed directory:**
- `install/helm/openchoreo-build-plane/` -> `install/helm/openchoreo-workflow-plane/`

**Renamed CRD files in control-plane chart:**
- `crds/openchoreo.dev_buildplanes.yaml` -> `crds/openchoreo.dev_workflowplanes.yaml`
- `crds/openchoreo.dev_clusterbuildplanes.yaml` -> `crds/openchoreo.dev_clusterworkflowplanes.yaml`

**Modified:**
- `install/helm/openchoreo-workflow-plane/Chart.yaml`, `values.yaml`, `values.schema.json`, `_helpers.tpl`, `NOTES.txt`, all templates
- `install/helm/openchoreo-control-plane/templates/controller-manager/controller-manager-role.yaml`
- `install/helm/openchoreo-control-plane/templates/openchoreo-api/clusterrole.yaml`
- `install/helm/openchoreo-control-plane/templates/cluster-gateway/clusterrole.yaml`
- `install/helm/openchoreo-control-plane/values.yaml`
- `install/helm/openchoreo-data-plane/values.yaml`, `values.schema.json`
- `install/helm/openchoreo-observability-plane/values.yaml`, `values.schema.json`

### Step 16: E2E Tests & Makefiles

**Renamed:**
- `test/e2e/k3d/buildplane.yaml` -> `workflowplane.yaml`

**Modified:**
- `make/e2e.mk` — `E2E_BP_*` -> `E2E_WP_*`, `_e2e.install-bp` -> `_e2e.install-wp`
- `make/k3d.mk` — `K3D_BP_NAMESPACE` -> `K3D_WP_NAMESPACE`
- `test/e2e/k3d/coredns-custom.yaml` — `e2e-bp.local` -> `e2e-wp.local`

### Step 17: PROJECT File

**Modified:**
- `PROJECT` — Kubebuilder resource definitions

### Step 18: Samples & Documentation

**Modified sample YAML files** (all `buildPlaneRef` -> `workflowPlaneRef`):
- `samples/workflows/ci/docker.yaml`, `react.yaml`, `google-cloud-buildpacks.yaml`
- `samples/getting-started/workflows/*.yaml`
- `samples/ocSchema/workflows/*.yaml`
- All other sample YAML files with `buildPlaneRef` fields

**Renamed:**
- `docs/configure-build-plane.md` -> `docs/configure-workflow-plane.md`

**Modified documentation:**
- `docs/proposals/0245-introduce-build-plane.md`
- `docs/contributors/build-engines.md` — `bpClient` -> `wpClient` in interface docs
- `docs/install-guide-multi-cluster.md`
- `CLAUDE.md`

**Modified install scripts:**
- `install/k3d/single-cluster/values-bp.yaml` -> `values-wp.yaml`
- `install/k3d/preload-images.sh` — all `BP`/`bp` vars and flags -> `WP`/`wp`
- `install/quick-start/*.sh` — `BUILD_PLANE_NS` -> `WORKFLOW_PLANE_NS`, `ENABLE_BUILD_PLANE` -> `ENABLE_WORKFLOW_PLANE`, `--bp-values` -> `--wp-values`, `--bp-chart` -> `--wp-chart`
- `.github/workflows/cleanup-artifacts.yml`

### Step 19: Code Generation

All generation targets ran successfully:
```
make generate          # DeepCopy methods regenerated
make manifests         # CRD YAMLs and RBAC regenerated
make openapi-codegen   # API models/handlers/client regenerated
make code.gen          # Full generation (includes Helm schema and samples)
```

### Step 20: Build & Test

```
make go.build          # All 6 binaries compile (manager, occ, openchoreo-api, observer, cluster-gateway, cluster-agent)
make fmt               # Clean (some files auto-formatted)
make vet               # Clean, no issues
```

### Step 21: Variable Name Cleanup

**Go variable/parameter renames (`bp` -> `wp`, `cbp` -> `cwp`, `bpClientMgr` -> `wpClientMgr`, `bpClient` -> `wpClient`):**

All Go source files across the codebase that used `bp`/`cbp` as abbreviations for BuildPlane/ClusterBuildPlane have been updated to use `wp`/`cwp` for WorkflowPlane/ClusterWorkflowPlane. This includes:

- Service struct fields and constructors (10+ files)
- Function/method parameters (interface.go, service.go, service_authz.go for both workflowplane and clusterworkflowplane packages)
- Controller helper functions (run_engine.go, controller_finalize.go)
- MCP handler helper functions (transform_resources.go, infrastructure.go)
- CLI command variables (workflowplane.go, logs.go)
- Test fixture data names across all test files
- Error messages referencing "Build plane"
- Documentation interface examples

### Step 22: Enforce Workflow Resource Namespace

**Context:** With the rename to WorkflowPlane, the workflow execution namespace changed from `ci-<namespaceName>` to `workflows-<namespaceName>`. All additional resources defined in the `resources[]` section of a Workflow must be applied to this enforced namespace and cannot override it.

**Modified:**
- `internal/pipeline/workflow/pipeline.go` — `renderResources()` now force-sets `metadata.namespace` on all rendered resources to the enforced namespace (`workflows-<namespaceName>`), regardless of what the template specifies. Added `extractEnforcedNamespace()` and `setResourceNamespace()` helpers.

**Tests added:**
- `internal/pipeline/workflow/pipeline_test.go` — `TestPipeline_Render_ResourceNamespaceEnforcement` with 4 test cases:
  - Resource with a hardcoded different namespace gets overridden
  - Resource using `${metadata.namespace}` CEL expression gets enforced namespace
  - Resource without namespace gets enforced namespace added
  - Multiple resources all get enforced namespace

**Follow-up (separate PR):** Add validating admission webhooks for Workflow and ClusterWorkflow to reject resources with namespaces other than `${metadata.namespace}` at admission time. See `samples/try-out/18. enforce workflow resource namespace/TASK.md`.

### Step 23: Rename Build Execution Namespace

**Context:** The workflow execution namespace was still using the old `openchoreo-ci-<namespace>` naming convention. Renamed to `workflows-<namespace>` to align with the WorkflowPlane terminology. Workflow sample templates updated to use the `${metadata.namespace}` CEL variable instead of hardcoding the prefix.

**Go source files modified:**
- `internal/pipeline/workflow/pipeline.go` — `ciNamespace` -> `workflowNamespace`, format `"openchoreo-ci-%s"` -> `"workflows-%s"`
- `internal/openchoreo-api/services/gitsecret/service.go` — prefix `"openchoreo-ci-"` -> `"workflows-"`, `getCINamespace()` -> `getWorkflowNamespace()`, all `ciNamespace` vars -> `workflowNamespace`
- `internal/openchoreo-api/legacyservices/gitsecret_service.go` — same renames as above
- `internal/observer/opensearch/queries.go` — format `"openchoreo-ci-%s"` -> `"workflows-%s"`, updated comment

**Workflow sample YAML files** (changed from hardcoded `openchoreo-ci-${metadata.namespaceName}` or `openchoreo-ci-${metadata.orgName}` to `${metadata.namespace}`):
- `samples/workflows/ci/docker.yaml`
- `samples/workflows/generic/github-stats-report/workflow-github-stats-report.yaml`
- `samples/workflows/generic/scm-create-repo/github-create-repo.yaml`
- `samples/workflows/generic/scm-create-repo/codecommit-create-repo.yaml`
- `samples/gitops-workflows/component-workflows/build-and-release/react/react-gitops-release.yaml`
- `samples/gitops-workflows/component-workflows/build-and-release/docker/docker-gitops-release.yaml`
- `samples/gitops-workflows/component-workflows/build-and-release/google-cloud-buildpacks/google-cloud-buildpacks-gitops-release.yaml`
- `samples/gitops-workflows/workflows/bulk-release/bulk-gitops-release.yaml`
- `samples/01-private-repos/component-workflow.yaml`
- `samples/component-workflows/website/connect-private-registry.mdx`
- `samples/component-workflows/website/component-workflow-secrets.mdx`
- `samples/component-workflows/advanced-schema/google-cloud-buildpacks.yaml`
- `samples/private-registry/component-workflow-google.yaml`
- `samples/private-registry/component-workflow-docker.yaml`
- `samples/private-repo-registry/secret-reference.yaml`
- `samples/private-repo-registry/component.yaml`
- `samples/private-repo-registry/component-workflow.yaml`
- `samples/private-repo-registry/commands.yaml`

**README/doc files** (changed `openchoreo-ci-default` to `workflows-default`, `openchoreo-ci-<namespace>` to `workflows-<namespace>`):
- `samples/workflows/ci/README.md`
- `samples/workflows/generic/scm-create-repo/README.md`
- `samples/gitops-workflows/component-workflows/build-and-release/react/README.md`
- `samples/gitops-workflows/component-workflows/build-and-release/docker/README.md`
- `samples/gitops-workflows/component-workflows/build-and-release/google-cloud-buildpacks/README.md`
- `samples/gitops-workflows/workflows/bulk-release/README.md`
- `samples/from-source/web-apps/react-starter/README.md`
- `samples/from-source/services/ballerina-buildpack-patient-management/README.md`
- `samples/from-source/services/go-docker-greeter/README.md`
- `samples/from-source/services/go-google-buildpack-reading-list/README.md`
- `samples/01-private-repos/github-with-secret/README.md`
- `samples/01-private-repos/github-without-secret/README.md`
- `samples/01-private-repos/gitlab-with-secret/README.md`
- `samples/component-workflows/website/component-workflow-secrets.mdx`
- `samples/component-workflows/website/connect-private-registry.mdx`
- `samples/private-repo-registry/git-provider.yaml`
- `samples/private-repo-registry/discussion.md`
- `samples/private-registry/secret.yaml`
- `install/quick-start/build-deploy-greeter.sh`

### Step 24: Fix Lint & Test Leftovers

**Removed:**
- `internal/openchoreo-api/services/buildplane/` — empty leftover directory with 0-byte `service.go`

**Fixed API handler compilation:**
- `internal/openchoreo-api/api/handlers/workflowplanes.go` — added missing `request.Params.LabelSelector` arg to `NormalizeListOptions()`
- `internal/openchoreo-api/api/handlers/clusterworkflowplanes.go` — same fix

**Fixed clusterworkflowplane test files (still referenced old `ClusterBuildPlane` types):**
- `internal/controller/clusterworkflowplane/controller_test.go` — changed package to `clusterworkflowplane_test`, use qualified `clusterworkflowplane.Reconciler`
- `internal/controller/clusterworkflowplane/controller_integration_test.go` — full rename: `ClusterBuildPlane` -> `ClusterWorkflowPlane`, `clusterbuildplane` import -> `clusterworkflowplane`, `cbp` -> `cwp` variables, helper functions, test descriptions, expected condition reasons
- `internal/controller/clusterworkflowplane/controller_unit_test.go` — `clusterbuildplane` import -> `clusterworkflowplane`, function/const references, expected values (`"ClusterBuildPlaneCreated"` -> `"ClusterWorkflowPlaneCreated"`, `"Buildplane is created"` -> `"Workflowplane is created"`, finalizer value)

**Fixed lint warning:**
- `internal/pipeline/workflow/pipeline_test.go` — extracted repeated `"workflows-my-namespace"` string to `testWorkflowsNamespace` constant (goconst)

## Excluded from Rename

- `samples/try-out/` — excluded per task scope
- `discussions/` — historical design documents, left as-is

## Breaking Changes

1. **CRD Resource Names**: `buildplanes` -> `workflowplanes`, `clusterbuildplanes` -> `clusterworkflowplanes`. Existing CRs will not be recognized after upgrade without migration.
2. **JSON/YAML Tags**: `buildPlaneRef` -> `workflowPlaneRef` in Workflow and ClusterWorkflow specs. Existing YAML manifests referencing the old field name will break.
3. **Label Keys**: `openchoreo.dev/build-plane` -> `openchoreo.dev/workflow-plane`. Affects label selectors on existing resources.
4. **Helm Chart Name**: `openchoreo-build-plane` -> `openchoreo-workflow-plane`. Existing Helm releases will need migration.
5. **CLI Commands**: `occ buildplane` / `occ bp` -> `occ workflowplane` / `occ wp`. Scripts using old commands will break.
6. **REST API Paths**: `/buildplanes` -> `/workflowplanes`, `/clusterbuildplanes` -> `/clusterworkflowplanes`.
7. **Cluster Agent Plane Type**: `build-plane` -> `workflow-plane` in cluster-agent configuration.
