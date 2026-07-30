# Plan: Replace Original Privileged Templates with Rootless Equivalents

## Goal

Replace the original privileged workflow templates and CI workflows in-place with the
working rootless `-claude` versions. Preserve all Kubernetes metadata names so that
existing component types, sample components, e2e tests, and generated files continue
to work without modification.

**Builder choice for Dockerfile builds:** BuildKit (not Buildah).

---

## 1. Mapping: Original -> Rootless Source

### 1.1 Workflow Templates (ClusterWorkflowTemplate)

| Original File | metadata.name | Rootless Source (`-claude`) | New metadata.name |
|---|---|---|---|
| `workflow-templates/containerfile-build.yaml` | `containerfile-build` | `rootless-buildkit/containerfile-build-claude.yaml` | `containerfile-build` |
| `workflow-templates/paketo-buildpacks-build.yaml` | `paketo-buildpacks-build` | `rootless-cnb/paketo-buildpacks-build-claude.yaml` | `paketo-buildpacks-build` |
| `workflow-templates/gcp-buildpacks-build.yaml` | `gcp-buildpacks-build` | `rootless-cnb/gcp-buildpacks-build-claude.yaml` | `gcp-buildpacks-build` |
| `workflow-templates/ballerina-buildpack-build.yaml` | `ballerina-buildpack-build` | `rootless-cnb/ballerina-buildpack-build-claude.yaml` | `ballerina-buildpack-build` |
| `workflow-templates/publish-image-k3d.yaml` | `publish-image` | `rootless-publish/publish-image-k3d.yaml` | `publish-image` |
| `workflow-templates/publish-image.yaml` | `publish-image` | Based on `rootless-publish/publish-image-k3d.yaml` | `publish-image` |
| `workflow-templates/generate-workload-k3d.yaml` | `generate-workload` | `rootless-tooling/generate-workload-k3d.yaml` | `generate-workload` |
| `workflow-templates/generate-workload.yaml` | `generate-workload` | Based on `rootless-tooling/generate-workload-k3d.yaml` | `generate-workload` |

**Production vs k3d — minimal differences:**

The production variants are derived from the k3d rootless versions with these changes only:

| Template | Difference from k3d |
|---|---|
| `publish-image.yaml` | `REGISTRY_ENDPOINT="ttl.sh/openchoreo-builds"`, `--tls-verify=true` |
| `generate-workload.yaml` | `oauth-token-url` default uses `https://`, `curl -sk` (skip TLS verify) |

**Image strategy — no custom images:**
- `generate-workload` uses plain `alpine:3.23.4` with `apk add --no-cache curl jq yq` at runtime.
  No custom `ci-tools` image needed. `Dockerfile.ci-tools` is deleted.
- All other templates use off-the-shelf public images.

**Image versions (latest stable):**

| Image | Version |
|---|---|
| `moby/buildkit` | `v0.30.0` |
| `quay.io/podman/stable` | `v5.8.2` |
| `alpine` | `3.23.4` |

**Not affected:**
- `workflow-templates/checkout-source.yaml` — no privileged mode, unchanged.

### 1.2 CI Workflows (ClusterWorkflow)

| Original File | metadata.name | Rootless Source (`-claude`) | New metadata.name |
|---|---|---|---|
| `ci-workflows/dockerfile-builder.yaml` | `dockerfile-builder` | `ci-workflows/dockerfile-builder-buildkit-claude.yaml` | `dockerfile-builder` |
| `ci-workflows/paketo-buildpacks-builder.yaml` | `paketo-buildpacks-builder` | `ci-workflows/paketo-buildpacks-builder-rootless-claude.yaml` | `paketo-buildpacks-builder` |
| `ci-workflows/gcp-buildpacks-builder.yaml` | `gcp-buildpacks-builder` | `ci-workflows/gcp-buildpacks-builder-rootless-claude.yaml` | `gcp-buildpacks-builder` |
| `ci-workflows/ballerina-buildpack-builder.yaml` | `ballerina-buildpack-builder` | `ci-workflows/ballerina-buildpack-builder-rootless-claude.yaml` | `ballerina-buildpack-builder` |

