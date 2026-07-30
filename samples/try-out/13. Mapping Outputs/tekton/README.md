# Argo Workflows vs Tekton: Concept Mapping

This document maps Argo Workflow concepts to their Tekton equivalents, focusing on parameter passing, output propagation between steps/tasks, and reusable templates.

## Quick Reference Table

| Concept | Argo Workflows | Tekton |
|---|---|---|
| Reusable template (cluster-scoped) | `ClusterWorkflowTemplate` | `ClusterTask` |
| Reusable template (namespace-scoped) | `WorkflowTemplate` | `Task` |
| Orchestration (task ordering) | Inline `steps` / `dag` in the template | `Pipeline` (separate resource) |
| Execution instance | `Workflow` | `PipelineRun` |
| Input parameters | `arguments.parameters` / `inputs.parameters` | `params` |
| Output values | `outputs.parameters` with `valueFrom.path` | `results` with `$(results.<name>.path)` |
| Pass output to next step | `{{steps.<step>.outputs.parameters.<name>}}` | `$(tasks.<task>.results.<name>)` |
| Access workflow-level param | `{{workflow.parameters.<name>}}` | `$(params.<name>)` (in Pipeline) or `$(params.<name>)` (in Task) |
| Shared storage | `volumeClaimTemplates` on Workflow | `workspaces` + `volumeClaimTemplate` on PipelineRun |
| Secrets as volumes | `volumes[].secret` on template | `volumes[].secret` on Task step, or `workspaces` |
| Service account | `serviceAccountName` on Workflow | `taskRunTemplate.serviceAccountName` on PipelineRun |
| TTL / cleanup | `ttlStrategy.secondsAfterCompletion` | No built-in TTL; use Tekton Results API or CronJob cleanup |
| Reference a cluster template | `templateRef.clusterScope: true` | `taskRef.kind: ClusterTask` |

---

## 1. Defining Parameters (Inputs)

### Argo: Workflow-level parameters

```yaml
# Argo Workflow
spec:
  arguments:
    parameters:
      - name: git-repo
        value: https://github.com/example/repo
      - name: branch
        value: main
```

Templates access these via `{{workflow.parameters.git-repo}}`.

### Tekton: Pipeline params + Task params

In Tekton, params are declared at **two levels** — the Pipeline and each Task. The Pipeline passes values down to Tasks explicitly.

```yaml
# Tekton Pipeline
spec:
  params:
    - name: git-repo
      type: string
    - name: branch
      type: string
      default: main
  tasks:
    - name: checkout
      taskRef:
        name: checkout-source
        kind: ClusterTask
      params:
        - name: git-repo
          value: $(params.git-repo)    # Pipeline param → Task param
        - name: branch
          value: $(params.branch)
```

```yaml
# Tekton ClusterTask
spec:
  params:
    - name: git-repo
      type: string
    - name: branch
      type: string
  steps:
    - name: clone
      image: alpine/git
      script: |
        git clone --branch $(params.branch) $(params.git-repo) /workspace/source
```

The PipelineRun provides actual values:

```yaml
# Tekton PipelineRun
spec:
  pipelineRef:
    name: my-pipeline
  params:
    - name: git-repo
      value: https://github.com/example/repo
    - name: branch
      value: main
```

---

## 2. Defining Outputs and Passing Between Steps

This is the most critical difference. Argo uses file-based outputs declared on templates; Tekton uses **results**.

### Argo: outputs.parameters with valueFrom.path

```yaml
# Argo template
- name: checkout-source
  container:
    image: alpine/git
    command: [sh, -c]
    args:
      - |
        git rev-parse HEAD | cut -c1-8 > /tmp/git-revision.txt
  outputs:
    parameters:
      - name: git-revision
        valueFrom:
          path: /tmp/git-revision.txt
```

The next step consumes it:

```yaml
- name: build-image
  arguments:
    parameters:
      - name: git-revision
        value: '{{steps.checkout-source.outputs.parameters.git-revision}}'
```

### Tekton: results + $(tasks.*.results.*)

```yaml
# Tekton ClusterTask
spec:
  results:
    - name: git-revision
      description: Short commit SHA
  steps:
    - name: clone
      image: alpine/git
      script: |
        git rev-parse HEAD | cut -c1-8 > $(results.git-revision.path)
```

The Pipeline wires results between tasks:

