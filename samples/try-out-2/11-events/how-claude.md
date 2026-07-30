# How the Event System Works — in simple terms

**Companion to:** `proposal-claude.md`
**Audience:** anyone who wants to understand the moving parts without reading the full spec.

---

## The one-sentence idea

> When something meaningful happens inside OpenChoreo (a build fails, a project
> is created…), the controller that noticed it **writes down a small note**
> ("event"). A separate worker **reads the notes, checks who asked to be told,**
> and **delivers** the news — as an email, a webhook, or by doing an action like
> starting a build.

Think of it like a **post office**:
- **Producers** = people dropping letters in the mailbox (controllers writing events).
- **The mailbox / sorting room** = a durable store where letters wait safely (the *outbox*).
- **The sorting clerk** = looks at each letter and the address book to decide who wants it (the *router* + your `EventSubscription`s).
- **The delivery drivers** = actually carry each letter to its destination and retry if nobody answers (the *delivery workers*).

Nobody delivers a letter by hand the instant it's written — it always goes
through the mailbox first. That's what keeps the system reliable.

---

## The cast of characters

> **Kept deliberately small.** The whole thing is **one config file you write**
> (`EventSubscription`) and **one behind-the-scenes record** (`PlatformEvent`) —
> plus one new background service. Everything else below is a *role* inside that
> service, not a separate thing to install.

| Piece | What it is | Its job |
|---|---|---|
| **Producer** | Code inside an existing controller (e.g. the WorkflowRun controller) — a **library**, not a new service | Notices a real change and **writes an event** |
| **Event (`PlatformEvent`)** | A small, saved record (a K8s object) — the **only** behind-the-scenes object | The "letter" — says *what happened* (CloudEvents format), **and** tracks its own delivery status |
| **Outbox** | Just the collection of stored `PlatformEvent`s | Keeps letters safe until delivered |
| **`EventSubscription`** | A config file **you** write (the **only** thing you author) | Says *"tell me when X happens (or happens N times), and here's the webhook/email to use"* |
| **Aggregate evaluator** | A role in the eventing service (optional, only for counting rules) | Counts events over a time window; when a rule trips ("5 in 10 min"), emits a new **derived event** |
| **Router + delivery workers** | Roles in the eventing service | Match each event to subscriptions, then **send** the webhook/email (or perform the action) and retry |

Everything runs in just **two places**: the **producers** live inside the
existing controllers, and one small **eventing service** does all the matching,
counting, and delivering. The destination (webhook URL / email settings) is
written **inline** in your subscription — there's no separate "destination"
object to manage.

---

## The life of one event, start to finish

1. **Something happens.** A build finishes and fails.
2. **The producer writes it down.** The WorkflowRun controller — which already
   knows the build failed (it sets a `WorkflowFailed` condition) — creates a
   `PlatformEvent` in the **outbox**. The event has a **fixed, unique ID** built
   from the resource + what happened, so writing it twice by accident is harmless
   (same ID = "already exists" = fine).
   - The event carries only **safe, useful facts** (project, component, reason) —
     never passwords or the whole resource.
3. **The event waits safely.** It's now stored. Even if a pod restarts, the event
   is not lost.
4. **The router reads it.** It looks at every `EventSubscription` and asks:
   *"Does anyone care about `workflowrun.failed` for this project?"*
5. **For each interested subscriber, it adds a delivery entry** onto the event
   itself (in the event's `status`). One event can carry several entries (email to
   team A, webhook to system B) — no separate "delivery" object needed.
6. **A delivery worker picks up an entry**, reads the **inline** address from the
   subscription (webhook URL / email settings, with credentials from a Secret),
   formats the message, and **sends it**.
7. **Retry if needed.** If the webhook is down, the worker waits and tries again
   (with growing delays). After too many failures that entry goes to
   **dead-letter**, where an operator can inspect and **replay** it later.
8. **Done.** The entry is marked `Succeeded` and the event is eventually cleaned up.

Key guarantee: **at-least-once.** The message may very rarely be delivered twice
(so receivers should ignore duplicates using the event ID), but it won't silently
vanish.

---

## Two kinds of "delivery"

- **Notification** → send it *outside* OpenChoreo (webhook, email, page). The
  address is written **inline** in the subscription (credentials via a Secret).
