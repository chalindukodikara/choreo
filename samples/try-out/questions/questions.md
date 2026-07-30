# Open Questions: GitOps DX (CLI + UI)

The same PR-based flow must work from both `occ` and Backstage.

1. **GitOps mode toggle** — How (and at what scope) do users opt into "every action creates a PR"?

2. **Git providers & credentials** — How do we support all major git providers and configure the repo, org, and credentials in one place for both CLI and UI?

3. **Repo layout** — How does OC know where to write each CR, given that every team's GitOps repo is structured differently?

4. **Multiple GitOps repos** — How do we handle setups with one repo per env (or per cluster), especially when a single action spans repos?

5. **Reconciliation lag** — How do we communicate the gap between "PR merged" and "actually deployed" in the UI?