```yaml
# In the Pipeline spec
tasks:
  - name: checkout-source
    taskRef:
      name: checkout-source
      kind: ClusterTask

  - name: build-image
    runAfter:
      - checkout-source
    taskRef:
      name: build-image
      kind: ClusterTask
    params:
      - name: git-revision
        value: $(tasks.checkout-source.results.git-revision)
```

**Key differences:**
- Argo writes to any file path, then maps it in `outputs.parameters[].valueFrom.path`
- Tekton writes directly to `$(results.<name>.path)` (a special termination path)
- Tekton results have a **4096-byte limit per result** (use workspaces for larger data)
- Argo references: `{{steps.<step>.outputs.parameters.<name>}}`
- Tekton references: `$(tasks.<task>.results.<name>)`

---

## 3. Surfacing Outputs to the PipelineRun Status

### Argo

Argo automatically surfaces all outputs in the Workflow status. You can access any step's output from the Workflow status after completion.

### Tekton: Pipeline-level results

You must explicitly declare which task results should be promoted to Pipeline results:

```yaml
# Pipeline spec
spec:
  results:
    - name: image
      description: Published image reference
      value: $(tasks.publish-image.results.image)
    - name: workload-cr
      description: Generated workload CR
      value: $(tasks.generate-workload-cr.results.workload-cr)
```

After the PipelineRun completes, these appear in:

```yaml
status:
  results:
    - name: image
      value: "registry.example.com/my-image:v1-abc12345"
    - name: workload-cr
      value: |
        apiVersion: openchoreo.dev/v1alpha1
        kind: Workload
        ...
```

This is the Tekton equivalent of the `mappings` concept in the OpenChoreo Workflow CR — you explicitly choose which task outputs to expose.

---

## 4. Shared Storage (Workspaces vs VolumeClaimTemplates)

### Argo: volumeClaimTemplates

```yaml
spec:
  volumeClaimTemplates:
    - metadata:
        name: workspace
      spec:
        accessModes: [ReadWriteOnce]
        resources:
          requests:
            storage: 2Gi
  templates:
    - name: my-step
      container:
        volumeMounts:
          - name: workspace
            mountPath: /mnt/vol
```

### Tekton: workspaces

Tekton uses **workspaces** — a first-class abstraction for shared storage.

**Task declares it needs a workspace:**
```yaml
# ClusterTask
spec:
  workspaces:
    - name: source
      description: Shared workspace for source code
  steps:
    - name: clone
      script: |
        git clone ... $(workspaces.source.path)/source
```

**Pipeline binds workspaces across tasks:**
```yaml
# Pipeline
spec:
  workspaces:
    - name: shared-workspace
  tasks:
    - name: checkout
      workspaces:
        - name: source
          workspace: shared-workspace    # same workspace, different binding name
    - name: build
      workspaces:
        - name: source
          workspace: shared-workspace    # tasks share the PVC
```

**PipelineRun provides the actual PVC:**
```yaml
# PipelineRun
spec:
  workspaces:
    - name: shared-workspace
      volumeClaimTemplate:
        spec:
          accessModes: [ReadWriteOnce]
          resources:
            requests:
              storage: 2Gi
```

---

## 5. Reusable Templates (ClusterWorkflowTemplate vs ClusterTask)

### Argo: ClusterWorkflowTemplate

A ClusterWorkflowTemplate can contain multiple templates (steps), an entrypoint, and its own DAG:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ClusterWorkflowTemplate
metadata:
  name: docker
spec:
  entrypoint: build-workflow
  templates:
    - name: build-workflow
      steps:
        - - name: checkout
            template: checkout-source
        - - name: build
            template: build-image
    - name: checkout-source
      container: ...
    - name: build-image
      container: ...
```

A Workflow references it:
```yaml
spec:
  workflowTemplateRef:
    clusterScope: true
    name: docker
```

### Tekton: ClusterTask + Pipeline

Tekton separates these concerns:
- **ClusterTask** = a single reusable task (one container/step group)
- **Pipeline** = orchestration of multiple tasks (equivalent to the entrypoint DAG)

```yaml
# Each Argo template → one ClusterTask
apiVersion: tekton.dev/v1
kind: ClusterTask
metadata:
  name: checkout-source
spec:
  steps:
    - name: clone
      image: alpine/git
      script: ...
```

```yaml
# The entrypoint/DAG → a Pipeline
apiVersion: tekton.dev/v1
kind: Pipeline
metadata:
  name: docker-build
