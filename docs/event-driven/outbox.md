# The Outbox Pattern

## Learning Objectives

- Understand the dual-write problem and why naive event publishing is unsafe
- Implement the Outbox pattern using SQL as a transactional event store
- Use Watermill's SQL Pub/Sub to publish events within a database transaction
- Forward events from SQL to a real message broker (Postgres → Redis)
- Use the Forwarder envelope pattern to route multiple event types through a single outbox topic

---

## Background: Replacing Dead Nation (Strangler Pattern)

When migrating away from a third-party system (Dead Nation), use the **Strangler Pattern**:

1. Create a new endpoint that intercepts all requests
2. Proxy them to both old and new systems in parallel
3. Once new system is validated, remove the legacy dependency

Key principle: **never do a big-bang rewrite** — migrate step by step and verify each step.

---

## The Dual-Write Problem

Publishing an event after committing a DB transaction is not safe:

```
Service → DB: commit booking ✓
Service → PubSub: publish BookingMade ✗ (service dies)
```

Result: booking exists in DB, event never published → downstream consumers never notified.

Reversing the order (publish before commit) is also wrong:

```
Service → PubSub: publish BookingMade ✓
Service → DB: commit booking ✗ (service dies)
```

Result: event published but transaction rolled back → **overbooking possible**.

---

## Solution: The Outbox Pattern

**Store events in the same DB transaction as business data, then forward them to Pub/Sub asynchronously.**

```
Service → DB (transaction):
  - Store business data
  - Store event in outbox table
  → Commit

Forwarder (loop):
  - Query unforwarded events
  - Publish to Pub/Sub
  - Mark as forwarded
  → Commit
```

If the forwarder dies mid-publish, the event is republished on restart → **at-least-once delivery**. This is safe as long as handlers are idempotent.

---

## Implementation with Watermill SQL Pub/Sub

### 1. Publish Within a Transaction

Create a SQL publisher scoped to the active transaction:

```go
import watermillSQL "github.com/ThreeDotsLabs/watermill-sql/v3/pkg/sql"

func PublishInTx(msg *message.Message, tx *sqlx.Tx) error {
    publisher, err := watermillSQL.NewPublisher(
        tx, // ContextExecutor — binds publisher to this transaction
        watermillSQL.PublisherConfig{
            SchemaAdapter: watermillSQL.DefaultPostgreSQLSchema{},
        },
        watermill.NewSlogLogger(nil),
    )
    if err != nil {
        return err
    }
    return publisher.Publish("ItemAddedToCart", msg)
}
```

> A new publisher is created per transaction — this is intentional and has negligible overhead.

### 2. Subscribe from SQL

```go
subscriber, err := watermillSQL.NewSubscriber(
    db,
    watermillSQL.SubscriberConfig{
        SchemaAdapter:  watermillSQL.DefaultPostgreSQLSchema{},
        OffsetsAdapter: watermillSQL.DefaultPostgreSQLOffsetsAdapter{},
    },
    logger,
)

// Required: initialize tables before subscribing
err = subscriber.SubscribeInitialize(topic)

messages, err := subscriber.Subscribe(ctx, topic)
```

SQL Pub/Sub creates two tables:
- `watermill_<topic>` — message storage
- `watermill_offsets_<topic>` — per-consumer-group offset tracking

> SQL Pub/Sub can replace a real message broker when deploying one is not feasible.

### 3. Forward SQL → Redis (Forwarder)

```go
import "github.com/ThreeDotsLabs/watermill/components/forwarder"

func RunForwarder(db *sqlx.DB, rdb *redis.Client, outboxTopic string, logger watermill.LoggerAdapter) error {
    sqlSubscriber, _ := watermillSQL.NewSubscriber(db, ...)
    sqlSubscriber.SubscribeInitialize(outboxTopic)

    redisPublisher, _ := redisstream.NewPublisher(...)

    fwd, err := forwarder.NewForwarder(sqlSubscriber, redisPublisher, logger, forwarder.Config{
        ForwarderTopic: outboxTopic,
    })
    if err != nil {
        return err
    }

    go func() {
        if err := fwd.Run(context.Background()); err != nil {
            panic(err)
        }
    }()

    <-fwd.Running() // wait for forwarder to be ready
    return nil
}
```

---

## Multi-Topic Forwarding via Envelope Pattern

Rather than one SQL table per event type, use a **single outbox topic** with the Forwarder publisher decorator. The decorator wraps messages in an envelope containing the destination topic.

```
Publisher → events_to_forward (Postgres)
Forwarder reads events_to_forward → routes to ItemAddedToCart, OrderSent (Redis)
```

### Setup

```go
const outboxTopic = "events_to_forward"

func PublishInTx(topic string, msg *message.Message, tx *sql.Tx, logger watermill.LoggerAdapter) error {
    sqlPublisher, _ := watermillSQL.NewPublisher(tx, watermillSQL.PublisherConfig{
        SchemaAdapter: watermillSQL.DefaultPostgreSQLSchema{},
    }, logger)

    // Wrap publisher — all publishes are routed through outboxTopic with destination in envelope
    wrappedPublisher := forwarder.NewPublisher(sqlPublisher, forwarder.PublisherConfig{
        ForwarderTopic: outboxTopic,
    })

    return wrappedPublisher.Publish(topic, msg) // topic stored in envelope, not used as actual destination
}
```

The Forwarder reads from `events_to_forward`, extracts the destination topic from the envelope, and re-publishes to Redis.

---

## Summary

| Component | Role |
|---|---|
| `watermillSQL.NewPublisher(tx, ...)` | Transactional write to SQL outbox |
| `watermillSQL.NewSubscriber(db, ...)` | Read from SQL outbox |
| `subscriber.SubscribeInitialize(topic)` | Creates SQL tables (must call before first use) |
| `forwarder.NewForwarder(sub, pub, ...)` | Forwards messages from SQL → Redis |
| `forwarder.NewPublisher(pub, ...)` | Wraps publisher to envelope messages for single outbox topic routing |

**Flow:** Business data + outbox event written atomically → Forwarder polls SQL → publishes to Redis Pub/Sub → consumers process idempotently.
