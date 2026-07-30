# External Observability Modules — Implementation Plan

## Overview

Create community modules that let operators ship **logs and traces** to popular observability backends. Each module follows the same structure as the existing modules in `community-modules/`. Metrics are already handled by Prometheus and do not need new modules.

### Three Pillars of Observability

| Pillar | Tool | New modules needed? |
|--------|------|---------------------|
| **Logs** | Fluent Bit (DaemonSet) ships container logs to backend | Yes — one `observability-logs-<vendor>` per vendor |
| **Tracing** | OpenTelemetry Collector ships distributed traces to backend | Yes — one `observability-tracing-<vendor>` per vendor |
| **Metrics** | Prometheus scrapes pods directly | No — already universal |

### Existing Modules (Reference)

```
community-modules/
├── observability-logs-opensearch         # Logs → OpenSearch (via Fluent Bit)
├── observability-logs-openobserve        # Logs → OpenObserve (via Fluent Bit + adapter)
├── observability-tracing-opensearch      # Traces → OpenSearch (via OTel Collector)
├── observability-tracing-openobserve     # Traces → OpenObserve (via OTel Collector)
```

---

## Quick Reference: Cost & Hosting

### Free self-hosted (no subscriptions, run entirely in-cluster)
- **ELK** — fully free, no license needed

### Free SaaS tiers
- **New Relic** — free tier with 100GB/month log ingest
- **Datadog** — 14-day free trial only (not a permanent free tier)

### Paid (require cloud accounts or subscriptions)
- AWS CloudWatch — pay-per-use, requires AWS account
- Azure Monitor — pay-per-use, requires Azure account
- Google Cloud Observability — pay-per-use, requires GCP account
- Splunk Cloud — paid SaaS (self-hosted Splunk Enterprise has a free dev license)

### SaaS vs Self-hosted

| Type | Vendors |
|------|---------|
| **SaaS-only** (logs ship to vendor cloud) | Datadog, New Relic, AWS CloudWatch, Azure Monitor, Google Cloud Observability |
| **Self-hosted** (or hybrid) | ELK (free in-cluster), Splunk (Enterprise self-hosted / Cloud SaaS) |

---

## Implementation Order

| Priority | Module | Logs | Tracing | Reason |
|----------|--------|------|---------|--------|
| 1 | ELK | `observability-logs-elk` | `observability-tracing-elk` | Free, self-hosted, near-identical to existing OpenSearch modules |
| 2 | Datadog | `observability-logs-datadog` | `observability-tracing-datadog` | Wide adoption, mature Fluent Bit + OTel plugins |
| 3 | New Relic | `observability-logs-newrelic` | `observability-tracing-newrelic` | Free tier (100GB/mo), straightforward APIs |
| 4 | Splunk | `observability-logs-splunk` | `observability-tracing-splunk` | SPL query language needs more adapter work |
| 5 | AWS CloudWatch | `observability-logs-cloudwatch` | `observability-tracing-xray` | Requires AWS account, native plugins exist |
| 6 | Azure Monitor | `observability-logs-azure-monitor` | `observability-tracing-azure-monitor` | Requires Azure account, native plugins exist |
| 7 | Google Cloud | `observability-logs-gcloud` | `observability-tracing-gcloud` | Requires GCP account, Stackdriver plugin exists |

---

## Module Anatomy (Reference)

### Logs Module Structure

```
observability-logs-<vendor>/
├── module.yaml                 # CI manifest: Docker images to build
├── README.md
├── init/                       # Setup container (index templates, retention, dashboards)
│   ├── Dockerfile
│   └── setup.sh (or Go binary)
├── internal/                   # Adapter service (only if vendor query API differs from OpenSearch)
│   ├── main.go
│   ├── handlers.go
│   └── ...
├── helm/
│   ├── Chart.yaml              # Dependencies: fluent-bit + vendor backend chart (if self-hosted)
│   ├── values.yaml
│   └── templates/
│       ├── fluent-bit/
│       │   └── config.yaml     # ConfigMap: INPUT → FILTER → OUTPUT (vendor-specific)
│       ├── <vendor>/           # Backend-specific resources (if self-hosted)
│       └── <vendor>-setup/     # Setup jobs
```

