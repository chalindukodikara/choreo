# Testing Guide: Phase 1 — `ocSchema` API Restructuring

This guide walks through validating the Phase 1 schema restructuring on a local k3d cluster.

**Branch:** `issue-2493_add_ocschema_field`

---

## Prerequisites

- k3d cluster running (`k3d cluster list` should show `openchoreo`)
- `kubectl` configured to point to the k3d cluster
- Go toolchain installed

---

## 1. Build and Load Images

### 1.1 Update Image Pull Policies

Ensure both deployments use `Never` so they pull from the locally loaded images:

```bash
# Check current policies
kubectl get deploy controller-manager openchoreo-api -n openchoreo-control-plane \
  -o jsonpath='{range .items[*]}{.metadata.name}: {.spec.template.spec.containers[0].imagePullPolicy}{"\n"}{end}'

# Patch if needed (only patch the ones not already set to Never)
kubectl patch deploy controller-manager -n openchoreo-control-plane \
  -p '{"spec":{"template":{"spec":{"containers":[{"name":"manager","imagePullPolicy":"Never"}]}}}}'

kubectl patch deploy openchoreo-api -n openchoreo-control-plane \
  -p '{"spec":{"template":{"spec":{"containers":[{"name":"api-server","imagePullPolicy":"Never"}]}}}}'
```

### 1.2 Build Images

```bash
make k3d.build.controller
make k3d.build.openchoreo-api
```

### 1.3 Load Images into k3d

```bash
make k3d.load.controller
make k3d.load.openchoreo-api
```

### 1.4 Restart Deployments

```bash
kubectl rollout restart deploy controller-manager openchoreo-api -n openchoreo-control-plane
kubectl rollout status deploy controller-manager openchoreo-api -n openchoreo-control-plane --timeout=90s
```

Verify no crash loops:

```bash
kubectl get pods -n openchoreo-control-plane -l app.kubernetes.io/component=controller-manager
# Should show RESTARTS: 0 after a minute
```

---

## 2. Clean Up Old CRs and CRDs

Delete existing CRs that use the old flat schema format. Order matters (reverse dependency):

```bash
# Delete in dependency order
kubectl delete componentreleases --all -n default
kubectl delete components --all -n default
kubectl delete workflowruns --all -n default
kubectl delete traits --all -n default
kubectl delete componenttypes --all -n default
kubectl delete workflows.openchoreo.dev --all -n default

# Also clean cluster-scoped if any exist
kubectl delete clustercomponenttypes --all 2>/dev/null
kubectl delete clustertraits --all 2>/dev/null
```

> **Note:** Use `workflows.openchoreo.dev` (fully qualified) to avoid conflict with the Argo `workflows.argoproj.io` CRD.

---

## 3. Install New CRDs

```bash
make install
```

Verify the CRD schema now has the `ocSchema` sub-key:

```bash
kubectl get crd componenttypes.openchoreo.dev \
  -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.schema.properties}' \
  | python3 -m json.tool
```

Expected output should show `ocSchema` as a top-level key under `schema.properties`, containing `types`, `parameters`, and `envOverrides`.

---

## 4. Apply Getting-Started Samples

```bash
# Component types
kubectl apply -f samples/getting-started/component-types/service.yaml
kubectl apply -f samples/getting-started/component-types/worker.yaml
kubectl apply -f samples/getting-started/component-types/web-application.yaml  # use webapp.yaml if that's the name
kubectl apply -f samples/getting-started/component-types/scheduled-task.yaml

# Traits
kubectl apply -f samples/getting-started/component-traits/

# Workflows
kubectl apply -f samples/getting-started/workflows/
```

> **Note:** Skip any `*-claude-*.yaml` experimental files — they use `openAPIV3Schema` which is a Phase 2 feature.

Verify all resources were created:

```bash
kubectl get ct -n default
kubectl get traits -n default
kubectl get workflows.openchoreo.dev -n default
```

---

## 5. Verify Schema Structure in Cluster

Check that the stored schema uses the `ocSchema` nesting:

```bash
# ComponentType
kubectl get ct service -n default -o jsonpath='{.spec.schema}' | python3 -m json.tool

# Trait
kubectl get trait observability-alert-rule -n default -o jsonpath='{.spec.schema}' | python3 -m json.tool

# Workflow
kubectl get workflows.openchoreo.dev docker -n default -o jsonpath='{.spec.schema}' | python3 -m json.tool
```

All three should show `ocSchema` as the top-level key wrapping `types`, `parameters`, and/or `envOverrides`.

---

## 6. Apply a From-Source Sample

Test end-to-end component creation and build:

```bash
kubectl apply -f samples/from-source/services/go-docker-greeter/greeting-service.yaml
```

Verify:

```bash
# Component should be created
kubectl get components -n default

# WorkflowRun should be created
kubectl get workflowruns -n default

# Check build pods in CI namespace
kubectl get pods -n openchoreo-ci-default
```

---

## 7. Validate the all.yaml Combined Sample

```bash
kubectl apply -f samples/getting-started/all.yaml --dry-run=server
```

All resources should pass server-side validation.

---

## 8. Check for Errors

### Controller Logs

```bash
kubectl logs deploy/controller-manager -n openchoreo-control-plane --tail=50 \
  | grep -iE 'error|panic|fatal' | grep -v 'ClusterWorkflow'
```

Should be empty (ClusterWorkflow RBAC errors are pre-existing and unrelated).

### API Server Logs

```bash
kubectl logs deploy/openchoreo-api -n openchoreo-control-plane --tail=50 \
  | grep -iE 'error|panic|fatal'
```

Should be empty.

---

## 9. Run Unit Tests

```bash
# Core packages affected by the change
go test ./api/... \
       ./internal/pipeline/... \
       ./internal/scaffold/... \
       ./internal/validation/... \
       ./internal/openchoreo-api/... \
       ./internal/webhook/...
```

All tests should pass. Webhook/controller envtest tests may fail if `kube-apiserver` binary is not set up locally — this is a pre-existing environment issue, not related to the schema change.

---

## Known Issues (Pre-existing, Not Related to Schema Change)

| Issue | Cause | Workaround |
|-------|-------|------------|
| Controller crash-loops on startup | `ClusterWorkflow` RBAC missing from Helm-deployed ClusterRole | The generated `config/rbac/role.yaml` already includes it. Re-run `make install` or patch the ClusterRole manually (see below). |
| `host.k3d.internal` DNS resolution failure in build pods | k3d host DNS not resolvable from CI namespace pods | Patch ClusterWorkflowTemplates to use in-cluster service DNS (e.g., `registry.openchoreo-build-plane.svc.cluster.local:10082`) |

### Manual ClusterRole Fix (if controller crash-loops)

If the controller keeps restarting due to ClusterWorkflow RBAC, verify and patch:

```bash
# Check if clusterworkflows is in the role
kubectl get clusterrole openchoreo-controller-manager-role -o yaml | grep clusterworkflows

# If missing, the generated role.yaml already has it — re-apply:
kubectl apply -f config/rbac/role.yaml

# Then restart
kubectl rollout restart deploy controller-manager -n openchoreo-control-plane
```

---

## What Changed (Summary)

The `spec.schema` field in ComponentType, Trait, and Workflow CRDs changed from flat fields to a nested `ocSchema` sub-key:

**Before:**
```yaml
spec:
  schema:
    types: ...
    parameters: ...
    envOverrides: ...
```

**After:**
```yaml
spec:
  schema:
    ocSchema:
      types: ...
      parameters: ...
      envOverrides: ...
```

This is a **breaking change** — all YAML files must be updated. No backward compatibility shim.