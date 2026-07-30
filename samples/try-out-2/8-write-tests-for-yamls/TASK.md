# Task: Tests for the workflow-template YAMLs

## Goal

Write automated tests for the build/CI workflow templates in
`samples/getting-started/workflow-templates/*.yaml` so that if anyone edits
these YAMLs in a way that breaks a supported feature (private-repo auth, a
specific git provider, checkout, build, or image push), a test fails and tells
them exactly what broke.

The tests must be resilient: they should assert on the **behaviors/contracts**
the scripts implement, not on incidental formatting, so harmless edits don't
cause false failures while real regressions are caught.

---

## What we are actually testing (evaluation findings)

These are not standalone scripts. Each template is an Argo
`ClusterWorkflowTemplate` whose logic lives as an **embedded shell script**
inside `spec.templates[].container.args[0]` (a multi-line YAML block scalar).
Two consequences shape the whole approach:

1. The script body is interleaved with Argo placeholders like
   `{{workflow.parameters.git-repo}}` and `{{inputs.parameters.git-revision}}`.
   The raw string is **not valid shell until those placeholders are
   substituted**. Any behavioral test has to substitute them first.
2. So a test harness needs to: load the YAML, locate the right template by
   name, pull out `container.args[0]`, then either (a) assert on its contents
   (static) or (b) substitute placeholders + stub external binaries and run it
   (behavioral).

### The files and what each one owns

| File | Template name | Feature surface to protect |
|------|---------------|----------------------------|
| `checkout-source.yaml` | `checkout` | **All git auth + provider logic** (the bulk of the scenarios below) |
| `containerfile-build.yaml` | `build-image` | Dockerfile build via podman, build-env/build-args, path validation |
| `ballerina-buildpack-build.yaml` | build | Ballerina buildpack build |
| `gcp-buildpacks-build.yaml` | build | GCP buildpacks build |
| `paketo-buildpacks-build.yaml` | build | Paketo buildpacks build |
| `publish-image.yaml` | `publish-image` | podman load/tag/push to `ttl.sh`, `--tls-verify=true`, optional auth |
| `publish-image-k3d.yaml` | `publish-image` | k3d variant: local registry, `--tls-verify=false` |
| `generate-workload.yaml` / `generate-workload-k3d.yaml` | — | Workload descriptor generation |

### Concrete behaviors in `checkout-source.yaml` (the contract to lock down)

- **Auth-type detection** keyed off secret file presence:
  - `ssh-privatekey` present → SSH auth path.
  - else `password` present → basic auth path (username defaults to `git`).
  - secret dir present but neither → hard error + exit 1.
  - secret dir empty/absent → public-repo path (no auth).
- **SSH path**:
  - Writes `~/.ssh/id_rsa` (CRLF stripped, trailing newline ensured, `chmod 600`).
  - Validates key looks like `BEGIN.*PRIVATE KEY`; errors if not.
  - Writes `~/.ssh/config` with explicit `Host` blocks +
    `StrictHostKeyChecking no` for: **github.com, gitlab.com, bitbucket.org,
    git-codecommit.\*.amazonaws.com**.
  - **AWS CodeCommit**: if repo is `ssh://...git-codecommit...` and an
    `ssh-key-id` file exists and the URL has no `@`, inject the key id:
    `ssh://<KEYID>@...`.
  - **https→ssh rewrite** (github/gitlab/bitbucket): a non-codecommit
    `https://host/path.git` is rewritten to `git@host:path.git` via sed.
- **Basic-auth path**: percent-encodes username & password and embeds them in
  the `https://user:pass@host/...` URL; warns if REPO isn't https; sets
  `credential.helper store`.
- **Checkout logic**: if `COMMIT` set → `clone --no-checkout --depth 1` +
  fetch/checkout commit; else → `clone --single-branch --branch $BRANCH`.
  Writes 8-char revision to `/tmp/git-revision.txt`.
- **Validation guards**: empty `REPO` → exit 1; neither `COMMIT` nor `BRANCH`
  → exit 1.

