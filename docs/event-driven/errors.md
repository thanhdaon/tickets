## Error Flow

```
Message → Handler → Success? → Yes → ACK
                             → No  → NACK → Pub/Sub Redelivers → Handler
```

When a handler returns an error, the broker receives a NACK and redelivers the message.

---

## Temporary Errors

Network and I/O failures (database down, external API timeout) are transient — retrying is the correct response. The message is redelivered until it succeeds.

**Benefits of retrying:**

- System auto-heals without human intervention
- No message loss during brief outages

### Retry strategies

| Strategy            | Behaviour                      | Trade-off                            |
| ------------------- | ------------------------------ | ------------------------------------ |
| Immediate redeliver | Fastest recovery               | Load spike if the dependency is down |
| Fixed delay         | Spreads load                   | Messages may arrive out of order     |
| Exponential backoff | Reduces pressure progressively | Slightly more complex to configure   |

### Retry middleware

```go
middleware.Retry{
    MaxRetries:      10,
    InitialInterval: time.Millisecond * 100,
    MaxInterval:     time.Second,
    Multiplier:      2,
    Logger:          logger,
}
```

Each retry doubles the delay (100ms → 200ms → 400ms … up to 1s), capped at `MaxRetries`.

---

## Malformed Messages

A message that cannot be processed due to bad data (broken JSON, wrong topic, unknown schema) will never succeed — retrying is pointless. Remove it from the queue by returning `nil`.

```go
// Discard messages with unexpected type
if msg.Metadata.Get("type") != "booking.created" {
    slog.Error("unexpected message type, discarding", "payload", string(msg.Payload))
    return nil
}
```

Always log the payload before discarding so the message can be investigated later.

### Removing a specific message by UUID

If a single bad message was published by mistake:

```go
if msg.UUID == "5f810ce3-222b-4626-bc04-cbfb460c98c7" {
    return nil
}
```

Pragmatic for quick fixes with a fast CI/CD pipeline, but requires every published message to have a unique UUID.

---

## Permanent Errors

Some domain errors are non-retryable by design (e.g. missing required field). Model them explicitly and skip NACK via middleware.

```go
type PermanentError interface {
    IsPermanent() bool
}

func SkipPermanentErrorsMiddleware(h message.HandlerFunc) message.HandlerFunc {
    return func(msg *message.Message) ([]*message.Message, error) {
        msgs, err := h(msg)
        var permErr PermanentError
        if errors.As(err, &permErr) && permErr.IsPermanent() {
            return nil, nil // ACK without retrying
        }
        return msgs, err
    }
}
```

Example permanent error type:

```go
type MissingInvoiceNumber struct{}

func (e MissingInvoiceNumber) Error() string   { return "missing invoice number" }
func (e MissingInvoiceNumber) IsPermanent() bool { return true }
```

Raise an alert when a permanent error occurs — it usually indicates a data or contract problem worth investigating.

---

## Code Bugs

Bugs that cause every message to fail are a special case. The retry loop accumulates failed messages until a fix is deployed.

**Fix-forward approach:**

```go
// Bug: rejects valid phone numbers like "+123456789"
phonePattern := regexp.MustCompile(`^\d+$`)

// Fix: accept leading "+"
phonePattern := regexp.MustCompile(`^\+\d+$`)
```

Once the fixed version is deployed, all queued messages are processed automatically. No manual intervention needed — this is where durable retries shine.

---

## Summary

| Error type        | Cause                      | Strategy                                         |
| ----------------- | -------------------------- | ------------------------------------------------ |
| Temporary         | Network/infra failure      | Retry with exponential backoff                   |
| Malformed message | Bad payload or wrong topic | Return `nil` (ACK and discard), log payload      |
| Permanent         | Domain rule violation      | Custom middleware, return `nil`, alert           |
| Code bug          | Handler logic error        | Fix-forward: deploy fix, retries drain the queue |
