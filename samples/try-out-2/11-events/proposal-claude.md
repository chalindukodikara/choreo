# Proposal: Platform Event & Notification System

**Status:** Draft for discussion (revision 4)
**Author:** Claude
**Related:** `TASK.md`, `proposal-codex.md`, `how-claude.md` (this folder), `samples/component-alerts/*`

> **Revision history.**
> - **r1** — first sketch (central informer layer, ad-hoc event names). Superseded.
> - **r2** — corrected to **producer-owned events**, a **durable outbox**,
>   **CloudEvents 1.0**, deterministic IDs (ideas from `proposal-codex.md`).
> - **r3** — made **stateful / aggregate events** first-class (counts, windows,
>   digests, dead-man, firing/resolved) via an **aggregate evaluator** stage.
> - **r4 (this)** — **slims the footprint from 5 CRDs to 2** and to **one new
>   deployment**: destinations are inlined (drop `EventDestination`), delivery
>   state moves onto `PlatformEvent.status` (drop `EventDelivery`), and aggregate
>   windows start **in-memory, rebuilt from retained events** (drop
>   `AggregateState`). The heavier "split" variants are kept as an explicit
>   **scale-up path** (§A), not the default. Nothing is implemented yet, so this
>   intentionally breaks r3's API.

---

## 1. Summary

OpenChoreo has no first-class way to say **"when _X_ happens (or happens _N_
times), do _Y_"** — where _X_ is a lifecycle transition (a build fails, a
component/project is created, a workload becomes unhealthy) and _Y_ is a delivery
(email, webhook, page) or a bounded in-cluster action (trigger a build).

This proposal introduces a **deliberately lightweight, producer-driven event
system** — **one public CRD, one internal record, one new deployment**:

1. **`EventSubscription`** (the only public CRD) — selects events, applies a CEL
   filter, optionally **aggregates** (count/rate/window/dedup), and declares
   actions with an **inline** webhook/email destination.
2. **Event producers in the owning controllers** — a small **library** (not a new
   deployment) that emits sanitized, versioned CloudEvents on authoritative
   transitions.
3. **`PlatformEvent`** (the only internal record) — the durable outbox entry; it
   also **carries per-sink delivery state in its status**, so there is no separate
   delivery CRD.
4. **One eventing deployment** — runs the subscription reconciler, the router, the
   delivery workers, and (when enabled) the aggregate evaluator. The evaluator's
   window state lives **in memory, rebuilt from retained `PlatformEvent`s**.
5. **Reuse of existing delivery code** (observer's email/webhook senders, CEL
   templating, secrets) — *not* a fork of the observability channel.

**One-line recommendation:** ship the smallest thing that is still reliable —
**producers + one `PlatformEvent` outbox + one eventing deployment**, driven by a
single **`EventSubscription`** CRD; reuse the observer's senders; keep **Argo
Events** and the **`event-forwarder`** off the public path. Split records/CRDs
back out only when measured volume demands it (§A).

---

## 1a. CRD & component budget (what "lightweight" means here)

| | r3 (rigorous) | **r4 (default, lightweight)** | Split back out when… (§A) |
|---|---|---|---|
| **Public CRDs** | 2 — `EventSubscription`, `EventDestination` | **1 — `EventSubscription`** (inline destination) | a destination is reused by many subs / needs central rotation |
| **Internal records** | 3 — `PlatformEvent`, `EventDelivery`, `AggregateState` | **1 — `PlatformEvent`** (delivery state in `.status`) | high fan-out causes status-write contention; long windows exceed retention |
| **New deployments** | 1 eventing service | **1 eventing deployment** (same) | — |
| **Producers** | library in existing controllers | **same** (no new deployment) | — |
| **Total CRDs** | **5** | **2** | grows toward 5 only on demand |

The reductions and their trade-offs are justified inline where each collapse
happens (§6.1, §8, §9); the full scale-up criteria are in **§A**.

---

## 2. Motivating scenarios