spec:
  tasks:
    - name: checkout
      taskRef:
        name: checkout-source
        kind: ClusterTask
    - name: build
      runAfter: [checkout]
      taskRef:
        name: docker-build-image
        kind: ClusterTask
```

**Key difference:** In Argo, one ClusterWorkflowTemplate bundles everything. In Tekton, tasks are independent and a Pipeline composes them. This makes individual tasks more reusable across different Pipelines.

---

## 6. Secrets

### Argo: volumes on templates

```yaml
- name: checkout-source
  volumes:
    - name: git-secret
      secret:
        secretName: '{{workflow.parameters.git-secret}}'
  container:
    volumeMounts:
      - name: git-secret
        mountPath: /etc/secrets/git-secret
```

### Tekton: volumes on Task steps (same pattern)

```yaml
spec:
  params:
    - name: git-secret
      type: string
  volumes:
    - name: git-secret
      secret:
        secretName: $(params.git-secret)
        optional: true
  steps:
    - name: clone
      volumeMounts:
        - name: git-secret
          mountPath: /etc/secrets/git-secret
```

Alternatively, Tekton can use workspaces bound to Secrets:

```yaml
# In PipelineRun
workspaces:
  - name: git-credentials
    secret:
      secretName: my-git-secret
```

---

## 7. Task Ordering

### Argo: Sequential steps (list of lists)

```yaml
steps:
  - - name: step-1        # First (sequential group)
      template: checkout
  - - name: step-2        # Second (runs after step-1)
      template: build
  - - name: step-3a       # Third group — step-3a and step-3b run in PARALLEL
      template: test
    - name: step-3b
      template: lint
```

### Tekton: runAfter + implicit parallelism

```yaml
tasks:
  - name: checkout
    taskRef: ...

  - name: build
    runAfter: [checkout]     # Sequential: waits for checkout
    taskRef: ...

  - name: test
    runAfter: [build]        # Parallel: test and lint both wait for build
    taskRef: ...

  - name: lint
    runAfter: [build]        # Parallel: runs alongside test
    taskRef: ...
```

Tasks without `runAfter` or with the same `runAfter` targets run in parallel by default.

---

## 8. Complete Flow Summary

```
Argo                                    Tekton
────                                    ──────
ClusterWorkflowTemplate                 ClusterTask (one per template)
  ├─ template: checkout-source    →       ClusterTask: checkout-source
  ├─ template: build-image        →       ClusterTask: docker-build-image
  ├─ template: publish-image      →       ClusterTask: publish-image
  └─ template: generate-workload  →       ClusterTask: generate-workload
  └─ entrypoint: build-workflow   →     Pipeline: docker-build-pipeline
                                            (defines task order + param wiring)

Workflow (execution)               →    PipelineRun
  ├─ arguments.parameters          →      params
  ├─ serviceAccountName            →      taskRunTemplate.serviceAccountName
  ├─ volumeClaimTemplates          →      workspaces[].volumeClaimTemplate
  └─ workflowTemplateRef           →      pipelineRef

Output passing:
  {{steps.X.outputs.parameters.Y}} →    $(tasks.X.results.Y)

Surfacing outputs to status:
  (automatic in Argo)              →    Pipeline.spec.results (explicit declaration)
```

## 9. Limitations to Be Aware Of

| Limitation | Details |
|---|---|
| **Result size** | Tekton results are limited to **4096 bytes** per result. For larger outputs (like a full workload CR YAML), write to a workspace file instead and read it in downstream tasks. |
| **No built-in TTL** | Tekton doesn't have `ttlStrategy`. Use Tekton Results API, or a CronJob to clean up old PipelineRuns. |
| **ClusterTask deprecation** | `ClusterTask` is deprecated in Tekton v0.44+. The replacement is [Tekton Resolver](https://tekton.dev/docs/pipelines/cluster-resolver/) with `resolver: cluster` in `taskRef`. See below. |
| **No direct step-to-step output** | Within a single Task, steps share a `/workspace` directory. Between Tasks in a Pipeline, you must use `results` or shared workspaces. |

### Using Resolvers (post-deprecation of ClusterTask)

```yaml
# Instead of:
taskRef:
  name: checkout-source
  kind: ClusterTask

# Use:
taskRef:
  resolver: cluster
  params:
    - name: kind
      value: task
    - name: name
      value: checkout-source
    - name: namespace
      value: tekton-tasks    # namespace where the Task lives
```