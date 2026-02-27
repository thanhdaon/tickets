# Observability in Event-Driven Systems

## Why It's Harder

In synchronous systems, a failed request is immediately visible. In event-driven systems:
- Event emission succeeds, but processing may fail silently
- Failures are delayed and distributed across services
- The "happy path" hides problems until they accumulate

**Trade-off**: Better fault tolerance and decoupling, but harder to see when things break.

## Learn More

- [Tracing](tracing.md) — Distributed tracing concepts and OpenTelemetry
- [Metrics](metrics.md) — Key metrics to track and common measurement pitfalls
- [Alerting](alerting.md) — Alert types, thresholds, and incident handling
