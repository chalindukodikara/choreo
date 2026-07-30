# Website Documentation Update — Current Context

## What Was Done

### 1. API Reference Pages — Updated
- **`website/docs/reference/api/platform/workflow.md`** — Rewritten with all new fields: `buildPlaneRef`, `externalRefs`, `resources`, `ttlAfterCompletion`. Includes ExternalRef, WorkflowResource, BuildPlaneRef sub-sections, template variable reference, and two full examples (Docker build + generic automation).
- **`website/docs/reference/api/application/workflowrun.md`** — Rewritten with new status fields: `tasks` (WorkflowTask), `startedAt`, `completedAt`, `resources`. Added component WorkflowRun labels section, ResourceReference/WorkflowTask tables, and status example with tasks.

### 2. Old API Reference Pages — Deleted
- `website/docs/reference/api/platform/componentworkflow.md` — **Deleted**
- `website/docs/reference/api/application/componentworkflowrun.md` — **Deleted**

### 3. New User Guide Structure — Created
```
website/docs/user-guide/workflows/
├── overview.md                    — Unified workflow model, types, lifecycle, defaults
├── creating-workflows.md          — Step-by-step: CWT → Workflow CR, params, externalRefs, resources
├── running-workflows.md           — WorkflowRun creation, monitoring, cleanup
├── additional-resources.md        — Resources field (ExternalSecrets, ConfigMaps, conditional)
└── ci/
    ├── overview.md                — How CI workflows differ, annotations, labels, default CWTs
    ├── component-type-setup.md    — allowedWorkflows config, validation (WorkflowNotAllowed/NotFound)
    ├── component-workflows.md     — component-workflow-parameters annotation, auto-build, webhooks
    ├── workload-generation.md     — generate-workload-cr step, controller special case, alternatives
    ├── external-ci.mdx            — Moved from ci/, relative paths fixed
    └── private-repository.md      — Moved from ci/, image paths + content updated
```

### 4. Cross-References — Updated
Files updated to fix links and replace ComponentWorkflow/ComponentWorkflowRun references:

| File | Changes |
|------|---------|
| `user-guide/gitops/build-and-release-workflows.mdx` | All YAML examples updated (kind: WorkflowRun, no owner/systemParameters), prose updated, kubectl commands updated, See Also links fixed |
| `user-guide/gitops/bulk-promote.mdx` | CI Overview link → Workflows Overview |
| `concepts/platform-abstractions.md` | ComponentWorkflow section → Workflow section (rewritten) |
| `concepts/developer-abstractions.md` | ComponentWorkflowRun section → WorkflowRun section (rewritten) |
| `getting-started/try-it-out/on-k3d-locally.mdx` | ComponentWorkflows → Workflows |
| `operations/auto-build.mdx` | kubectl get componentworkflowrun → workflowrun |
| `operations/namespace-management.mdx` | kubectl get componentworkflow → workflow (all occurrences) |
| `operations/authorization.md` | componentworkflow:view removed, componentworkflowrun:view → workflowrun:view |
| `user-guide/authorization/overview.md` | Permission table updated to workflow:view, workflow:create, workflowrun:view |

## What Remains

### Not Yet Updated (need review)
- **`reference/cli-reference.md`** — Still has `occ componentworkflow` and `occ componentworkflowrun` CLI commands (lines 812-927). Need to verify if CLI commands were renamed in the codebase before updating docs.

### Old User Guide Files — Not Yet Deleted
The old `user-guide/ci/` files still exist (the new `user-guide/workflows/` files replace them):
- `user-guide/ci/overview.md`
- `user-guide/ci/component-workflow-schema.md`
- `user-guide/ci/custom-workflows.md`
- `user-guide/ci/generic-workflows.md`
- `user-guide/ci/additional-resources.md`
- `user-guide/ci/private-repository.md`
- `user-guide/ci/external-ci.mdx`

These should be deleted once we confirm no other docs link to them. The images in `user-guide/ci/images/` are still referenced by the new `private-repository.md` (via `../../ci/images/`), so keep the images directory.

### Potential Issues to Verify
1. The `build-and-release-workflows.mdx` YAML examples now use `WorkflowRun` with `labels` for component/project and flat `parameters` (no `systemParameters`). Verify this matches how the actual GitOps workflows define their schema — the gitops Workflow CRs may still use `systemParameters`-like parameter nesting in `schema.parameters`.
2. The `component-workflow-parameters` annotation value format (`repoUrl: parameters.repository.url`) should be verified against actual default workflow YAMLs in the `samples/` directory.
3. The `externalRefs` CEL syntax (`${externalRefs.repo-credentials.spec.*}`) should be verified against controller implementation.
