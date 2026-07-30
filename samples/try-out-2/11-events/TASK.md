# Task: Platform Event & Notification System

## Goal

Platform engineers and developers want to react to **things that happen inside
OpenChoreo's control plane** — a component is created, a build fails, a workload
won't come up, a project appears — and get that fact delivered somewhere they
care about (an email, a webhook into their own system, an on-call phone call,
another OpenChoreo action).

Today there is **no first-class way to say _"when X happens, notify Y"_** for
control-plane lifecycle events. This task is to design (and then incrementally
build) that system.

---

## Motivating scenarios

These are the concrete asks that started this task. They are deliberately varied
so the design is stress-tested against real needs, not one narrow case.

| # | Event (the "when") | Delivery (the "how") | Notes |
|---|--------------------|----------------------|-------|
| 1 | A `Component` is created | Trigger a build (`WorkflowRun`) | Event → **in-cluster action**, not just a notification |
| 2 | A build (`WorkflowRun`) fails | Push the event into an external system | Webhook / message bus |
| 3 | A build fails | Send an email | Email channel |
| 4 | A deployment workload fails in a given environment | Call an external system (paging) | Filtered by **environment** |
| 5 | A `Project` is created | Emit an event | Broad, type-level subscription |
| 6 | A project with a **specific name** is created | Emit an event | Same type as #5 but **filtered by name/selector** |
| 7 | **More than 5 build failures in 10 minutes** (for a component/env) | Page on-call, once | **Stateful** — needs counting over a time window, not single-event matching |
| 8 | … (open-ended — the system must extend to new event types) | | |

**The common shape:** users declare (a) **which events** they care about
(by type, and by a filter/selector such as name, namespace, project, or
environment), (b) **optionally how to aggregate them** (count/rate/window — see
scenario #7 and the family it belongs to: consecutive streaks, flood
suppression, hourly digests, dead-man "no event in 24h", firing/resolved), and
(c) **how those events should be delivered**. The platform must let them express
all three without writing Go.

> Scenarios #1–#6 are **discrete** (one event → decide). Scenario #7 is
> **stateful** and is a different class of problem: a single-event filter cannot
> count across events, so the design needs an aggregation stage. See
> `proposal-claude.md` §7 for the full family and the proposed `aggregate`
> mechanism.

---

## Why the existing observability alerts are *not* this

OpenChoreo already ships a notification path, and this task must be positioned
against it so we don't duplicate or confuse the two.

**What already exists (observability plane):**

- `ObservabilityAlertRule` (`api/v1alpha1/observabilityalertrule_types.go`) and
  the `observability-alert-rule` `Trait` (`samples/component-alerts/alert-rule-trait.yaml`)
  — fire on **log / metric / budget** conditions evaluated against telemetry
  (e.g. "CPU > 80% for 5m", "error-log rate > N").
- `ObservabilityAlertsNotificationChannel`
  (`api/v1alpha1/observabilityalertsnotificationchannel_types.go`) — an
  **environment-scoped** channel (`email` / `webhook`) with CEL-templated
  subject/body/payload and secret-backed auth.
- Dispatch code in the **observer** service:
  `internal/observer/service/notification_dispatch.go`,
  `internal/observer/notifications/{email,webhook,sender}.go`.

**The gap:** that path only fires on **telemetry** (logs, metrics, traces,
budget) sampled in the observability plane. It has **no visibility into
control-plane lifecycle** — the creation of a CR, a reconcile failure, a build
result, a promotion. Those facts live in the **control plane** (CR events,
`.status` conditions, `WorkflowRun.status.phase`), never in OpenSearch/Prometheus.

So the two systems are complementary:

| | Observability alerts (exists) | Platform events (this task) |
|---|---|---|
| Source of truth | Telemetry (logs/metrics/traces/budget) | Control-plane CR lifecycle & status |
| Evaluated by | Observer service, on a schedule/window | Controllers / an event dispatcher, on reconcile |
| Trigger example | "error rate > 5% over 5m" | "`WorkflowRun` moved to `Failed`" |
| Scope | Per component/environment | Any CR: project, component, build, workload |

