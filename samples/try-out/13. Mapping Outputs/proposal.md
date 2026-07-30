# Proposal: Workflow Results: Mapping Workflow Engine Outputs to WorkflowRun Status

## Problem

Today, the only output extracted from a workflow engine (Argo) run is the hard-coded `workload-cr` parameter from the `generate-workload-cr` task. This extraction is baked into the controller logic and is invisible in the `WorkflowRun` status — the only trace is a `WorkloadUpdated` condition.

Workflow authors have no declarative way to surface arbitrary outputs (image names, registry URLs, execution metrics, generated manifests, etc.) from their workflow steps into the `WorkflowRun` status. Consumers of `WorkflowRun` (higher-level controllers, CLI tools, UIs) therefore have no structured path to read these values without reaching into the underlying engine resources directly, which breaks the abstraction.

## Goals

1. Allow workflow authors to **declaratively define results** they want surfaced from a workflow run.
2. Store resolved results as **structured data in `WorkflowRunStatus`** so that any consumer can read them without knowledge of the underlying engine.
3. Design the mechanism to be **engine-agnostic** — it must work for Argo Workflows today and be extendable to Tekton Pipelines (or any future engine) without API changes.
4. Support multiple value sources: task/step output parameters, CEL expressions over workflow metadata, and literal values.

**Example in YAML:**

```yaml
kind: Workflow

spec:
  results:
    - name: image
      description: "Full image reference produced by the build"
      valueFrom:
        taskResult:
          task: publish-image
          result: image
    - name: git-revision
      description: "The resolved git commit SHA"
      valueFrom:
        taskResult:
          task: checkout-source
          result: git-revision
    - name: workload
      description: "Generated Workload CR manifest"
      valueFrom:
        taskResult:
          task: generate-workload-cr
          result: workload-cr
    - name: registry-url
      description: "Static registry base URL"
      valueFrom:
        expression: "'ghcr.io/openchoreo/sample-workloads'"
    - name: component-name
      description: "Component name from workflow run metadata labels"
      valueFrom:
        expression: ${metadata.labels['openchoreo.dev/component']}
  # ... rest of spec
```

### 2. `WorkflowRunStatus.Results` — Resolved Output Values

```yaml
kind: WorkflowRun

status:
  conditions: [...]
  tasks: [...]
  results:
    - name: image
      value: "default-greeting-service:v1"
    - name: git-revision
      value: "a1b2c3d4e5f6"
    - name: workload
      value: |
        apiVersion: openchoreo.dev/v1alpha1
        kind: Workload
        ...
    - name: registry-url
      value: "ghcr.io/openchoreo/sample-workloads"
  startedAt: "2025-01-15T10:00:00Z"
  completedAt: "2025-01-15T10:05:30Z"
```

## Engine Abstraction

The key design principle is that `WorkflowResult.ValueFrom.TaskResult` uses **OpenChoreo task names** (the same names that appear in `WorkflowRunStatus.Tasks[]`), not engine-specific node IDs. Each engine adapter is responsible for mapping these abstract task names to engine-specific output lookups:

| Concept | Argo Workflows | Tekton Pipelines |
|---|---|---|
| Task name | `node.DisplayName` | `taskRun.pipelineTaskName` |
| Output parameter | `node.Outputs.Parameters[name].Value` | `taskRun.Status.Results[name].Value` |