| # | Event (the "when") | Delivery (the "how") | What it stresses |
|---|--------------------|----------------------|------------------|
| 1 | `Component` created | Trigger a build (`WorkflowRun`) | Event → **in-cluster action** |
| 2 | Build (`WorkflowRun`) failed | Push to an external system | Webhook / bus sink |
| 3 | Build failed | Send an email | Email sink |
| 4 | Workload unhealthy **in a given environment** | Call an external system (page) | **Filter** on environment |
| 5 | `Project` created | Emit an event | Broad, type-level subscription |
| 6 | Project created **with a specific name** | Emit an event | **CEL filter** on name |
| 7 | **>5 build failures in 10 min** (for a component/env) | Page on-call once | **Stateful: count + window** |
| 8 | … future event types | … | Extensibility |

Cases 1–6 are **discrete** (one event → decide). Case 7 is **stateful** — it
needs memory across events. §7 covers the whole family it belongs to.

Invariant: users declare **(a) which events**, **(b) optionally how to aggregate
them**, and **(c) how to deliver** — declaratively, no Go.

---

## 3. Positioning against what already exists

### 3.1 vs. observability alerts

The observability plane (`ObservabilityAlertRule` +
`ObservabilityAlertsNotificationChannel` + `internal/observer/notifications/*`)
already does **window + threshold** evaluation — but on **telemetry**
(logs/metrics/budgets). It is blind to **control-plane facts** — CR creation,
condition/phase transitions, promotions — which never reach OpenSearch/Prometheus.

r3 adds windowing/counting too (§7), so the boundary must be crisp:

| | Observability alerting | Lifecycle eventing (this proposal) |
|---|---|---|
| Input | Logs/metrics/budgets sampled over time | Discrete OpenChoreo **domain transitions** |
| Windowing | Over metric/log series | Over the **event stream** (§7) |
| Example | "error rate >5% for 10 min" | ">5 build **failures** in 10 min" |
| State | Firing/resolved alert | Event occurrence, plus firing/resolved for aggregates |
| Owner | Observability plane | The controller/plane that owns the transition |

**Guiding principle — _reuse the delivery half; own the trigger half; borrow
alerting's concepts, not its telemetry pipeline._** Share the SMTP/webhook
senders and CEL templating. Reuse the *concepts* of window/threshold/firing/
resolved/cooldown for §7. Do **not** reuse `ObservabilityAlertsNotificationChannel`
unchanged (environment-scoped, alert-coupled templates); a unified generic
`NotificationChannel` is a **separate later migration**.

### 3.2 vs. the existing `event-forwarder`

`internal/eventforwarder/` watches OpenChoreo resources and pushes hints to
Backstage. It is **deliberately lossy**: 1s debounce (`forwarder.go:41`), an
in-memory queue that **drops under overload** (`dispatcher/dispatcher.go`), and
best-effort single delivery — safe only because Backstage periodically re-syncs.
Those trade-offs are right for catalog invalidation and **wrong for user
automation** (side effects need durability, authz, retry, dedup, audit).
**Decision:** keep its catalog path unchanged; reuse only generic pieces (HTTP
hardening, worker-pool patterns). Do **not** route lifecycle events through it.

---

## 4. Why not derive events from Kubernetes watches

Producing **domain** events from raw informer add/update/delete is wrong, for
reasons concrete in this codebase:

- An informer's initial **list re-fires add-handlers for existing objects** → a
  restart would re-emit `*.created` for every project/component.
- Updates are **coalesced**; relists produce spurious updates — no reliable "what
  transitioned" signal.
- A watch reports **mutation, not intent**. A `WorkflowRun` update isn't a build
  failure; the signal is `ConditionWorkflowFailed → True`
  (`internal/controller/workflowrun/controller.go:312`, `controller_conditions.go:16`).
- Watch history is bounded; raw objects **leak internal fields**.

**The owning controller already computes the authoritative transition** and is
the right producer.

---

## 5. Event envelope — CloudEvents 1.0

Adopt CloudEvents 1.0 JSON (from `proposal-codex.md`). Transport-neutral, carries
identity fields for dedup.