**Design principle:** *reuse the delivery half, replace the trigger half.* The
channel abstraction, CEL templating, secret handling, and email/webhook senders
are already good; we should generalize and reuse them rather than build a second
notification stack.

---

## Open design questions (to answer in the proposal)

1. **New CR(s)?** Do we introduce a new `EventSubscription`-style CR for the
   "when → how" binding, and a general `NotificationChannel` (lifting the
   `Observability`-prefixed, environment-bound one into a plane-agnostic type)?
2. **Fit with controller architecture.** Where do events *originate*? Options:
   (a) each controller emits, (b) a central watcher/informer derives events from
   CR `.status` transitions, (c) reuse native K8s `Event` objects. Where does
   the "match subscription → dispatch" logic run — a new controller, or a
   service in `openchoreo-api`/observer?
3. **Can we reuse Argo Events?** OpenChoreo already depends on **Argo Workflows**
   (`internal/dataplane/kubernetes/types/argoproj.io/...`, `cmd/main.go`), but
   **not** Argo Events. Assess `EventSource`/`Sensor`/`EventBus` vs. a native
   controller — dependency weight, whether it can watch arbitrary OpenChoreo CRs,
   and whether scenario #1 (event → trigger a build) is better served by it.
4. **Controller vs. service.** Is this a reconciler (CRD-driven, watches +
   status), a long-running service (consumes a stream, dispatches), or both?
5. **Relationship to `component-alerts`.** Explicitly map how this coexists with
   `samples/component-alerts/alert-notification-channels.yaml` and
   `alert-rule-trait.yaml` — shared channel type? shared dispatch code? Avoid a
   forked notification stack.
