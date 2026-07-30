# OpenClaw on OpenChoreo — article workspace

Pre-publication scratch area for an article about running [OpenClaw](https://docs.openclaw.ai/) (a self-hosted ChatOps gateway) as the conversational surface for OpenChoreo's CI/CD and component-creation workflows. Audience is platform engineers; the developer experience is the *outcome* the article describes, not the audience it is written for.

## Files

| File | What it is |
|---|---|
| [`TASK.md`](TASK.md) | Phase-by-phase plan + verified status from the local test round. Read this for the audit trail of what was actually run, what passed, and what is deferred until a real OpenClaw image / Slack workspace exists. |
| [`openclaw-componenttype.yaml`](openclaw-componenttype.yaml) | Namespace-scoped `ComponentType/openclaw-gateway` in the `default` namespace. Renders Deployment, Service, ConfigMap (config.json from `parameters.enabledChannels`), ExternalSecret (pulls per-channel bot tokens + agent API key from the ClusterSecretStore), and an HTTPRoute for the Web Control UI. Locks `replicas == 1` (singleton gateway) and requires at least one enabled channel. |
| [`smoke-component.yaml`](smoke-component.yaml) | Happy-path Workload + Component pair that exercises the CT end-to-end with `enabledChannels: [slack, discord]` and `autoDeploy: true`. |
| [`smoke-guardrails.yaml`](smoke-guardrails.yaml) | Two negative-test components — one with empty `enabledChannels`, one with no workload endpoints. Both expected to fail; see TASK.md Phase 4 for the exact rejection messages. |

## Reproducing the test round locally

Assumes the OpenChoreo single-cluster k3d setup from https://openchoreo.dev/docs/getting-started/try-it-out/on-k3d-locally/ is already up.

```bash
# Seed the OpenBao keys the CT's defaults reference.
kubectl exec -n openbao openbao-0 -- sh -c '
  export BAO_ADDR=http://127.0.0.1:8200 BAO_TOKEN=root
  for k in agent-api-key slack-bot-token discord-bot-token telegram-bot-token \
           teams-bot-token whatsapp-bot-token matrix-bot-token \
           signal-bot-token imessage-bot-token; do
    bao kv put secret/openclaw-$k value="dev-$k-placeholder" > /dev/null
  done
'

# Apply the ComponentType.
kubectl apply -f samples/try-out-2/1-openclaw/openclaw-componenttype.yaml

# Happy-path smoke test (renders 6 K8s objects into the data-plane namespace).
kubectl apply -f samples/try-out-2/1-openclaw/smoke-component.yaml

# Guardrail negatives — both should fail at the right layer.
kubectl apply -f samples/try-out-2/1-openclaw/smoke-guardrails.yaml
kubectl get component bad-no-channels bad-no-endpoint \
  -o jsonpath='{range .items[*]}{.metadata.name}: {.status.conditions[0].message}{"\n"}{end}'
```

Teardown is whole-cluster: `k3d cluster delete openchoreo`.

## What this directory does *not* contain (yet)

- A real OpenClaw container image (the smoke component intentionally points at a placeholder).
- A Casbin policy mapping a chat-platform identity to an OpenChoreo subject.
- Live demo transcripts from a Slack/Discord workspace.

These are the article's content, not its prerequisites — see TASK.md Phase 5.
