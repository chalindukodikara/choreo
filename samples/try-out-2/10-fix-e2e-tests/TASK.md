# Task: Fix failing e2e tests

## Objective

Investigate and fix the e2e test failures that are currently occurring in CI, so that
the e2e workflow runs green again.

## Failing CI runs (most recent first)

- https://github.com/openchoreo/openchoreo/actions/runs/28488013896
- https://github.com/openchoreo/openchoreo/actions/runs/28414700414
- https://github.com/openchoreo/openchoreo/actions/runs/28343658881

Start by pulling the logs from these runs (e.g. `gh run view <run-id> --log-failed`)
to identify exactly which suites/specs fail and the error messages. Note whether the
same tests fail across all three runs (a real regression) or whether they differ
(flaky tests / environment issues) — treat these differently.

## Suspected causes

- **UI test failures** are likely caused by recent changes to the Backstage portal
  (locators, page structure, or component behavior changing under the page objects).
  Check whether the failing UI specs correspond to recently changed UI code.

## Where things live

- **Go e2e tests:** `test/e2e/`
  - `suites/` — one package per feature area (authz, build, gateway, gitops, mcp,
    observability, occ, secrets, …), labeled `tier1`/`tier2`/`tier3`
  - `framework/` — shared helpers (kubectl/occ wrappers, waits, gateway, MCP client)
  - `k3d/` — cluster bring-up config
- **UI (Playwright) tests:** `test/ui/`
  - `specs/` — test suites (auth, catalog, lifecycle, dev-ops, pe-ops, abac-ui, config)
  - `po/` — page objects (locators + intent-named methods) — the most likely place a
    UI change breaks a selector
  - `fixtures/` — per-role auth state + kubectl wrappers
- **Backstage portal source (the UI under test):**
  `/Users/chalindu/Documents/WSO2/Repos/openchoreo all/backstage-plugins/`
  - `packages/`, `plugins/` — the app + plugin code whose changes may have broken the
    UI specs

## How to run locally

```sh
# Go e2e
make e2e                                   # full lifecycle: setup → test → teardown
make e2e.setup                             # create openchoreo-e2e k3d cluster + install planes
make e2e.test                              # run suites against existing cluster
make e2e.test E2E_LABEL_FILTER='tier1'     # scope to a tier

# UI e2e
make e2e.setup E2E_WITH_UI=true            # enable Backstage on the e2e cluster
cd test/ui && npm test                     # run Playwright specs
```

Useful knobs: `E2E_WITH_BUILD=true`, `E2E_WITH_OBSERVABILITY=true` (required for tier 3),
`E2E_KEEP_RESOURCES=true` (skip cleanup for debugging). `make e2e.status` and
`make e2e.diagnostics` help inspect a broken cluster.

## Deliverables

1. Root cause for each failing test (regression vs. flake), with a short note on what
   changed.
2. Fixes applied to the test code and/or the underlying source as appropriate — prefer
   fixing the real defect over papering over it (only mark a test skipped/flaky with
   justification if it is genuinely non-deterministic and out of scope).
3. Confirmation that the relevant e2e suites pass locally after the fix, with the
   commands used and their output.

## Constraints

- Do not push, open PRs, or make any external/network-mutating calls — local changes and
  read-only `gh` commands only.