```json
{
  "specversion": "1.0",
  "id": "evt-<deterministic>",
  "source": "https://api.openchoreo.dev/organizations/acme",
  "type": "dev.openchoreo.workflowrun.failed.v1",
  "subject": "projects/shop/components/checkout/workflow-runs/build-7f5d",
  "time": "2026-07-01T09:40:31Z",
  "data": { "project": "shop", "component": "checkout", "reason": "WorkflowFailed",
            "message": "build task exited with status 1" }
}
```

- `id` is **deterministic** (resource UID + type + transition identity) → a
  reconcile retry re-derives the same id; `AlreadyExists` == success.
- `type` is a versioned OpenChoreo event type.
- `data` is an **allowlisted, versioned** payload — never a whole CR, never secrets.
- Consumers dedup on `(source, id)`.

### Event catalog (initial)

**Base events** (from producers):

| Scenario | Event type | Emitted by / when |
|---|---|---|
| Component created (#1) | `…component.created.v1` | Component controller, first observation |
| Build failed (#2,#3,#7) | `…workflowrun.failed.v1` | WorkflowRun controller, `ConditionWorkflowFailed → True` |
| Build succeeded | `…workflowrun.succeeded.v1` | WorkflowRun controller, `ConditionWorkflowSucceeded → True` |
| Workload unhealthy (#4) | `…deployment.degraded.v1` / `.recovered.v1` | Deployment-status reconciler, health transition |
| Project created (#5,#6) | `…project.created.v1` | Project controller |

**Derived events** (from the aggregate evaluator, §7) — first-class, versioned,
reusable:

| Derived event | Meaning |
|---|---|
| `…workflowrun.failure_threshold_exceeded.v1` | ≥N failures matched a window rule |
| `…workflowrun.failures_recovered.v1` | a previously-tripped rule returned to normal |
| `…<type>.digest.v1` | a batched summary of events in a window |
| `…<type>.absent.v1` | an expected event did **not** occur within a window (dead-man) |

The catalog is explicit and versioned; avoid a catch-all `*.updated`.

---

## 6. Public API

### 6.1 `EventSubscription`

Selects event types, optionally filters (CEL over the envelope), optionally
**aggregates** (§7), and declares bounded actions. **No `aggregate` block ⇒
today's discrete, per-event behavior.**

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: EventSubscription
metadata: { name: too-many-build-failures, namespace: acme }
spec:
  eventTypes: [dev.openchoreo.workflowrun.failed.v1]
  filter: event.data.project == "shop"
  aggregate:                              # OPTIONAL — omit for per-event delivery
    groupBy: [component, environment]     # a separate counter per component+env
    window: { type: sliding, size: 10m }  # sliding | tumbling | consecutive
    condition: "count >= 5"               # trips the rule
    cooldown: 1h                          # re-fire suppression (flood control)
    emitResolved: true                    # also fire when it returns to normal
  actions:
    - name: page-oncall
      webhook:                            # INLINE destination (no separate CRD)
        url: https://events.pagerduty.com/...
        headers:
          Authorization: { valueFrom: { secretKeyRef: { name: pd-creds, key: token } } }
        timeout: 10s
      delivery: { maxAttempts: 8, initialBackoff: 2s, maxBackoff: 10m }
status:
  observedGeneration: 1
  conditions: [{ type: Ready, status: "True", reason: Valid }]
```

Invariants:
- `eventTypes` accepts only catalog-registered types; `filter` is type-checked,
  cost-bounded, may not fetch resources or do network calls.
- Each action sets **exactly one** delivery target: an inline `webhook`, an inline
  `email`, or a supported internal action (§6.2). Credentials are **always** Secret
  refs, so rotation lives in one Secret even though the URL/headers are inline.
- A subscription generation is eligible only for events created **after** it
  becomes `Ready` — retries can't backfill history.
- Delivery counts live in metrics, **not** subscription status.

> **Why inline instead of an `EventDestination` CRD?** For the volumes lifecycle
> events run at, a separate destination CRD is mostly ceremony — one more object
> to create, RBAC, and reconcile. Inlining drops a public CRD. The cost is that a
> webhook shared by many subscriptions repeats its `url`/`headers` (creds still
> centralize in the Secret). If that repetition becomes painful, promote it to a
> reusable `EventDestination` + `destinationRef` later — an **additive** change
> (§A), not a rewrite.

### 6.2 Internal automation actions (scenario #1)

`workflowRun: { useComponentWorkflow: true }` creates a build from the
component's configured workflow — an **allowlisted** action, not arbitrary K8s.
Authorized as the subscription owner (`internal/authz`), stamped with the source
event id, deterministic name so redelivery can't spawn duplicate builds.

---

## 7. Stateful / aggregate events (the "count/window" family)

Cases like #7 need **memory across events** — the stateless CEL `filter` sees one
event at a time and cannot count, so aggregation is a **distinct stage**.

### 7.1 The family (why one mechanism covers many asks)

| Pattern | Example | Expressed as |
|---|---|---|
| **Threshold / count** | >5 build failures in 10 min | `window` + `count >= 5` |
| **Rate / percentage** | failure rate >20% | `count(failed)/count(total) > 0.2` (needs both streams) |
| **Consecutive streak** | 3 failures in a row per component | `window.type: consecutive` |
| **Flood suppression / dedup** | failed 50×, notify once/hour | `cooldown: 1h` |
| **Digest / batch** | one hourly email of all failures | `window.type: tumbling` + digest action |
| **Absence / dead-man** | no successful build in 24h; expected event missing | `absence` rule (timer-driven) |
| **Firing / resolved** | notify on "bad", again on "recovered" | `emitResolved: true` |
| **Escalation** | still failing 30 min after first alert → page | open-state + timer (built on the above) |
| **Correlation across types** | build ok but deploy degraded within 10 min | join two event streams (advanced) |

**Absence** is special: it can't be triggered *by* an event (the point is that
none arrived) — it needs the evaluator's own **clock**. That alone justifies a
stateful evaluator rather than pure event matching.

### 7.2 How it works

The **aggregate evaluator** is a consumer *and* producer:

1. It reads matching base events from the outbox.
2. Per `groupBy` key, it maintains window state (a counter/ring buffer/timestamp).
3. When `condition` trips, it **emits a derived event** (e.g.
   `…failure_threshold_exceeded.v1`) back into the outbox.
4. The derived event flows through the **same router → delivery** path as any
   event — so aggregates are observable, auditable, and reusable by other subscriptions.
5. `cooldown` suppresses re-firing; `emitResolved` fires a `…recovered.v1` when
   the group falls back below the condition.

Producers stay dumb. Discrete eventing (§6, no `aggregate`) is untouched.

```
 base events ─▶ [ outbox ] ─▶ aggregate evaluator ─▶ derived events ─▶ [ outbox ] ─▶ router ─▶ delivery
                    ▲                (stateful)                              │
                    └──────────── discrete subscriptions skip the evaluator ─┘
```

### 7.3 Semantics that must be nailed (the real cost)

- **Window types:** `sliding` (last T), `tumbling` (fixed buckets → digests),
  `consecutive` (last-N outcomes, no clock).
- **Fire-once + cooldown/hysteresis:** otherwise a flapping build sends 50 pages;
  `emitResolved` gives alert-style firing/resolved pairs.
- **State durability without a new CRD:** counters/windows must survive restarts,
  but the lightweight design keeps them **in memory and rebuilds from retained
  `PlatformEvent`s** on restart (no `AggregateState` CRD) — see §9. Promote to a
  persisted record only for windows longer than retention (§A).
- **Cardinality limits:** a counter per `component × environment × window` can
  explode; enforce a max group count per subscription.
- **Late/out-of-order events:** windows must tolerate a bounded lateness skew.

### 7.4 Two ways to build it (pick in the ADR)

1. **Self-contained evaluator in the eventing service** *(recommended)* — owns
   its window state; works even without the observability plane installed;
   reuses alerting's *concepts* (window/threshold/firing/resolved/cooldown) but
   not its telemetry pipeline. More new code (state store, windows).
2. **Feed control-plane events as counters into the observability alert engine** —
   reuse its windowing/thresholds, route the resulting alert back as a derived
   event. Less new code, but couples eventing to the observability plane and adds
   latency; can't count things the obs plane never sees.

Recommendation: **option 1**, phased after discrete eventing works (§13).

---

## 8. Architecture

One new **eventing deployment** hosts the reconciler, evaluator, router, and
workers. Producers are a **library** compiled into the existing controllers — no
extra deployment.

```
 owning controllers        ┌──── eventing deployment (1 new component) ─────────────┐
 (producer library)        │                                                        │
  Component  ─┐            │   ┌────────────────────────────┐                        │
  WorkflowRun ┼─ CloudEvent┼──▶│  PlatformEvent  (outbox +  │◀── derived events ──┐  │
  Deployment ─┤ (idempotent│   │  per-sink delivery state   │                     │  │
  Project    ─┘  create)   │   │  in .status)               │                     │  │
                           │   └──────────┬─────────────────┘                     │  │
                           │       discrete│      │aggregate                       │  │
                           │  (skip eval)  ▼      ▼                                │  │
                           │        ┌──────────┐ ┌────────────────────┐            │  │
   EventSubscription ──────┼───────▶│  router  │ │ aggregate evaluator│────────────┘  │
   (the only public CRD)   │        │+ workers │ │ (in-memory windows,│  emits derived │
                           │        └────┬─────┘ │  rebuilt on restart)│  events        │
                           │             │       └────────────────────┘                │
                           └─────────────┼──────────────────────────────────────────────┘
                                         ▼  update PlatformEvent.status per sink; workers lease
                          webhook(HMAC) / email / platform action / dead-letter + replay
```

**Producers** detect the authoritative transition, build a sanitized CloudEvent,
and **idempotently create** a `PlatformEvent`. They never call a webhook/SMTP
inside a reconcile loop. Because producers are reconcilers, a crash before the
write is recovered on the next reconcile (same deterministic id → `AlreadyExists`).

**Subscription reconciler** validates event types, CEL filter, aggregate rules,
inline destinations (Secret access), tenant scope; publishes `Ready`; keeps the
router's compiled cache.

**Aggregate evaluator** (§7) consumes base events, keeps window state **in memory**
(rebuilt from retained `PlatformEvent`s on restart), emits derived events back as
new `PlatformEvent`s.

**Router + workers** watch `PlatformEvent`s, match subscriptions, and for each
matched action attempt delivery, recording **per-sink state in the event's
`.status`** (no separate delivery object) with backoff + jitter, dead-letter +
replay on exhaustion.

Production, aggregation, and routing are all **idempotent** (deterministic ids,
`AlreadyExists` == success). Delivery stays **asynchronous** so a slow endpoint
never blocks reconciliation.

---

## 9. Durability & delivery semantics (at-least-once)

A **single** internal, non-public CRD in a system namespace is the K8s-native
store — it is both the outbox **and** the delivery ledger:

| Record | Immutable identity | Mutable state |
|---|---|---|
| `PlatformEvent` | CloudEvent envelope (deterministic `id`), producer, time | `expiry`; and a **`status.deliveries[]`** list — one entry per matched (subscription, action) with `phase, attempts, nextAttemptTime, lastError` |

```
per-sink delivery entry:  Pending → Delivering → Succeeded
                             │  └─(transient)→ RetryPending → Delivering
                             └─(permanent / exhausted)→ DeadLettered →(replay)→ Pending
```

- The event is stored **before** any delivery; workers claim a delivery entry with
  optimistic concurrency + a bounded lease → survive restarts; redelivery after a
  lost lease is the documented source of at-least-once. Consumers dedup by `id`.
- **Delivery state lives on `PlatformEvent.status`** (dropping the `EventDelivery`
  CRD). Trade-off: many sinks on one event write the same object's status
  concurrently, so the router serializes per-event status updates. Fine at
  lifecycle-event volume; if fan-out or churn causes contention, split entries
  into their own `EventDelivery` records (§A) — the state machine is unchanged.
- **Aggregate durability without a CRD:** window state is **in memory**, rebuilt on
  restart by replaying retained `PlatformEvent`s within the window (retention must
  cover the longest window). A brief post-restart rebuild may re-emit or delay a
  threshold event; cooldown + deterministic derived-event ids keep that idempotent.
  Promote to an `AggregateState` CRD only for windows longer than retention or
  very high group cardinality (§A).
- **Upgrade safety:** an installation **eventing activation timestamp** makes
  producers emit `*.created` only for objects created at/after activation.
- **Deletion events** need a finalizer/pre-delete record — deferred.
- If volume outgrows K8s records, put the same producer/evaluator/router
  interfaces over NATS JetStream/Kafka without changing the public CRD.

---

## 10. Argo Events assessment (task question #3)

OpenChoreo depends on Argo **Workflows**, not Argo Events. Argo Events offers
real parts (resource EventSources, Sensors with retry/dead-letter, JetStream
EventBus) **but**: a raw resource UPDATE isn't a stable domain event; bodies leak
internal shape; **it has no stateful count/window operator** for case #7; email
isn't first-class; generic K8s-object triggers are a privilege-escalation
surface; the EventBus is namespaced while our events cross planes; public Argo
CRDs would couple our API to a dependency. **Decision: do not expose Argo Events
as the public API.** Optional internal adapter only.

---

## 11. Security & tenancy

- Namespace scope; cross-org needs a cluster-admin API. Inline destinations only
  read Secrets in the subscription's own namespace; Secret reads narrowly authorized.
- HTTPS by default; **SSRF protection** (block link-local/metadata/loopback);
  **HMAC-sign** webhooks; redact credentials/sensitive fields from status/logs/metrics.
- Payload-size, template-cost, retry, concurrency, **groupBy-cardinality**, and
  per-tenant rate limits. Audit subscription changes, deliveries, replays, actions.
- Internal actions run under the owner's permissions, not controller credentials.

---

## 12. Answers to the task's open questions

1. **New CR?** Yes — **one** public CRD, `EventSubscription` (inline destination),
   plus **one** internal record, `PlatformEvent`. Two CRDs total. (§1a, §6)
2. **Fit with controller architecture?** Owning controllers **produce** events (a
   library, no new deployment); one eventing deployment aggregates + delivers. No
   central informer-derives-events layer. (§4, §7, §8)
3. **Reuse Argo Events?** No on the public path (and it can't do #7 anyway). (§10)
4. **Controller or service?** Both — controllers produce + reconcile config;
   evaluator + workers run async. (§8)
5. **Different from observability notifications?** Different producer + input;
   eventing windows over the **event stream**, alerting over **telemetry**;
   shares senders + windowing *concepts* only. (§3.1, §7)
6. **Architecture?** §5 envelope + §7 aggregation + §8 diagram + §9 durability.

---

## 13. Rollout plan

**Phase 1 — discrete contract + reliable webhooks.** Catalog + schemas for
project/component created, workflowrun failed/succeeded, deployment
degraded/recovered; `EventSubscription` (no `aggregate` yet, **inline webhook**);
producer library + the `PlatformEvent` outbox; emit from Project, Component,
WorkflowRun, deployment-status controllers; CEL filter, retries (via
`status.deliveries[]`), dead-letter, replay, HMAC, metrics, audit.

**Phase 2 — email + automation.** Inline email destination (shared sender);
allowlisted `WorkflowRun` action with idempotency + authz; per-tenant quotas.

**Phase 3 — stateful aggregation (§7).** `aggregate` block + evaluator with
**in-memory windows** (rebuilt from retained events): threshold/count,
sliding/tumbling/consecutive, cooldown, `emitResolved`, then digest and
absence/dead-man. Cardinality limits.

**Phase 4 — scale + integrations (only if measured need).** The §A splits
(`EventDestination`, `EventDelivery`, `AggregateState`), optional JetStream/Kafka,
optional Argo Events adapter, and a **separate** generic `NotificationChannel`
migration with observability.

### Walking skeletons (do first per phase)
- Phase 1: `workflowrun.failed → email`, from the existing `ConditionWorkflowFailed`.
- Phase 3: `>5 workflowrun.failed in 10m (per component) → page`, proving the
  evaluator + in-memory window rebuild-on-restart end-to-end.

---

## 14. Acceptance criteria (selected)

- Restarting a controller/eventing pod emits **no** false `*.created` events.
- A `WorkflowRun` produces **one** logical failed event per failure transition.
- A webhook outage is retried across restarts; redelivery keeps the same id.
- **A ">5 in 10m" rule fires once per window+cooldown, survives a pod restart
  mid-window, and (if enabled) emits a matching `recovered` event.**
- A tenant cannot subscribe to another tenant's events or reference its Secrets.
- Payloads contain no whole CRs or credentials; delivery never blocks reconcile.
- The Backstage `event-forwarder` keeps working independently.

---

## 15. Open questions

1. Which controller owns the canonical `deployment.degraded` transition across a
   remote data plane?
2. `PlatformEvent` retention — must be ≥ the longest aggregate window (§9), since
   window state is rebuilt from retained events. What default?
3. Does `*.created` mean API persistence, initial reconcile, or readiness?
4. Which workflow may a `component.created` subscription invoke, and who configures it?
5. For §7: which window types ship first, and what are the default cardinality /
   lateness limits? Do we build the self-contained evaluator or reuse the alert
   engine (§7.4)?
6. What measured event rate should trigger migration from K8s records to a broker?

---

## 16. Pointers for the implementer

- Emit points: `internal/controller/workflowrun/controller.go:309-313`,
  `controller_conditions.go` (`ConditionWorkflowFailed`/`Succeeded`); reuse
  `controller.MarkTrueCondition` sites as the producer hook.
- Status/watch patterns: `internal/controller/*/controller_status.go`,
  `controller_watch.go`.
- Reusable senders/templating: `internal/observer/notifications/{email,webhook,sender}.go`,
  `internal/observer/service/notification_dispatch.go`, `internal/template/`.
- Windowing **concepts** to borrow (not the telemetry pipeline):
  `internal/observer/service/alerts*.go`, `api/v1alpha1/observabilityalertrule_types.go`.
- Lossy forwarder to coexist with: `internal/eventforwarder/`.
- Authz: `internal/authz/`. Manager wiring: `cmd/main.go`.

---

## Appendix A — Scale-up path (when to add CRDs back)

The default (§1a) is 2 CRDs. Each collapse is an **additive** upgrade later, so
starting small costs nothing architecturally. Add a split **only when a measured
signal** says so:

| Split to add | Trigger to add it | What changes | Backward-compatible? |
|---|---|---|---|
| **`EventDestination` + `destinationRef`** | The same webhook/email is copied across many subscriptions, or you need central credential/endpoint rotation and reuse | Add the CRD; `EventSubscription` gains `destinationRef` **alongside** inline destinations | Yes — inline stays valid |
| **`EventDelivery` record** | High fan-out (one event → many sinks) causes `PlatformEvent.status` write contention, or you need per-sink replay/audit at scale | Move `status.deliveries[]` entries into their own objects; the delivery state machine is unchanged | Yes — same phases/semantics |
| **`AggregateState` CRD** | Windows longer than event retention, or group cardinality large enough that in-memory rebuild is slow/expensive | Persist evaluator counters instead of rebuilding from events | Yes — evaluator API unchanged |
| **Broker (NATS JetStream / Kafka)** | Event/delivery churn exceeds what etcd-backed records handle comfortably | Put the same producer/evaluator/router interfaces over the broker | Yes — public CRD + CloudEvents schema unchanged |

**Rule of thumb:** ship the 2-CRD version, watch the operational signals in §11
(status-write rate, queue depth, cardinality, retention pressure), and split the
*one* dimension that actually hurts — never all four preemptively.
```