6. **End-to-end architecture.** Event taxonomy, filtering/selector model,
   at-least-once vs best-effort delivery, retry/dead-letter, ordering, security
   (who may subscribe to what), and how "event → in-cluster action" (#1) differs
   from "event → external notification" (#2–#6).

---

## Proposed design (summary — see `proposal-claude.md` for the full spec)

> The authoritative design lives in **`proposal-claude.md`** (currently at
> revision 4). `proposal-codex.md` is a parallel, deeper writeup. This is a
> short summary so the task and the design stay in one glance; if the two ever
> disagree, the proposal wins.

The design converged (across revisions) on a **deliberately lightweight footprint
— 2 CRDs, 1 new deployment**:

1. **One public CRD — `EventSubscription`** ("when + optional aggregate → how"),
   with the webhook/email destination written **inline** (no separate
   `EventDestination` CRD in v1). Low-level senders are **reused** from
   `internal/observer/notifications/*`; a unified generic `NotificationChannel`
   shared with observability is a **later, separate** migration (not v1).
2. **Producers, not a central informer.** Each event is emitted by the
   controller that owns the transition (e.g. the WorkflowRun controller on
   `ConditionWorkflowFailed → True`), as a **library** (no new deployment), which
   idempotently writes a **CloudEvents** record into a **durable outbox**.
   Deriving events from raw informer add/update was considered and **rejected**
   (restart re-fires `*.created`; an update isn't a domain fact; leaks internals).
3. **One internal record — `PlatformEvent`** — the durable outbox entry that
   **also carries per-sink delivery state in its `.status`** (no separate
   `EventDelivery` CRD). At-least-once with deterministic IDs (dedup), retry,
   dead-letter, replay, audit; delivery is async so a slow endpoint never blocks
   reconciliation.
4. **An aggregate evaluator** for the **stateful** family (scenario #7): counts
   events over sliding/tumbling/consecutive windows, applies cooldown and
   firing/resolved, and emits **derived events** (e.g. `…failure_threshold_exceeded`).
   Window state is **in-memory, rebuilt from retained `PlatformEvent`s** on restart
   (no `AggregateState` CRD).
5. **One new deployment** hosts the subscription reconciler + router + workers +
   evaluator.
6. **Argo Events off the public path** — it can't do the stateful counting in #7,
   exposes internal resource shapes, and would couple our API to a dependency.
   Optional internal adapter only.

> **Scale-up path.** The heavier variants (`EventDestination` for reuse,
> `EventDelivery` for high fan-out, `AggregateState` for long windows, a broker
> for high volume) are all **additive** upgrades added only when a measured signal
> demands — see `proposal-claude.md` §A. Start at 2 CRDs.

### Alternatives considered

- **Extend the observability alert path to lifecycle events** — rejected: it's
  telemetry-shaped and blind to control-plane state. (We *do* reuse its senders
  and borrow its window/threshold/firing-resolved *concepts* for scenario #7.)
- **Central informer that derives events from CR add/update/delete** — rejected
  (see point 2 above): unsafe on restart, no reliable "what transitioned" signal.
- **Emit only native K8s `Event`s and let users watch them** — rejected: lossy,
  TTL'd, no delivery, no external sinks, no filter/aggregate DSL.
- **Reuse the lossy `internal/eventforwarder` path** — rejected for user
  automation (debounced, drops under load); keep it for Backstage catalog only.
- **Argo Events as the front door** — rejected (weight/opacity, no stateful
  counting), kept as an optional internal action backend.

---

## Deliverables

1. **Design doc / ADR** answering all six open questions, with the event catalog
   (base **and** derived events, covering scenarios #1–#7) and the
   `EventSubscription` (incl. inline destinations + the `aggregate` block) and the
   internal `PlatformEvent` sketched as Go types — 2 CRDs total, with the §A
   scale-up path noted.
2. **Reuse plan** for `internal/observer/notifications/*` → shared
   `internal/notifications` package, plus the *deferred* migration story for
   `ObservabilityAlertsNotificationChannel` and `samples/component-alerts/*`.
3. **Argo Events evaluation** write-up (dependency cost, capability fit — note it
   can't do the stateful counting in #7 — recommendation) to defend the
   build-vs-reuse call.
4. **Architecture diagram**: producer → outbox → (aggregate evaluator) → router →
   delivery (webhook | email | action), including where each part runs and how it
   hooks the existing condition/status patterns.
5. **Two thin walking-skeletons**: (a) discrete — `workflowrun.failed → email`
   via a reused sender; (b) stateful — `>5 workflowrun.failed in 10m → page`,
   proving the aggregate evaluator + durable window state before committing to
   the full catalog.

## Where things live (for the implementer)

- Event/status sources: `api/v1alpha1/*_types.go` (esp. `workflowrun_types.go`
  `status.phase`; `component_types.go`, `project_types.go`), controller status
  patterns in `internal/controller/*/controller_status.go`.
- Watch/index pattern to mirror: `internal/controller/*/controller_watch.go`.
- Reusable notification code: `internal/observer/notifications/{email,webhook,sender}.go`,
  `internal/observer/service/notification_dispatch.go`.
- Existing channel/alert CRs to generalize/coexist with:
  `api/v1alpha1/observabilityalertsnotificationchannel_types.go`,
  `api/v1alpha1/observabilityalertrule_types.go`,
  `samples/component-alerts/*`.
- Argo Workflows integration (what we already depend on):
  `internal/dataplane/kubernetes/types/argoproj.io/...`, `cmd/main.go`.
- CEL templating engine (reused for channel templates): `internal/template/`.
- Authz gate for subscriptions/actions: `internal/authz/`.

---

_Propose the design, then discuss it._ Once the ADR lands and the catalog +
API are agreed, build the **discrete** path first (phases 1–2 in
`proposal-claude.md`), then the **stateful aggregation** path (phase 3), each
starting from its walking skeleton in deliverable 5.
