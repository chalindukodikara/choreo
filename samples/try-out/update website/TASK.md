# Task: Update Website Documentation for Recent Workflow Changes

## Background

Three major changes were recently made to the OpenChoreo codebase that affect the website documentation:

1. **BuildPlane renamed to WorkflowPlane** — The "build plane" now handles all workflows (CI builds + generic workflows), so it was renamed to better reflect its purpose.
   - Reference: `samples/try-out/17. rename buildplane/TASK.md`
   - Reference: `samples/try-out/17. rename buildplane/IMPLEMENTATION.md`

2. **OpenAPIV3Schema support added to Workflows** — `spec.schema` wrapper was removed; `parameters` and `environmentConfigs` are now top-level fields using a `SchemaSection` that supports both `ocSchema` and `openAPIV3Schema` formats.
   - Reference: `samples/try-out/16. OpenAPI V3 Shema/TASK2.md`

3. **Workload creation moved into workflow execution** — Instead of the controller creating Workload CRs from Argo output, the ClusterWorkflowTemplate now calls the OpenChoreo API server directly to create workloads.
   - Reference: `samples/try-out/15. Create workload from workflows/TASK.md`

---

## Part 1: Remove Obsolete Documentation

Delete these files — they document CRDs that no longer exist:

| File | Reason |
|------|--------|
| `website/docs/reference/api/application/build.md` | `Build` CRD was deprecated and removed |
| `website/docs/reference/api/application/componentworkflowrun.md` | `ComponentWorkflowRun` CRD replaced by `WorkflowRun` |
| `website/docs/reference/api/platform/componentworkflow.md` | `ComponentWorkflow` CRD replaced by `Workflow` |

After deleting, remove any sidebar/nav references to these pages (check `website/sidebars.js` or equivalent config).

---

## Part 2: Rename BuildPlane to WorkflowPlane

Apply the following renames **across all website docs**:

| Old Term | New Term |
|----------|----------|
| `BuildPlane` | `WorkflowPlane` |
| `ClusterBuildPlane` | `ClusterWorkflowPlane` |
| `buildPlaneRef` | `workflowPlaneRef` |
| `buildplane` (CLI/URL) | `workflowplane` |
| `build-plane` (Helm/labels) | `workflow-plane` |
| `Build Plane` (prose) | `Workflow Plane` |
| `openchoreo-build-plane` (Helm chart) | `openchoreo-workflow-plane` |
| `openchoreo.dev/build-plane` (label) | `openchoreo.dev/workflow-plane` |
| `occ buildplane` / `occ bp` | `occ workflowplane` / `occ wp` |
| `openchoreo-ci-<namespace>` (execution namespace) | `workflows-<namespace>` |
| `/buildplanes` (API path) | `/workflowplanes` |
| `/clusterbuildplanes` (API path) | `/clusterworkflowplanes` |

### Files that need BuildPlane -> WorkflowPlane renaming

**Dedicated reference pages (rename or recreate):**
- `website/docs/reference/api/platform/buildplane.md` -> rename file to `workflowplane.md`, update all content
- `website/docs/reference/api/platform/clusterbuildplane.md` -> rename file to `clusterworkflowplane.md`, update all content
- `website/docs/reference/helm/build-plane.mdx` -> rename file to `workflow-plane.mdx`, update all content

**Getting started:**
- `website/docs/getting-started/quick-start-guide.mdx`
- `website/docs/getting-started/try-it-out/on-k3d-locally.mdx`
- `website/docs/getting-started/try-it-out/on-your-environment.mdx`

**Concepts:**
- `website/docs/concepts/platform-abstractions.md`
- `website/docs/concepts/developer-abstractions.md`

**Operations (heavily affected):**
- `website/docs/operations/deployment-topology.mdx` — architecture diagrams, resource hierarchy, kubectl commands
- `website/docs/operations/auto-build.mdx`
- `website/docs/operations/container-registry-configuration.mdx`
- `website/docs/operations/multi-cluster-connectivity.mdx`
- `website/docs/operations/namespace-management.mdx`
- `website/docs/operations/cluster-agent-rbac.mdx`
- `website/docs/operations/upgrades.mdx`
- `website/docs/operations/external-ca-tls-setup.md`
- `website/docs/operations/authorization.md`
- `website/docs/operations/observability-alerting.mdx`
- `website/docs/operations/modules/building-a-module.md`

