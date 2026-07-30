1. `GET /api/v1/healthz` - Liveness/readiness for routing and diagnostics.
2. `GET /api/v1/capabilities` - Declares supported features, external link, k8s-native, and source related apis.
3. `GET /api/v1/namespaces/{namespace}/workflowruns/{runName}/status` - Whether the workflow logs can be fetched through the adapter (regardless of whether they are live or archived). The workflow run CR status already tells the OpenChoreo API whether the run is running or not.
4. `GET /api/v1/namespaces/{namespace}/workflowruns/{runName}/logs` - Returns normalized log entries. Replaces Argo pod lookup, container filtering, and timestamp parsing in OpenChoreo API.
5. `GET /api/v1/namespaces/{namespace}/workflowruns/{runName}/events` - Returns normalized timeline events.
6. `GET /api/v1/namespaces/{namespace}/workflowruns/{runName}/external-link` - Returns upstream URL and display identity. Required for GitHub Actions, Jenkins, Azure Pipelines, and useful for Argo/Tekton dashboards.
7. `POST /api/v1/namespaces/{namespace}/workflowruns/{runName}/cancel` - Cancels an active run if supported by the engine.
8. `GET /api/v1/namespaces/{namespace}/source/branches?repository=&secretName=` - Lists repository branches using credentials passed. `repository` (required) and `secretName` are query params.
9. `GET /api/v1/namespaces/{namespace}/source/branches/{branch}/commits?repository=&secretName=` - Lists commits for a repository/branch passed. `repository` (required) and `secretName` are query params.
