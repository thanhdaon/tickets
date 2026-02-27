# Metrics in Event-Driven Systems

## Key Metrics

### 1. Event Processing Lag
Time between event emission and processing completion. Alert when lag exceeds acceptable threshold.

### 2. Queue Depth
Number of unprocessed events in each queue. Alert when depth grows continuously.

### 3. Processing Success/Failure Rate
Track success and failure counts by event type and handler. Alert when failure rate exceeds threshold.

### 4. Dead Letter Queue Size
Events that failed all retry attempts. Alert when DLQ receives any events (should be zero in healthy system).

### 5. Event Throughput
Events processed per second. Alert when throughput drops significantly from baseline.

## Pub/Sub Metrics

Beyond application-level metrics, message brokers often provide infrastructure metrics via [Prometheus exporters](https://prometheus.io/docs/instrumenting/exporters/):

- Messages in topic/subscription
- Oldest message age (useful for detecting stuck messages)
- Unacked message count

These are particularly useful for monitoring outbox queues—stuck messages often indicate a frozen system where events aren't being delivered.

## Common Pitfalls

1. **Only monitoring the "publish" side** — Publishing success ≠ processing success
2. **Missing correlation IDs** — Can't trace requests through multiple events
3. **High-cardinality metric labels** — Avoid labels with unbounded values (e.g., error messages, user IDs, request IDs). Each unique label combination creates a new time series, leading to high memory usage and performance issues. In worst cases, this can crash your application due to memory exhaustion