---

## 2. Name Preservation — Why It Matters

These names are referenced by other files. Changing them would break the system.

| Consumer | References | Files |
|---|---|---|
| Component types | `allowedWorkflows[].name` | `component-types/service.yaml`, `worker.yaml`, `webapp.yaml`, `scheduled-task.yaml` |
| Sample components | `spec.workflow.name` | `from-source/**/greeting-service.yaml`, `reading-list-service.yaml`, `patient-management-service.yaml`, `react-web-app.yaml`, url-shortener components |
| CI workflows | `templateRef.name` | Each CI workflow references its build template + `publish-image` + `generate-workload` |
| E2E tests | Hardcoded names | `test/e2e/e2e_test.go` (line 100) |
| Makefile gen | File list | `make/lint.mk` — `GETTING_STARTED_FILES` and `WORKFLOW_TEMPLATE_FILES` |
| Generated files | Auto-generated from above | `all.yaml`, `workflow-templates.yaml` |
| OCC CLI | Hardcoded example | `internal/occ/cmd/clusterworkflow/cmd.go` |
| Custom workflow samples | `templateRef.name` | `samples/workflows/custom-steps/linter/dockerfile-builder-linter.yaml` |
| Other component-type samples | `allowedWorkflows[].name` | `samples/component-types/component-with-configs/`, `component-with-embedded-traits/` |

---

## 3. Steps

### Step 1: Replace Workflow Templates

For each of the 8 workflow templates in section 1.1:

1. Copy content from the rootless source file into the original file.
2. Change `metadata.name` to the original name (remove `-buildkit-claude`, `-rootless-claude`, `-rootless` suffixes).
3. Remove the `-claude` comment header (the "Based on..." / "Adds:..." comments) — replace with a brief comment noting it's rootless.

**Specific renames inside files:**

| File | Change |
|---|---|
| `containerfile-build.yaml` | `name: containerfile-build-buildkit-claude` -> `name: containerfile-build` |
| `paketo-buildpacks-build.yaml` | `name: paketo-buildpacks-build-rootless-claude` -> `name: paketo-buildpacks-build` |
| `gcp-buildpacks-build.yaml` | `name: gcp-buildpacks-build-rootless-claude` -> `name: gcp-buildpacks-build` |
| `ballerina-buildpack-build.yaml` | `name: ballerina-buildpack-build-rootless-claude` -> `name: ballerina-buildpack-build` |
| `publish-image-k3d.yaml` | `name: publish-image-rootless` -> `name: publish-image` |
| `publish-image.yaml` | `name: publish-image-rootless` -> `name: publish-image` |
| `generate-workload-k3d.yaml` | `name: generate-workload-rootless` -> `name: generate-workload` |
| `generate-workload.yaml` | `name: generate-workload-rootless` -> `name: generate-workload` |

**Production variants — additional changes after copying from k3d rootless:**

`publish-image.yaml`:
- Change `REGISTRY_ENDPOINT="host.k3d.internal:10082"` to `REGISTRY_ENDPOINT="ttl.sh/openchoreo-builds"`
- Change `--tls-verify=false` to `--tls-verify=true`

`generate-workload.yaml`:
- Change `oauth-token-url` default from `"http://..."` to `"https://..."`
- Change `curl -s` to `curl -sk` in OAuth and API calls (skip TLS verify for self-signed certs)

### Step 2: Replace CI Workflows

For each of the 4 CI workflows in section 1.2:

1. Copy content from the rootless `-claude` source file into the original file.
2. Change `metadata.name` to the original name (remove `-buildkit-claude`, `-rootless-claude` suffixes).
3. **Update internal `templateRef.name` values** to point to the original template names (not the `-claude`/`-rootless` names):