### ⚠️ Finding to settle before writing tests — "new provider still works"

Task item #4 ("if they add a new provider it still should work") is **only
half true today**, and the test should pin down which half:
- The **https→ssh sed rewrite is generic** — it works for any host, so a new
  provider over https-rewritten-to-ssh is covered.
- The **`~/.ssh/config` Host list is hard-coded** to 4 providers. A new
  provider gets **no `StrictHostKeyChecking no` entry**, so a fresh SSH host
  would prompt/fail on host-key verification.

Decision needed: do we (a) test that the 4 known providers are present (lock
current behavior), or (b) treat "new provider works out of the box" as a
desired behavior and write a **failing/red test** that documents the gap?
Recommend (a) now + an `// TODO`/skipped test for (b).

### Build templates — `containerfile-build`, `ballerina/gcp/paketo-buildpacks-build`

These four scripts share a strong common contract, which is exactly what makes
them test-worthy (a change to one should usually change all, or be deliberate):

- **App/Dockerfile path validation**: containerfile checks `$DOCKERFILE_PATH`
  *and* `$DOCKER_CONTEXT` exist; buildpacks check `$APP_PATH` is a dir. Each
  errors + `exit 1` + `ls -la` on failure. Assert the guard exists per file.
- **`build-env` / `build-args` plumbing**: all parse a JSON array via
  `jq -r '.[] | "--env \(.name)=\(.value)"'` and skip when empty or `[]`.
  containerfile additionally handles `--build-arg` from `build-args`. Assert
  both the empty-guard and the jq shape (regressions here silently drop env).
- **Output contract**: every build ends by writing the image to
  `/mnt/vol/app-image.tar` (`podman save` / after `pack build`). This is the
  hand-off to `publish-image`; assert the path/literal in every build file.
- **Buildpack specifics worth pinning**:
  - Builder/run images are **pinned by `@sha256:` digest** (supply-chain
    contract). A test can assert each buildpack references a digest, not a
    floating tag — and optionally snapshot the exact digests so bumps are
    reviewed deliberately.
  - Buildpacks start a rootless podman service and **wait** on it
    (`until podman info ... grep -q true`) and on image existence
    (`until podman image exists`). containerfile does *not* (uses podman
    directly). Assert the buildpack readiness-wait blocks are present.
  - `pack build` uses `--docker-host inherit`, `--pull-policy always`,
    `--builder`, `--run-image`, `--path "$WORKDIR/$APP_PATH"`. Assert these
    flags so a refactor can't silently drop pinning or pull policy.
- **Param surface drift**: `gcp` and `paketo` read `build-env` from
  `workflow.parameters`, while `ballerina` and `containerfile` read it from
  `inputs.parameters`. This inconsistency is a real footgun — a test that
  records the expected source per template makes intentional vs accidental
  changes visible.

### `publish-image` (+ k3d variant)

- Both files declare `metadata.name: publish-image`. Confirm only one is
  installed per profile; assert cloud variant = `--tls-verify=true` +
  `ttl.sh/openchoreo-builds`; k3d variant = `--tls-verify=false` +
  `host.k3d.internal:10082`.
- **Auth-optional push**: cover both `if [ -f "$AUTH_FILE" ]` (uses
  `--authfile`) and the anonymous else branch.
- **Input contract guard**: errors + `exit 1` when `/mnt/vol/app-image.tar`
  is missing (the build→publish handoff). Assert it.
- **Tag/output contract**: `podman load` → `podman tag` to
  `$REGISTRY_ENDPOINT/$SRC_IMAGE` → write final ref to `/tmp/image.txt`
  (consumed downstream). Assert the retag prefix and the output file.

### `generate-workload` (+ k3d variant) — by far the richest, mostly untested

This script is a full publish orchestration with many branches:

- **Descriptor detection**: `WORKLOAD_FROM_SOURCE` = true iff
  `<app-path>/workload.yaml` exists; drives `occ workload create` with vs
  without `--descriptor`. Cover both.