### Tracing Module Structure

```
observability-tracing-<vendor>/
├── module.yaml
├── README.md
├── init/                       # Setup container (trace index templates, etc.)
├── internal/                   # Adapter service (if vendor trace query API differs)
├── helm/
│   ├── Chart.yaml              # Dependencies: opentelemetry-collector + vendor backend (if self-hosted)
│   ├── values.yaml
│   └── templates/
│       ├── otel-collector/
│       │   └── config.yaml     # OTel Collector config: receivers → processors → exporters
│       ├── <vendor>/
│       └── <vendor>-setup/
```

### What stays the same across all logs modules
- Fluent Bit INPUT: `tail /var/log/containers/*.log`
- Fluent Bit FILTER: `kubernetes` metadata enrichment
- Helm chart structure and `module.yaml` format

### What stays the same across all tracing modules
- OTel Collector receiver: OTLP (gRPC + HTTP)
- OTel Collector processors: batch, resource enrichment
- Helm chart structure and `module.yaml` format

### What changes per module
- **Logs**: Fluent Bit OUTPUT plugin and its configuration
- **Tracing**: OTel Collector exporter and its configuration
- Backend Helm chart dependency (or none for SaaS-only vendors)
- Adapter service (translates Observer API queries to vendor-specific query API)
- Init/setup logic (index templates, dashboards, retention policies)

---

## Module 1: ELK (Elasticsearch + Kibana)

### Why first
- OpenSearch is an ES fork — Fluent Bit config, query API, and index structure are nearly identical
- Self-hosted, no subscriptions required
- Can copy existing OpenSearch modules and adapt minimally

### Logs Module Steps

1. **Scaffold the module**
   - Copy `observability-logs-opensearch/` as `observability-logs-elk/`
   - Update `module.yaml` with new image names

2. **Helm chart**
   - Replace `opensearch` dependency with `elasticsearch` (from `https://helm.elastic.co`)
   - Optionally add `kibana` chart as a dependency
   - Update `values.yaml`: rename opensearch references to elasticsearch

3. **Fluent Bit config**
   - Change OUTPUT from `Name opensearch` to `Name es`
   - Adjust TLS and auth settings for Elasticsearch defaults
   - Keep INPUT and FILTER identical

4. **Init container**
   - Adapt `setup-opensearch.sh` → `setup-elasticsearch.sh`
   - Create index templates, ILM policies (ES uses ILM instead of ISM)
   - Set up retention policies

5. **Adapter service**
   - Likely NOT needed — Elasticsearch query API is compatible with OpenSearch
   - If minor differences exist, a thin adapter translating field names

6. **Test**
   - Deploy locally with k3d alongside openchoreo-data-plane
   - Verify logs flow: container → Fluent Bit → Elasticsearch
   - Verify Observer API can query logs back

### Tracing Module Steps

1. **Scaffold** — copy `observability-tracing-opensearch/` as `observability-tracing-elk/`
2. **Helm chart** — replace OpenSearch dependency with Elasticsearch
3. **OTel Collector config** — change exporter from `opensearch` to `elasticsearch`
4. **Adapter** — likely not needed (same query API)
5. **Test** — verify traces flow: app → OTel Collector → Elasticsearch

---

## Module 2: Datadog

### Logs Module Steps

1. **Scaffold** `observability-logs-datadog/` — no backend chart (SaaS)
2. **Helm chart** — dependency: `fluent-bit` only
3. **Fluent Bit config**
   ```ini
   [OUTPUT]
       Name        datadog
       Match       kube.*
       apikey      ${DD_API_KEY}
       dd_service  openchoreo
       dd_source   kubernetes
       dd_tags     env:${ENVIRONMENT}
       TLS         On
       provider    ecs
   ```
4. **Adapter** — required, translate Observer API → Datadog Logs API (`/api/v2/logs/events/search`)
5. **Init** — minimal, create log pipelines/indexes via Datadog API (optional)

