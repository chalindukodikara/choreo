# Add Documentation for Build Layer Caching with a Zot-Based OCI Cache Registry

## Problem

OpenChoreo's CI build workflows (Dockerfile, Paketo, GCP Buildpacks, Ballerina) currently download all base images, dependencies, and buildpack layers from scratch on every build. There is no mechanism to cache and reuse intermediate build layers across runs.

This causes:

- **Redundant network traffic** -- every build re-pulls the same base layers, language runtimes, and dependency trees even when nothing has changed.
- **Slow build times** -- repeat builds take as long as the first build because no cached layers are available.
- **Wasted compute** -- build containers spend cycles re-downloading and re-extracting artifacts that already exist from a previous run.

## Proposal

Add documentation that guides users through deploying a **Zot-based OCI cache registry** in the workflow plane namespace and configuring OpenChoreo's build templates to use it for registry-backed layer caching.

The documentation should cover:

1. **Installing the Zot cache registry** via a standalone Helm chart into `openchoreo-workflow-plane`.
2. **Updating ClusterWorkflowTemplates** (`containerfile-build`, `paketo-buildpacks-build`, `gcp-buildpacks-build`, `ballerina-buildpack-build`) to add `build-cache` and `no-cache` input parameters with cache-aware build commands.
3. **Updating ClusterWorkflows** (`dockerfile-builder`, `paketo-buildpacks-builder`, `gcp-buildpacks-builder`, `ballerina-buildpack-builder`) to expose a `noCache` schema property so developers can opt in or out of caching per build.
4. **Verification steps** to confirm caching is working (second build should be faster).
5. **Disabling / uninstalling** the cache registry without breaking existing builds.

### Key design points

- **Opt-in by default** -- `noCache` defaults to `true` so clusters without the cache registry see no change in behavior.
- **Graceful fallback** -- build templates probe the cache registry at startup; if unreachable, builds proceed normally without errors.
- **Per-component cache isolation** -- cache refs include the component identity (`<namespace>-<project>-<component>`) to prevent collisions.
- **Automatic cleanup** -- Zot's built-in retention policies and online GC handle stale cache eviction (no CronJob required).
- **Podman + Pack CLI support** -- Dockerfile builds use `podman --cache-from/--cache-to`; buildpack builds use `pack --cache-image/--clear-cache`.

## Expected outcome

A documentation page with copy-pasteable inline commands that lets a user go from zero to fully cached builds in under 10 minutes.
