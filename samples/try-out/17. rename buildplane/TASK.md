# Task: Rename BuildPlane to WorkflowPlane

**Status: COMPLETED**

## Context
With the introduction of generic workflows, all workflows (component and generic) now execute in the workflow plane. The name `WorkflowPlane` better reflects the purpose of this resource. This is a mechanical rename across the entire codebase.

## Naming Conventions

| Old Name | New Name |
|---|---|
| `BuildPlane` | `WorkflowPlane` |
| `ClusterBuildPlane` | `ClusterWorkflowPlane` |
| `BuildPlaneRef` | `WorkflowPlaneRef` |
| `ClusterBuildPlaneRef` | `ClusterWorkflowPlaneRef` |
| `BuildPlaneSpec` | `WorkflowPlaneSpec` |
| `BuildPlaneStatus` | `WorkflowPlaneStatus` |
| `BuildPlaneList` | `WorkflowPlaneList` |
| `buildPlane` | `workflowPlane` |
| `buildplane` | `workflowplane` |
| `build-plane` | `workflow-plane` |
| `build_plane` | `workflow_plane` |
| `bp` (CLI alias) | `wp` |
| `buildplanes` (CLI alias) | `workflowplanes` |
| `bpClientMgr` (variable) | `wpClientMgr` |
| `bpClient` (variable) | `wpClient` |
| `cbp` (variable for ClusterBuildPlane) | `cwp` |

## Execution Steps

All steps below have been completed. See `IMPLEMENTATION.md` for details.

### Step 1: CRD Type Definitions (api/v1alpha1/) - DONE
### Step 2: Controllers - DONE
### Step 3: Webhooks - DONE (no webhook files existed for BuildPlane)
### Step 4: OpenAPI Spec and Generated API Code - DONE
### Step 5: API Handlers - DONE
### Step 6: Services - DONE
### Step 7: Legacy Services - DONE
### Step 8: MCP Handlers - DONE
### Step 9: MCP Tools (pkg/mcp/) - DONE
### Step 10: CLI Commands - DONE
### Step 11: Authorization - DONE
### Step 12: Labels - DONE
### Step 13: Cluster Gateway - DONE
### Step 14: Configuration & RBAC - DONE
### Step 15: Helm Charts - DONE
### Step 16: E2E Tests & Makefiles - DONE
### Step 17: PROJECT File - DONE
### Step 18: Samples & Documentation - DONE
### Step 19: Run Code Generation & Verify - DONE
### Step 20: Build & Test - DONE (build, fmt, vet all pass)
### Step 21: Variable Name Cleanup - DONE
### Step 22: Enforce Workflow Resource Namespace - DONE
  - Renamed all `bp`/`cbp` Go variable names to `wp`/`cwp` for WorkflowPlane/ClusterWorkflowPlane
  - Renamed `bpClientMgr` -> `wpClientMgr` and `bpClient` -> `wpClient` across all services
  - Fixed error messages ("Build plane" -> "Workflow plane")
  - Fixed test fixture data (e.g., `ci-bp` -> `ci-wp`, `shared-bp` -> `shared-wp`)
  - Fixed Makefile/script abbreviations (`e2e-bp` -> `e2e-wp`, `values-bp` -> `values-wp`)
  - Fixed CoreDNS config (`e2e-bp.local` -> `e2e-wp.local`)
  - Updated `docs/contributors/build-engines.md` interface docs
### Step 23: Rename Build Execution Namespace - DONE
  - Changed namespace format from `openchoreo-ci-<namespace>` to `workflows-<namespace>`
  - Updated `internal/pipeline/workflow/pipeline.go` — `ciNamespace` -> `workflowNamespace`, format string
  - Updated `internal/openchoreo-api/services/gitsecret/service.go` — prefix constant, `getCINamespace` -> `getWorkflowNamespace`, variable renames
  - Updated `internal/openchoreo-api/legacyservices/gitsecret_service.go` — same as above
  - Updated `internal/observer/opensearch/queries.go` — format string and comment
  - Updated all workflow sample YAML files to use `${metadata.namespace}` instead of hardcoding prefix
  - Updated sample READMEs and docs (`openchoreo-ci-default` -> `workflows-default`)
  - Updated `install/quick-start/build-deploy-greeter.sh`
### Step 24: Fix Lint & Test Leftovers - DONE
  - Removed empty `internal/openchoreo-api/services/buildplane/` leftover directory
  - Added missing `LabelSelector` param to `NormalizeListOptions` in `workflowplanes.go` and `clusterworkflowplanes.go`
  - Fixed `clusterworkflowplane` test files still referencing old `ClusterBuildPlane` types:
    - `controller_test.go` — moved to `_test` package, use qualified `clusterworkflowplane.Reconciler`
    - `controller_integration_test.go` — full rename from `ClusterBuildPlane` to `ClusterWorkflowPlane`
    - `controller_unit_test.go` — updated import, expected values, function names
  - Extracted `"workflows-my-namespace"` to `testWorkflowsNamespace` constant in `pipeline_test.go`

## Important Considerations

1. **CRD API Version**: The CRD group remains `openchoreo.dev`, but the resource names change. This is a **breaking change** for existing clusters. A migration strategy (CRD conversion webhook or manual migration) may be needed.

2. **Generated Files**: Never manually edit files in `api/v1alpha1/zz_generated.deepcopy.go`, `internal/openchoreo-api/api/gen/`, or `config/crd/bases/`. Always use `make generate`, `make manifests`, and `make openapi-codegen`.

3. **JSON Tags**: When renaming struct fields, ensure JSON/YAML tags are updated (e.g., `json:"buildPlaneRef"` -> `json:"workflowPlaneRef"`). This affects API serialization and is a breaking change.

4. **Label Keys**: Changing `openchoreo.dev/build-plane` to `openchoreo.dev/workflow-plane` affects label selectors on existing resources.

5. **Helm Chart Name**: Renaming `openchoreo-build-plane` chart affects existing Helm releases.

6. **CLI Aliases**: The `bp` alias changes to `wp`. Users with scripts using `occ buildplane` or `occ bp` will need to update.

7. **Order of Operations**: Follow the steps in order. API types (Step 1) must be done first, then `make generate manifests` before proceeding, since many other files depend on the generated types.
