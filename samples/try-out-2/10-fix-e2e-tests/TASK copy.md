# Task: Restore the E2E gate by fixing the recurring UI failure

## Goal

Make the E2E gate reliable and green at commit
`ad065f0fcb84e0ca8f49e32c1a4a9cb3a3df7b96` by:

1. fixing the deterministic Playwright selector failure reproduced by all three
   cited runs; and
2. investigating the single Tier 1 connection-readiness timeout without hiding a
   product regression or weakening the test.

Treat these as separate problems. The UI failure is a confirmed regression. The
Tier 1 failure is currently an isolated event and needs evidence before any code or
timeout change is justified.

## CI evidence already collected

All three runs tested the same commit.

| Run | Date (UTC) | Failed jobs | Relevant result |
|---|---:|---|---|
| [28488013896](https://github.com/openchoreo/openchoreo/actions/runs/28488013896) | 2026-07-01 | UI / Playwright | Same UI spec failed on the initial attempt and both retries |
| [28414700414](https://github.com/openchoreo/openchoreo/actions/runs/28414700414) | 2026-06-30 | UI / Playwright; Tier 1 | Same UI failure; Tier 1 had one ReleaseBinding readiness timeout |
| [28343658881](https://github.com/openchoreo/openchoreo/actions/runs/28343658881) | 2026-06-29 | UI / Playwright | Same UI spec failed on the initial attempt and both retries |

The other E2E legs passed in the latest run: Tier 1, Tier 2, Tier 3, External
IdP/Dex, and Quick Start Smoke.

### Confirmed recurring UI failure

- Spec: `test/ui/specs/config/component-config-edits.spec.ts:483`
- Test: `overrides inherited entries with read-only name fields`
- Failing action: click the `Override` action for the inherited
  `GREETING_PREFIX` environment variable (currently around line 503)
- Error: Playwright strict-mode violation; the locator resolves to two
  `Override environment variable` buttons
- Reproduction strength: 3/3 nightly runs, including both retries in every run
- Consequence: because the spec is serial, five later config tests do not run

The current locator climbs through broad ancestor `<div>` elements and uses
`.last()`. It can select a container that contains more than one matching row.
This is a test-locator defect unless artifact inspection shows the UI itself is
rendering a duplicate row or duplicate action unexpectedly.

### Isolated Tier 1 failure

- Suite: `test/e2e/suites/connections/`
- Spec: `Connection Resolution / project visibility / resolves consumer connections eventually`
- Failure: `consumer-development` remained `Ready=False` for 180 seconds
- Run: only `28414700414`; the same Tier 1 leg passed on the preceding and
  following cited runs

Do not classify this as "just a flake" solely because it passed later. Use the
failed run's diagnostics and controller events/logs to identify why `Ready` stayed
false. Also do not increase the three-minute timeout unless evidence shows the
system was making valid progress and merely exceeded an unrealistic deadline.

## Required work

### 1. Verify the failure evidence

- Read the failed logs with `gh run view <run-id> --log-failed`.
- Download or inspect the Playwright trace, screenshot, and error context from at
  least one failed run. Determine whether there is one logical row with an
  ambiguous locator or two rows/actions rendered by the product.
- Inspect the Tier 1 diagnostic artifact from run `28414700414`, including the
  `consumer-development` ReleaseBinding conditions, related events, and relevant
  controller logs.
- Record a short root-cause note for each failure. Clearly label the Tier 1 result
  as a confirmed product/test defect, an infrastructure issue, or inconclusive.

### 2. Fix the recurring Playwright failure

- Put reusable row-selection behavior in `test/ui/po/overrides.ts`; avoid adding
  another one-off XPath chain in the spec.
- Scope the locator to exactly one environment-variable row identified by
  `GREETING_PREFIX`, then select its exact accessible action name
  (`Override environment variable`). Apply the equivalent robust approach to file
  mount rows if they use the same ambiguous pattern.
- Preserve the behavioral assertions: inherited names remain read-only, edited
  values are saved, and the resulting resource state is verified.
- Do not use `.first()`, `.last()`, or `.nth()` merely to suppress Playwright
  strict mode. Positional selection is acceptable only when ordering is part of
  the product contract and is documented in the test.
- If the trace proves the product renders duplicate rows/actions, fix the product
  behavior instead and retain a strict locator that would catch a recurrence.

The portal implementation is not present under a `backstage-plugins/` directory in
this repository. If a separate Backstage checkout is available, use it only to
correlate the UI change or fix a confirmed product defect; do not make the task
depend on a machine-specific absolute path.

### 3. Handle the Tier 1 timeout based on evidence

- If diagnostics identify a deterministic controller or fixture defect, fix it and
  add or strengthen an assertion that exposes the actual failed condition/reason.
- If the failure was transient, improve diagnostics where practical so a future
  timeout reports all ReleaseBinding conditions, reasons, and relevant events
  instead of only `Ready=False`.
- Do not add unconditional sleeps, broad retries, skipped tests, or a larger
  timeout as the sole fix.
- If evidence is insufficient and the test is currently repeatably passing,
  document the investigation and leave behavior unchanged rather than guessing.

## Relevant files

- `test/ui/specs/config/component-config-edits.spec.ts`
- `test/ui/po/overrides.ts`
- `test/ui/playwright.config.ts`
- `test/e2e/suites/connections/connections_test.go`
- `test/e2e/framework/`
- `.github/workflows/e2e-gate.yml`
- `make/e2e.mk`

## Verification

Use the smallest useful test while iterating, then run the affected suite in the
same topology as CI.

```sh
# UI cluster and suite
make e2e.setup \
  E2E_WITH_BUILD=true \
  E2E_WITH_OBSERVABILITY=true \
  E2E_WITH_UI=true

cd test/ui
npm ci
npx playwright test specs/config/component-config-edits.spec.ts \
  --grep "overrides inherited entries with read-only name fields"

# Tier 1 connection suite against an existing e2e cluster
cd ../..
make e2e.test E2E_LABEL_FILTER=tier1
```

The config spec is serial and depends on state created by earlier tests. If the
grep-only command cannot establish its prerequisites, run the complete config spec
instead; do not change the spec to bypass its setup solely for local convenience.
Collect diagnostics before tearing down a failing cluster:

```sh
make e2e.status
make e2e.diagnostics
make e2e.down
```

## Definition of done

- [ ] The `GREETING_PREFIX` action locator resolves to exactly one element without
      positional disambiguation, and the inherited env-var and file-mount flows pass.
- [ ] The complete `component-config-edits.spec.ts` passes with no tests skipped
      because of an earlier serial failure.
- [ ] The Tier 1 timeout has an evidence-backed classification; any corresponding
      fix or diagnostic improvement is covered by a relevant test.
- [ ] No sleeps, blanket retries, unjustified timeout increases, or skipped tests
      were introduced.
- [ ] Relevant formatting, linting, and type checks pass for every changed file.
- [ ] The final report lists the root cause, changed files, verification commands,
      and results, plus any remaining uncertainty around the isolated Tier 1 event.

## Constraints

- Keep changes limited to the failures above and any directly shared page-object or
  diagnostic helper needed for a durable fix.
- Prefer semantic, accessible Playwright locators over CSS classes or generated
  Material UI class names.
- Preserve the product behavior covered by the tests; do not weaken assertions to
  make CI green.
- Do not push commits, rerun workflows, open pull requests, or make other external
  mutations. GitHub log and artifact access must remain read-only.