| templateRef in rootless-claude CI workflow | Change to |
|---|---|
| `containerfile-build-buildkit-claude` | `containerfile-build` |
| `paketo-buildpacks-build-rootless-claude` | `paketo-buildpacks-build` |
| `gcp-buildpacks-build-rootless-claude` | `gcp-buildpacks-build` |
| `ballerina-buildpack-build-rootless-claude` | `ballerina-buildpack-build` |
| `publish-image-rootless` | `publish-image` |
| `generate-workload-rootless` | `generate-workload` |

### Step 3: Regenerate Combined Files

```bash
make samples-gen            # regenerates all.yaml
make workflow-templates-gen # regenerates workflow-templates.yaml
```

No changes needed to `make/lint.mk` — the file lists reference the same original filenames
which we're replacing in-place.

### Step 4: Update Sample Components

The original sample components (`greeting-service.yaml`, `reading-list-service.yaml`,
`patient-management-service.yaml`, `react-web-app.yaml`) reference the original CI workflow
names (`dockerfile-builder`, `gcp-buildpacks-builder`, etc.) — these names are preserved,
so **no changes needed**.

### Step 5: Cleanup Experimental Files

Delete all experimental variants (rootless subdirectories and `-claude`/`-gpt`/`-rootless` CI
workflow files). These are no longer needed since their content has been merged into the originals.

**Workflow template directories to delete:**
- `workflow-templates/rootless-buildkit/` (3 files)
- `workflow-templates/rootless-buildah/` (3 files)
- `workflow-templates/rootless-cnb/` (9 files)
- `workflow-templates/rootless-publish/` (1 file)
- `workflow-templates/rootless-tooling/` (1 file)
- `workflow-templates/CONTEXT-claude.md`
- `workflow-templates/CONTEXT.md` (if exists)

**CI workflow files to delete:**
- `ci-workflows/dockerfile-builder-buildkit-claude.yaml`
- `ci-workflows/dockerfile-builder-buildkit-gpt.yaml`
- `ci-workflows/dockerfile-builder-buildkit.yaml`
- `ci-workflows/dockerfile-builder-buildah-claude.yaml`
- `ci-workflows/dockerfile-builder-buildah-gpt.yaml`
- `ci-workflows/dockerfile-builder-buildah.yaml`
- `ci-workflows/paketo-buildpacks-builder-rootless-claude.yaml`
- `ci-workflows/paketo-buildpacks-builder-rootless-gpt.yaml`
- `ci-workflows/paketo-buildpacks-builder-rootless.yaml`
- `ci-workflows/gcp-buildpacks-builder-rootless-claude.yaml`
- `ci-workflows/gcp-buildpacks-builder-rootless-gpt.yaml`
- `ci-workflows/gcp-buildpacks-builder-rootless.yaml`
- `ci-workflows/ballerina-buildpack-builder-rootless-claude.yaml`
- `ci-workflows/ballerina-buildpack-builder-rootless-gpt.yaml`
- `ci-workflows/ballerina-buildpack-builder-rootless.yaml`

**Sample component files to delete:**
- `from-source/services/go-docker-greeter/greeting-service-buildkit-claude.yaml`
- `from-source/services/go-docker-greeter/greeting-service-buildkit-gpt.yaml`
- `from-source/services/go-docker-greeter/greeting-service-buildah-claude.yaml`
- `from-source/services/go-docker-greeter/greeting-service-buildah-gpt.yaml`
- `from-source/services/go-google-buildpack-reading-list/reading-list-service-gcp-rootless-claude.yaml`
- `from-source/services/go-google-buildpack-reading-list/reading-list-service-gcp-rootless-gpt.yaml`
- `from-source/services/go-google-buildpack-reading-list/reading-list-service-gcp-rootless-no-cache-gpt.yaml`
- `from-source/services/go-google-buildpack-reading-list/reading-list-service-gcp-rootless.yaml`
- `from-source/services/ballerina-buildpack-patient-management/patient-management-service-rootless-claude.yaml`

### Step 6: Verify No Broken References

