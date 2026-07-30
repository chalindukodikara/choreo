## Problem Statement

OpenChoreo currently lacks comprehensive event-driven automation capabilities. Users cannot trigger workflows based on:

- **External Events**: Cron schedules, webhooks from external systems, cloud events
- **Internal Lifecycle Events**: Component creation, promotions

This limitation forces users to:
- Manually trigger workflows repeatedly
- Build custom automation outside the platform

## Proposed Solution: Argo Events Integration

Integrate [Argo Events](https://argoproj.github.io/argo-events/) into the OpenChoreo control plane as an optional event-driven automation module.

### Architecture Overview

<img width="1173" height="447" alt="image" src="https://github.com/user-attachments/assets/218c3f23-ef11-4e6f-86d9-fbab0ce56494" />


### Core Components

1. **EventSource CRD**: Define event sources (cron, webhooks, resource watchers, etc.)
2. **Sensor CRD**: Define event listeners and triggers that create WorkflowRun or ComponentWorkflowRun resources
3. **Event Bus**: Message bus for event distribution (NATS, Kafka, or STAN)

### Integration with OpenChoreo

Sensors can trigger existing OpenChoreo resources:
- Create `WorkflowRun` resources to execute workflows
- Execute custom actions

## Use Cases

### 1. Scheduled Workflows
**Scenario**: Run database backup workflow every night at 2 AM

```yaml
apiVersion: argoproj.io/v1alpha1
kind: EventSource
metadata:
  name: cron-backup
spec:
  schedule:
    backup-schedule:
      schedule: "0 2 * * *"
```
### 2. Lifecycle Event Automation
**Scenario**: Send Slack notification when component is promoted to production

- Watch for `ReleaseBinding` creation in production environment
- Trigger notification workflow

There are many other use cases that may arise in the future.

## Technical Considerations

### Resource Watching
- Argo Events can watch Kubernetes resources including custom CRDs
- We can use resource watchers to react to OpenChoreo lifecycle events:
  - `Component` creation/updates
  - `ComponentRelease` creation (new releases)
  - `ReleaseBinding` status changes (deployment events)
  - `Environment` creation (provisioning triggers)

### Event Bus Selection
- **NATS**: Lightweight, recommended for single-cluster setups
- **Kafka**: Better for high-throughput, multi-cluster scenarios


### Pod Scaling and Resource Footprint

#### Base Pod Count Formula

```
Total Pods = 1 (controller-manager)
           + N (EventBus pods)
           + M (EventSource CRs)
           + P (Sensor CRs)
```

Where:
- **controller-manager**: 1 pod (required)
- **EventBus pods**: 3 by default (scales based on event bus type and configuration)
  - **NATS Streaming**: 3 pods minimum - (3-5)
  - **NATS JetStream**: 3-5 pods - Modern NATS with better performance and features
  - **Kafka**: 3+ pods - Enterprise-grade, best for high-throughput multi-cluster scenarios
- **EventSource CRs**: 1 pod per EventSource CR
- **Sensor CRs**: 1 pod per Sensor CR

#### Scaling Considerations

**EventSource Consolidation**: Each EventSource CR can define multiple event types (cron schedules, webhooks, resource watchers), reducing the need for multiple EventSource pods.

**Sensor Consolidation**: Each Sensor CR can define multiple triggers, allowing a single Sensor to handle multiple automation workflows.

## References

- [Argo Events Documentation](https://argoproj.github.io/argo-events/)
- [Argo Events Architecture](https://argoproj.github.io/argo-events/concepts/architecture/)
- [Event Source Types](https://argoproj.github.io/argo-events/concepts/event_source/)
- [Sensor Triggers](https://argoproj.github.io/argo-events/concepts/trigger/)
- [Multiple Events per EventSource](https://argoproj.github.io/argo-events/eventsources/multiple-events/)
- [EventBus Configuration](https://argoproj.github.io/argo-events/eventbus/)
- [Controller Manager Pods](https://argoproj.github.io/argo-events/managed-namespace)
