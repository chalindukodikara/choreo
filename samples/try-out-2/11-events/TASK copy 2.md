What we want?
1. When a component creates people might need to trigger a build
2. When a build fails, others need to get the event into an external system
3. When a build fails, they want to get an email
4. When the deployment workload fails in this environment, they need to get a call to their system
5. When a project is created, they want to get an event
6. When a specific project created with this name, they want to get an event
7. etc

We need a good system for handling such events. Users should be able to define what they want, for what events, how they want it to be delivered.

We need to find,
1. Do we need to introduce a new CR for this?
2. How can we handle it with our controller architecture?
3. Can we reuse argo events for this?
4. Do we need a controller or service for the implementation?
5. Observability itself has notification mechanism only for logs, metrics, and distributed tracing alerts. How this one will be different? samples/component-alerts/alert-notification-channels.yaml, samples/component-alerts/alert-rule-trait.yaml
6. How will be the architecture of this design.

Propose a design and discuss about it.