- **Action** → do something *inside* OpenChoreo (e.g. start a build). This is a
  small **allowlist** of safe operations — not "run any command." It runs with
  the subscriber's own permissions, and uses a fixed name so it can't
  accidentally start the same build twice.

## Two kinds of "when": single events vs. counting

Most rules react to **one event** ("this build failed → email me"). But some
rules only make sense when you **count across many events over time**:

> "Page me only if there are **more than 5** build failures in **10 minutes**."

A plain filter can't do that — when a single event arrives, it has no idea how
many came before it. So there's an optional extra worker, the **aggregate
evaluator**, that sits in the middle and **keeps a tally**. When the tally trips
a rule (5 in 10 min), it writes a **brand-new event** — a "derived event" like
`failure_threshold_exceeded` — which then travels through the *same* mailbox →
sorter → driver pipeline as everything else.

Think of it as a clerk with a **tally counter and a stopwatch** sitting next to
the mailbox: they don't deliver anything themselves — they just watch the letters
go by, and when "5 within 10 minutes" happens, they drop a *new* letter in the
mailbox that says "threshold exceeded," and the normal delivery takes over.

Same idea covers a whole family of "counting" rules:

| You want… | The clerk does… |
|---|---|
| >5 failures in 10 min | counts within a moving 10-minute window |
| 3 failures **in a row** | remembers the last few outcomes per component |
| "it failed 50×, tell me **once an hour**" | fires once, then ignores repeats for an hour (*cooldown*) |
| one **hourly summary** email | collects everything in the hour, then sends a digest |
| "tell me when it **recovers** too" | also drops a "recovered" letter when things go back to normal |
| "**no** successful build in 24h" | watches the clock and fires when the expected event *never* arrives |

The important part: the counting worker **remembers state**. To keep things
light, it holds the tally **in memory** and, if it restarts halfway to "5
failures," simply **re-reads the recent stored events** to rebuild the count — so
it doesn't forget the 3 it already saw, without needing an extra database.

---

## Walking through each case

For every case, the shape is the same:
**(A) what the producer writes → (B) where it's stored → (C) the subscription
that catches it → (D) who does the work → (E) what the receiver gets.**

---

### Case 1 — Component created → trigger a build (in-cluster action)

**Plain English:** "Whenever a new component appears in the `shop` project,
automatically start its build."

- **(A) Producer:** the **Component controller** notices a new component and
  writes a `component.created` event.
- **(B) Stored:** in the outbox as a `PlatformEvent`.
- **(C) Subscription:**
  ```yaml
  spec:
    eventTypes: [dev.openchoreo.component.created.v1]
    filter: event.data.project == "shop"
    actions:
      - name: build-it
        workflowRun: { useComponentWorkflow: true }   # an ACTION, not a webhook
  ```
- **(D) Who works on it:** the router matches it; a delivery worker performs the
  **action** — it creates a `WorkflowRun` (a build) using the component's
  configured workflow, acting as the subscription's owner. A fixed name prevents
  a duplicate build if the event is seen twice.
- **(E) Result:** a build starts automatically. Nothing leaves the cluster.

> This is the only case where the "delivery" is an internal action instead of an
> outside message.

---

### Case 2 — Build failed → push to an external system (webhook)

**Plain English:** "When a build in `shop/checkout` fails, POST the details to
our CI dashboard."

- **(A) Producer:** the **WorkflowRun controller** — when the `WorkflowFailed`
  condition flips to true — writes a `workflowrun.failed` event.
- **(B) Stored:** outbox.
- **(C) Subscription (the address is written inline):**
  ```yaml
  kind: EventSubscription
  spec:
    eventTypes: [dev.openchoreo.workflowrun.failed.v1]
    filter: event.data.project == "shop" && event.data.component == "checkout"
    actions:
      - name: notify-ci
        webhook:                              # inline — no separate object
          url: https://ci.example.com/hooks/openchoreo
          headers:
            Authorization: { valueFrom: { secretKeyRef: { name: ci-creds, key: token } } }
  ```
- **(D) Who works on it:** router matches → adds a delivery entry to the event →
  a delivery worker POSTs the JSON to the URL, adds the auth header from the
  Secret, signs it (HMAC) so the receiver can verify it's really us, and retries
  if the endpoint is down.