### Tracing Module Steps

1. **Scaffold** `observability-tracing-datadog/`
2. **OTel Collector exporter**: `datadog` exporter with API key
3. **Adapter** — required, translate Observer trace queries → Datadog Trace Search API
4. **Init** — minimal

---

## Module 3: New Relic

### Logs Module Steps

1. **Scaffold** `observability-logs-newrelic/`
2. **Fluent Bit OUTPUT**: `Name nrlogs` with `api_key` and `endpoint`
3. **Adapter** — required, translate Observer queries to NRQL via New Relic API
4. **Init** — minimal (SaaS, no index setup needed)

### Tracing Module Steps

1. **Scaffold** `observability-tracing-newrelic/`
2. **OTel Collector exporter**: `otlp` exporter pointing to New Relic's OTLP endpoint
3. **Adapter** — required, translate trace queries to NRQL
4. **Init** — minimal

---

## Module 4: Splunk

### Logs Module Steps

1. **Scaffold** `observability-logs-splunk/`
2. **Fluent Bit OUTPUT**: `Name splunk` with HEC token, host, port
3. **Adapter** — required, translate Observer queries to SPL (Splunk search language)
4. **Init** — create indexes and HEC inputs via Splunk API
5. **Note**: supports both Splunk Cloud (SaaS) and self-hosted Splunk Enterprise

### Tracing Module Steps

1. **Scaffold** `observability-tracing-splunk/`
2. **OTel Collector exporter**: `splunk_hec` exporter
3. **Adapter** — required, translate trace queries to SPL
4. **Init** — create trace indexes

---

## Module 5: AWS CloudWatch

### Logs Module Steps

1. **Scaffold** `observability-logs-cloudwatch/`
2. **Fluent Bit OUTPUT**: `Name cloudwatch_logs` with `region`, `log_group_name`, `log_stream_prefix`
3. **Auth**: AWS credentials via IRSA (IAM Roles for Service Accounts) or secret
4. **Adapter** — required, translate Observer queries to CloudWatch Logs Insights API
5. **Init** — create log groups and retention settings

### Tracing Module Steps

1. **Scaffold** `observability-tracing-xray/`
2. **OTel Collector exporter**: `awsxray` exporter
3. **Adapter** — required, translate trace queries to X-Ray API
4. **Init** — minimal (X-Ray manages storage automatically)

---

## Module 6: Azure Monitor

### Logs Module Steps

1. **Scaffold** `observability-logs-azure-monitor/`
2. **Fluent Bit OUTPUT**: `Name azure` with `Customer_ID` (workspace ID) and `Shared_Key`
3. **Adapter** — required, translate Observer queries to Azure Log Analytics (KQL)
4. **Init** — minimal (Azure manages tables automatically)

### Tracing Module Steps

1. **Scaffold** `observability-tracing-azure-monitor/`
2. **OTel Collector exporter**: `azuremonitor` exporter with instrumentation key
3. **Adapter** — required, translate trace queries to Application Insights API
4. **Init** — minimal

---

## Module 7: Google Cloud Observability

### Logs Module Steps

1. **Scaffold** `observability-logs-gcloud/`
2. **Fluent Bit OUTPUT**: `Name stackdriver` with GCP credentials (Workload Identity or key file)
3. **Adapter** — required, translate Observer queries to Cloud Logging API
4. **Init** — minimal (GCP manages log buckets automatically)

### Tracing Module Steps

1. **Scaffold** `observability-tracing-gcloud/`
2. **OTel Collector exporter**: `googlecloud` exporter
3. **Adapter** — required, translate trace queries to Cloud Trace API
4. **Init** — minimal

---

## Common Work (Do Before or Alongside Module 1)

- [ ] Document the Observer API contract that adapters must implement (logs + traces)
- [ ] Extract a shared adapter SDK/interface from the `openobserve` adapter if patterns emerge
- [ ] Define a standard testing procedure: deploy module → generate logs/traces → verify query round-trip
- [ ] Add CI templates in `community-modules` for building and publishing new module images
