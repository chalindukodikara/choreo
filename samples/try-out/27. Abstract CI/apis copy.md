| `GET /api/v1/healthz` | Yes | Liveness/readiness for routing and diagnostics. |
| `GET /api/v1/capabilities` | Yes | Declares supported features, external link, k8s-native, and source related apis. |
| `GET /api/v1/workflowruns/{runName}/status` | Yes | Whether the workflow logs can be fetched through the adapter (workflow run cr status has whether it is running or not so the the openchoreo api can understand from that. |
| `GET /api/v1/workflowruns/{runName}/logs/streams` | Yes | Discovers available log streams before reading logs. Needed for engines with jobs, matrix builds, stages, attempts, or multiple containers. |
| `GET /api/v1/workflowruns/{runName}/logs` | Yes | Returns normalized log entries. Replaces Argo pod lookup, container filtering, and timestamp parsing in OpenChoreo API. |
| `GET /api/v1/workflowruns/{runName}/events` | Yes | Returns normalized timeline events. |
| `GET /api/v1/workflowruns/{runName}/external-link` | Yes | Returns canonical upstream URL and display identity. Required for GitHub Actions, Jenkins, Azure Pipelines, and useful for Argo/Tekton dashboards. |

| `POST /api/v1/workflowruns/{runName}/cancel` | Cancels an active run if supported by the engine. |
| `DELETE /api/v1/workflowruns/{runName}` | Deletes a run through the engine. |

| `GET /api/v1/source/{url}/branches?secretName=` | Lists repository branches using credentials passed. |
| `GET /api/v1/source/{url}/branches/{branch}/commits?secretName=` | Lists commits for a repository/branch passed. |