- **(E) Result:** the CI dashboard receives a JSON body with the event
  (`project`, `component`, `reason`, `message`, and the unique `id`).

---

### Case 3 — Build failed → send an email

**Plain English:** "Email the dev team when their build fails."

Almost identical to Case 2 — the only difference is the **destination type**.

- **(A) Producer:** same `workflowrun.failed` event (one event can feed **both**
  a webhook and an email — two delivery entries on the same event).
- **(C) Subscription (inline email):**
  ```yaml
  kind: EventSubscription
  spec:
    eventTypes: [dev.openchoreo.workflowrun.failed.v1]
    actions:
      - name: email-team
        email:                                # inline — no separate object
          to: [dev-team@acme.com]
          smtp: { host: mail.acme.com, port: 587, auth: { ...secretRef... } }
          template: { subject: "Build failed: ${component}", body: "..." }
  ```
- **(D) Who works on it:** the delivery worker uses the **same email sender code
  that observability alerts already use** (`internal/observer/notifications/email.go`),
  fills in the template with the event data, and sends via SMTP.
- **(E) Result:** the team gets an email. (Email can rarely duplicate — that's
  documented and accepted.)

> Cases 2 and 3 show the point of the design: **one event, many deliveries.** The
> producer doesn't know or care who's listening.

---

### Case 4 — Workload unhealthy in a specific environment → page an external system

**Plain English:** "If a running workload goes unhealthy **in production**, call
our on-call/paging system. Don't page us for dev."

- **(A) Producer:** the **deployment-status reconciler** watches the health that
  the data plane reports and writes a `deployment.degraded` event **only when
  health actually transitions** (healthy → unhealthy), not on every status blip.
- **(B) Stored:** outbox.
- **(C) Subscription — note the environment filter:**
  ```yaml
  spec:
    eventTypes: [dev.openchoreo.deployment.degraded.v1]
    filter: event.data.environment == "production"      # <-- the key part
    actions:
      - name: page-oncall
        webhook: { url: https://events.pagerduty.com/..., headers: { ...secretRef... } }
  ```
- **(D) Who works on it:** router matches, but the `filter` **drops** the same
  event for dev/staging — only production degradations get a delivery entry. A
  worker calls the paging webhook.
- **(E) Result:** on-call gets paged for prod only. This is the **"filter by
  attribute"** capability in action.

---

### Case 5 — Project created → emit an event (broad subscription)

**Plain English:** "Tell our governance system about **every** new project."

- **(A) Producer:** the **Project controller** writes a `project.created` event
  when a project first appears.
- **(B) Stored:** outbox.
- **(C) Subscription — no filter, catches all:**
  ```yaml
  spec:
    eventTypes: [dev.openchoreo.project.created.v1]
    actions:
      - name: notify-governance
        webhook: { url: https://governance.acme.com/hook, headers: { ...secretRef... } }
  ```
- **(D) Who works on it:** router matches every project-created event; a worker
  sends each to the governance webhook.
- **(E) Result:** governance is notified for **all** projects.

> **Restart safety:** when the feature is first switched on, it does **not** fire
> "created" for the hundreds of projects that already existed — a stored
> *activation timestamp* means only projects created **after** turn-on count.
> Same reason a controller restart won't spam you.

---

### Case 6 — Project created *with a specific name* → emit an event (CEL filter)

**Plain English:** "Only tell me when a project named `payments` is created."

Same event as Case 5 — the difference is a **filter** so you get one, not all.

- **(A) Producer:** same `project.created` event.
- **(C) Subscription — filtered by name:**
  ```yaml
  spec:
    eventTypes: [dev.openchoreo.project.created.v1]
    filter: event.data.project == "payments"      # CEL expression
    actions:
      - name: notify
        webhook: { url: https://governance.acme.com/hook, headers: { ...secretRef... } }
  ```
- **(D) Who works on it:** the router evaluates the CEL `filter` against the
  event's data; if it isn't the `payments` project, **no delivery entry is
  added** and nothing is sent.
- **(E) Result:** you're notified only for `payments`.