**User guide - workflows:**
- `website/docs/user-guide/workflows/overview.md`
- `website/docs/user-guide/workflows/start-here.mdx`
- `website/docs/user-guide/workflows/creating-workflows.mdx`
- `website/docs/user-guide/workflows/running-workflows.md`
- `website/docs/user-guide/workflows/workflow-schema.md`
- `website/docs/user-guide/workflows/ci/private-repository.md`
- `website/docs/user-guide/workflows/ci/workload-generation.md`

**User guide - other:**
- `website/docs/user-guide/gitops/build-and-release-workflows.mdx`
- `website/docs/user-guide/gitops/bulk-promote.mdx`
- `website/docs/user-guide/authorization/overview.md`

**Reference - cross-references in other API docs:**
- `website/docs/reference/api/platform/dataplane.md` — cross-references BuildPlane
- `website/docs/reference/api/platform/clusterdataplane.md` — cross-references ClusterBuildPlane
- `website/docs/reference/api/platform/observabilityplane.md` — cross-references BuildPlane
- `website/docs/reference/api/platform/clusterobservabilityplane.md` — cross-references ClusterBuildPlane
- `website/docs/reference/api/platform/workflow.md` — "build plane" references
- `website/docs/reference/api/application/workflowrun.md` — "build plane" references
- `website/docs/reference/api/application/component.md` — ComponentWorkflow and build plane references
- `website/docs/reference/api/application/project.md` — `buildPlaneRef` field reference
- `website/docs/reference/configuration-schema.md`
- `website/docs/reference/cli-reference.md`
- `website/docs/reference/mcp-servers/mcp-servers-overview.mdx`

**Other:**
- `website/docs/learn-from-examples/examples-catalog.mdx`

---

## Part 3: Update Workflow Schema Documentation

Update `website/docs/user-guide/workflows/workflow-schema.md` and any related reference pages to reflect:

- The `spec.schema` wrapper no longer exists
- `parameters` and `environmentConfigs` are now top-level fields on `WorkflowSpec`
- Each uses a `SchemaSection` with two mutually exclusive formats:
  - `ocSchema` — shorthand format (existing)
  - `openAPIV3Schema` — standard JSON Schema format (new)
- `$types` for reusable definitions is now embedded inline within `ocSchema` blocks
- Document the new `openAPIV3Schema` option with an example

Also update `website/docs/reference/api/platform/workflow.md` spec table if it still shows the old `spec.schema` structure.

---

## Part 4: Update Workload Generation Documentation

Update `website/docs/user-guide/workflows/ci/workload-generation.md` to reflect:

- The controller no longer has special-case logic to create Workload CRs from Argo workflow output
- The ClusterWorkflowTemplate now calls the OpenChoreo API server directly (`POST /api/v1/namespaces/{namespaceName}/workloads`)
- The workflow obtains an OAuth token via client credentials before calling the API
- Update any architecture diagrams or flow descriptions accordingly

---

## Part 5: Update Sidebar/Navigation

After all file renames and deletions, update the sidebar configuration to:
- Remove entries for deleted files (`build.md`, `componentworkflowrun.md`, `componentworkflow.md`)
- Update paths for renamed files (`buildplane.md` -> `workflowplane.md`, etc.)
- Verify all links resolve correctly

---

## Validation Checklist

- [ ] No remaining references to `BuildPlane`, `ClusterBuildPlane`, `buildplane`, `build-plane`, or `Build Plane` in website docs (except in migration/changelog notes if applicable)
- [ ] No remaining references to `ComponentWorkflow` or `ComponentWorkflowRun` in website docs
- [ ] No broken internal links after file renames/deletions
- [ ] Sidebar navigation reflects all changes
- [ ] Workflow schema docs show the new `SchemaSection` structure (no `spec.schema` wrapper)
- [ ] Workload generation docs reflect the new API-based approach