```bash
# Check that all referenced names still resolve
grep -rn 'templateRef' samples/getting-started/ci-workflows/*.yaml | grep -v 'checkout-source\|containerfile-build\|paketo-buildpacks-build\|gcp-buildpacks-build\|ballerina-buildpack-build\|publish-image\|generate-workload'

# Check that no -claude, -gpt, -rootless suffixed names remain in any non-deleted file
grep -rn 'claude\|rootless' samples/getting-started/ci-workflows/*.yaml samples/getting-started/workflow-templates/*.yaml

# Regenerate and verify clean git status for generated files
make code.gen-check
```

---

## 4. Files Changed Summary

### Replaced in-place (8 workflow templates)
- `samples/getting-started/workflow-templates/containerfile-build.yaml`
- `samples/getting-started/workflow-templates/paketo-buildpacks-build.yaml`
- `samples/getting-started/workflow-templates/gcp-buildpacks-build.yaml`
- `samples/getting-started/workflow-templates/ballerina-buildpack-build.yaml`
- `samples/getting-started/workflow-templates/publish-image-k3d.yaml`
- `samples/getting-started/workflow-templates/publish-image.yaml`
- `samples/getting-started/workflow-templates/generate-workload-k3d.yaml`
- `samples/getting-started/workflow-templates/generate-workload.yaml`

### Replaced in-place (4 CI workflows)
- `samples/getting-started/ci-workflows/dockerfile-builder.yaml`
- `samples/getting-started/ci-workflows/paketo-buildpacks-builder.yaml`
- `samples/getting-started/ci-workflows/gcp-buildpacks-builder.yaml`
- `samples/getting-started/ci-workflows/ballerina-buildpack-builder.yaml`

### Regenerated (2 combined files)
- `samples/getting-started/all.yaml`
- `samples/getting-started/workflow-templates.yaml`

### Deleted (~30+ experimental files)
- All files in `rootless-buildkit/`, `rootless-buildah/`, `rootless-cnb/`, `rootless-publish/`, `rootless-tooling/`
- All `-claude`, `-gpt`, `-rootless`, `-buildah`, `-buildkit` variant CI workflows
- All `-claude`, `-gpt`, `-rootless` variant sample components
- `install/base-images/Dockerfile.ci-tools` (custom image no longer needed)

### NOT changed
- `workflow-templates/checkout-source.yaml`
- `make/lint.mk` (file lists unchanged)
- `component-types/*.yaml` (names preserved)
- `from-source/**/greeting-service.yaml` etc. (names preserved)
- `test/e2e/e2e_test.go` (names preserved)
- `install/k3d/single-cluster/config.yaml` (k3d cluster config — K8s version already updated)

---

## 5. Risks

1. **K8s 1.34 requirement.** The rootless templates use `hostUsers: false` which requires
   K8s 1.33+ (GA user namespaces). The k3d config has already been updated to 1.34. Any
   user running an older cluster will get pod admission errors. This is a known, accepted
   breaking change (documented in TASK.md).

2. **Podman sidecar startup.** CNB templates wait up to 300s for the Podman socket. If the
   sidecar image pull is slow on first run, this could time out. Not a regression — the
   original also had startup delays, just hidden inside a single container.

3. **Custom workflow samples.** `samples/workflows/custom-steps/linter/dockerfile-builder-linter.yaml`
   references `containerfile-build` and `publish-image` by name — these names are preserved,
   so no breakage. But this workflow still uses the old privileged publish-image template
   internally if its own CI workflow references aren't updated.

---

## 6. Execution Order

| # | Step | Depends On | Risk |
|---|---|---|---|
| 1 | Replace 8 workflow templates (6 k3d + 2 production) | — | Low |
| 2 | Replace 4 CI workflows | Step 1 (need correct template names) | Low |
| 3 | `make samples-gen workflow-templates-gen` | Steps 1-2 | None |
| 4 | Verify references (`grep`, `make code.gen-check`) | Step 3 | None |
| 5 | Delete experimental files | Step 4 (verify first) | Low |
| 6 | Re-verify after deletion | Step 5 | None |
| 7 | Test on k3d cluster | Step 6 | Medium |