> Cases 5 and 6 are the **same event type** — "broad" vs "specific" is just
> whether you add a `filter`. CEL is the same expression language OpenChoreo
> already uses elsewhere. The filter only sees the event's own data — it can't go
> read other resources or call the network.

---

### Case 7 — More than 5 build failures in 10 minutes → page on-call (counting)

**Plain English:** "Don't bother me about a single flaky build. But if a
component fails **more than 5 times in 10 minutes**, page on-call — and only page
me once."

This is the first case that needs **memory across events**, so it uses the
`aggregate` block and the counting clerk from above.

- **(A) Producer:** the **WorkflowRun controller** writes the *same*
  `workflowrun.failed` event as cases 2/3 — it does **not** know or care about
  counting. It just reports each individual failure.
- **(B) Stored:** each failure lands in the outbox as usual.
- **(C) Subscription — note the `aggregate` block:**
  ```yaml
  spec:
    eventTypes: [dev.openchoreo.workflowrun.failed.v1]
    aggregate:
      groupBy: [component, environment]   # count each component+env separately
      window: { type: sliding, size: 10m }
      condition: "count >= 5"             # trip at the 5th failure in the window
      cooldown: 1h                        # then stay quiet for an hour
      emitResolved: true                  # also tell me when it recovers
    actions:
      - name: page-oncall
        webhook: { url: https://events.pagerduty.com/..., headers: { ...secretRef... } }
  ```
- **(D) Who works on it:** the **aggregate evaluator** reads each
  `workflowrun.failed` event, bumps the counter for that component+environment,
  and does nothing while the count is under 5. On the 5th failure inside the
  window it **emits a new derived event**,
  `workflowrun.failure_threshold_exceeded`, into the outbox. *That* event is what
  the on-call subscription actually delivers — so a normal delivery worker pages
  PagerDuty. `cooldown` stops it paging again for an hour; if failures stop,
  `emitResolved` sends a "recovered" page.
- **(E) Result:** one page when a component genuinely goes bad — not 50 pages for
  50 failures, and nothing for a single flaky build. If the evaluator pod restarts
  after seeing 3 failures, it rebuilds the count from the recent stored events, so
  it still knows it's at 3.

> The key trick: producers stay dumb (one letter per failure), and a separate
> counting worker turns "many small events" into "one meaningful event."

---

## Quick reference: where does each thing live?

| Question | Answer |
|---|---|
| **Who notices the change?** | The controller that owns that resource (Component, WorkflowRun, Project, deployment-status) |
| **How is the event "sent"?** | It isn't sent directly — the producer **writes it to the outbox** (a K8s record) |
| **Where is it stored?** | In the outbox as a `PlatformEvent` — the same object also tracks each send in its `status` (no separate delivery object) |
| **What decides who gets it?** | The **router**, by matching your `EventSubscription`s (type + optional CEL filter) |
| **Where's the destination (URL/email)?** | Written **inline** in your `EventSubscription` (credentials come from a Secret) — nothing else to create |
| **What about "count 5 in 10 min" rules?** | The **aggregate evaluator** keeps a running tally and emits a new "threshold exceeded" event when it trips |
| **Where is the counter kept?** | In memory, rebuilt from recent stored events on restart — no extra database |
| **Who actually delivers it?** | **Delivery workers** in the eventing service (webhook, email, or in-cluster action) |
| **What does the receiver get?** | A standard CloudEvents JSON with safe fields (project, component, reason…) and a unique `id` |
| **What if delivery fails?** | Automatic retries with growing delays → dead-letter → operator can replay |
| **Can it be lost?** | No — it's stored before any delivery attempt (**at-least-once**) |
| **Can it arrive twice?** | Rarely, yes — receivers should ignore duplicates using the event `id` |
| **Does a slow email block OpenChoreo?** | No — producing and delivering are separate; delivery is asynchronous |

---

## The three rules that make it trustworthy

1. **Store first, deliver later.** Nothing is delivered straight from a
   controller. The event is saved, then handed off. A crash can't lose it.
2. **Same thing, same ID.** Events and deliveries have deterministic IDs, so
   accidental repeats collapse into one — no duplicate builds, no double-counting.
3. **Producers don't know receivers.** A controller just says "this happened."
   Who cares about it, and how they're told, is entirely decided by
   `EventSubscription`s — so adding a new listener never means touching controller code.
