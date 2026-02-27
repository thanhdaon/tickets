# Alerting in Event-Driven Systems

## Alert Grouping

**Group alerts by consumer group/subscription, not just topic.** Different consumers on the same topic have different failure severities:

- `FlightCancelled` → Customer notification handler = **Critical** (wake someone up)
- `FlightCancelled` → Reporting read model handler = **Warning** (can wait until morning)

Add labels for criticality and team ownership to route alerts correctly.

## Alert Types

| Alert | Group By | Description | Critical | Threshold |
|-------|----------|-------------|----------|-----------|
| Unprocessed messages on the topic | Consumer group, topic | If above zero, it may mean that your handlers are not fast enough to process messages or there is a spinning message. | Depends on count and group | Depends on the scale of your system and how it's counted; 1 may be too extreme |
| Unprocessed messages in outbox | Topic | If above zero, a process responsible for forwarding your messages may not work properly. | Forwarding from outbox should be fast and may block many processes in your system | Depends on the scale of your system and how it's counted; 1 may be too extreme |
| Oldest message on the topic | Consumer group, topic | It's a different metric that can show if some message is spinning. | Depends on group | Depends on the SLA of your system, but generally 1-5 minutes |
| Message processing duration | Topic, handler name, quantile | Higher values show performance issues related to message processing. | No, as long as you don't have a specific SLA for processing messages | Depends on your system, but usually 10-60s |
| Latency between message publish and consume | Topic, handler name / group | Higher values than usual shows performance issues of your messaging infrastructure or performance issues related to message processing. | No, as long as you don't have a specific SLA for processing messages | - |
| Message processing error rate | Topic, handler name / group | A higher rate shows that more transient failures are happening (may be not visible in other metrics if messages are processed after retries). | Depends on the handler/group and how often processing fails | Depends on our system |
| Message processing rate | Topic, handler name / group | Values that are higher than normal can show anomalies in the system (may be a DDoS or bug). | No, as long as the processing rate is not much higher than usual (10-100x) | Should be based on a normal processing rate in your system (you can have non-critical alert for 3x more messages and critical for 10x messages) |

## Setting Thresholds

- **Start aggressive, then relax** — noisy at first, but better than missing incidents
- **Establish baselines first** — collect metrics before setting thresholds
- **Minimum viable alert**: at least one alert for stuck messages
- Tools: [Prometheus Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/) or any alerting tool with Prometheus integration

## Incident Handling

Alerts require a process, not just tooling:

- **On-call rotation** — don't be the only person solving alerts
- **Consider split rotations** — business hours vs. off-hours to avoid fatigue
- **Team ownership** — "you build it, you run it" when team size permits
- Tools: [Grafana OnCall](https://grafana.com/products/oncall/), [PagerDuty](https://www.pagerduty.com/)

## Common Pitfalls

1. **Alert fatigue** — Start with few, high-signal alerts
2. **No baseline metrics** — Collect normal operation data before setting thresholds