- **App-path resolution**: `DESCRIPTOR_PATH=/mnt/vol/source${APP_PATH:+/${APP_PATH#/}}`
  (strips leading slash, tolerates empty path). Missing dir → `exit 1`. This
  string munging is exactly the kind of thing behavioral tests should pin.
- **YAML→JSON pipeline**: `yq -o=json 'del(.apiVersion)|del(.kind)'` then
  `jq -c`. Assert the strip of apiVersion/kind (the API rejects them otherwise).
- **OAuth**: POST client-credentials, optional `scope` branch, parse
  `access_token`, `exit 1` if empty. Cover scope/no-scope and empty-token.
- **Create/conflict state machine** (highest-value coverage):
  - 2xx → created.
  - **409 + source-defined** → PUT full replace.
  - **409 + auto-generated** → GET existing, `jq` merge *only*
    `.spec.container.image`, PUT merged. (Protects the "don't clobber
    user/source fields on rebuild" behavior.)
  - other codes → `exit 1`.
- **WorkflowRun annotation**: GET run, set
  `openchoreo.dev/workload` (rtrimstr newline) + `workload-from-source`
  true/false, PUT. Assert both annotation keys and the from-source value.
- **k3d vs cloud variant differences** (from the diff — all routing/TLS, not
  logic): k3d adds `oauth-host-header`/`api-server-host-header` params, sends
  `-H "Host: ..."` on every curl, uses `http://` oauth + `curl -s` (TLS-valid
  ingress); cloud uses `https://` + `curl -sk` (self-signed). A variant-aware
  test should assert these per file rather than asserting one global shape.

### Cross-cutting invariants (cheap, high-signal, apply to every template)

- Every script begins with `set -e`.
- Every template is a valid `ClusterWorkflowTemplate` with the expected
  `metadata.name` and at least one `spec.templates[].container.args[0]`.
- The volume/secret wiring matches the script: `git-secret` mounted at
  `/etc/secrets/git-secret` (checkout), `registry-push-secret` at
  `/etc/secrets/registry-push-secret` (publish), `workspace`→`/mnt/vol`
  everywhere, `tools`→`/tools` (generate-workload). A script that reads a path
  not backed by a mount is a latent bug a structural test can catch.
- Secret volumes are `optional: true` (public-repo / anonymous-push paths must
  not require the secret to exist).

---

## Scenarios to cover (refined + expanded across all files)

### Checkout / auth (`checkout-source`)
1. Private repo over **HTTPS / basic-auth** for github, gitlab, aws(codecommit), bitbucket.
2. Private repo over **SSH** for github, gitlab, aws(codecommit), bitbucket.
3. **Checkout** by branch and by commit both work and emit `git-revision.txt`.
4. **New provider**: https→ssh rewrite is host-agnostic (covered); ssh-config
   host list gap is documented (see finding above).
5. **Auth detection** edge cases: ssh vs basic precedence, empty-secret error,
   public-repo (no secret) path.
6. **Validation guards**: missing REPO, missing both branch & commit.

### Build (`containerfile`, `ballerina`, `gcp`, `paketo`)
7. Each build template validates its app/Dockerfile path and errors on missing.
8. `build-env`/`build-args` JSON → `--env`/`--build-arg` flags, with empty/`[]`
   skipped.
9. Every build writes the image to `/mnt/vol/app-image.tar` (handoff contract).
10. Buildpacks pin builder/run images by `@sha256:` digest and wait for the
    podman socket + image existence; containerfile uses podman directly.
11. `build-env` param source is consistent-where-intended (inputs vs workflow).

### Publish (`publish-image`, `publish-image-k3d`)
12. **Image push** works with and without a registry push token/secret.
13. Cloud vs k3d variant: correct registry endpoint + `--tls-verify` value.
14. Errors when `/mnt/vol/app-image.tar` is missing; writes final ref to
    `/tmp/image.txt`.

### Generate workload (`generate-workload`, `generate-workload-k3d`)
15. Source-defined vs auto-generated workload (`workload.yaml` present or not).
16. App-path resolution + missing-dir error.
17. OAuth token: scope vs no-scope, empty-token failure.
18. Create vs 409-conflict handling: PUT full-replace (source) vs image-only
    merge (auto-generated); non-2xx failure.
19. WorkflowRun annotation keys + `workload-from-source` value.
20. k3d vs cloud: Host headers + http/https + `-s` vs `-sk`.

### Cross-cutting
21. Every template parses, has expected `metadata.name`, `set -e`, and script.
22. Volume/secret mount wiring matches the paths the scripts read; secret
    volumes are `optional: true`.

---

## Approach — two layers

**Layer 1 (primary): static / structural assertions.** Parse the YAML, extract
the script for a named template, and assert the required constructs are
present. Robust against reformatting; catches feature deletion. Example
assertions: ssh-config contains all 4 Host entries; codecommit key-id injection
block present; https→ssh sed present; basic-auth percent-encoding present;
both checkout branches present; publish covers auth/no-auth branches.

**Layer 2 (optional but high value): behavioral execution.** For
`checkout-source` only, substitute the `{{...}}` placeholders, put **stub**
`git`, `ssh-keygen`, `ssh` on `PATH`, point `$HOME` and the secret dir at temp
dirs, run the script with `sh`, and assert on the **transformed REPO URL** and
control flow (e.g. github https → `git@github.com:org/repo.git`; codecommit
ssh gets key-id; basic-auth URL is percent-encoded; empty REPO exits 1). This
is what actually proves the regex/encoding logic, which static checks can't.

Recommendation: implement Layer 1 for all templates first; add Layer 2 for the
checkout auth/provider matrix (highest-risk, hardest-to-eyeball logic).

---

## Where the tests go + tier classification

- **Tier: unit (`make test`).** These tests read static sample files and (for
  Layer 2) shell out locally. They need **no Kubernetes API server**, so they
  are *not* integration (envtest) and *not* e2e. They run in the standard unit
  pass. (Reference: CLAUDE.md "Test Tiers".)
- **Language/framework:** Go + `testify` to match repo conventions and run
  inside `make test`/CI with zero new tooling. Use `sigs.k8s.io/yaml` to parse
  the templates into a minimal struct (or `map[string]any`) and pull
  `spec.templates[].container.args[0]`.
- **Layer 2 caveat:** behavioral tests need `sh` (+ stubbed `git`/`ssh-keygen`)
  on the runner. Guard them so they `t.Skip()` when `sh` is unavailable, or
  gate behind a build tag, to keep `make test` green on all platforms.
- **Suggested location:** `test/workflowtemplates/` (new package), mirroring
  the existing `test/e2e`, `test/ui`, `test/utils` layout. Add a small helper
  to load a template + extract a named script so each test stays declarative.

---

## Deliverables / definition of done

- [ ] Resolve the "new provider" decision (finding above).
- [ ] Layer 1 static tests for every template listed in the table (checkout,
      4 builds, 2 publish, 2 generate-workload), covering scenarios 1–22.
- [ ] Layer 2 behavioral tests for the `checkout-source` auth/provider matrix
      and the `generate-workload` 409-conflict state machine (the two pieces of
      logic that static assertions can't really prove).
- [ ] Helper for loading a template and extracting `container.args[0]` by
      template name (so tests don't duplicate YAML plumbing).
- [ ] Tests run under `make test` and pass; behavioral tests skip cleanly when
      `sh`/stubs aren't available.
- [ ] A short README in `test/workflowtemplates/` explaining what contract each
      assertion protects, so future YAML editors know why a test failed.

## Open questions

- Should provider coverage be data-driven (a table of provider → input URL →
  expected transformed URL) so adding a provider is a one-line test addition?
- Do we want the k3d vs cloud `publish-image` variants both tested, or only the
  one shipped in the default profile?
- Is `ttl.sh` / registry endpoint considered part of the contract, or an env
  detail that may legitimately change (and so should not be asserted strictly)?
