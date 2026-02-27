# Read Models

## When to use read models

**High read throughput**: Build read-optimized models that scale horizontally, often using different databases ([Polyglot persistence](https://martinfowler.com/bliki/PolyglotPersistence.html)). Teams can choose storage formats optimized for their use cases.

**Legacy migration**: Emit events from legacy systems and build new read models in a decoupled way, avoiding large, hard-to-maintain data models.

**High resiliency requirements**: Aggregate data from multiple stores into one place to simplify reading operations and improve infrastructure stability.

**Event-driven cache**: Unlike traditional caches with complex invalidation policies, read models are updated by events and stay current automatically.

## Read models vs write models

- **Write model**: Strongly consistent, always up-to-date, guarantees domain invariants. This is your single source of truth (e.g., bookings table, tickets table).

- **Read model**: Optimized for reading, eventually consistent, derived from events.

Typical monoliths use only write models, which makes scaling and maintaining them difficult. A single source of truth simplifies system logic—update in one place, consumers adapt to changes.

## Tradeoffs

**Storage cost**: Data is duplicated across multiple databases (similar to RAID, HA, or horizontal scaling).

**Migration complexity**: Multiple models need updating instead of one. However, you can migrate incrementally and keep publicly-exposed models stable.

**Data bugs**: Fixing bugs requires emitting events to update all read models, not a single SQL update.

## Implementation details

To build a read model:
1. Consume all events with the data you need
2. Store this data in a format optimized for reading in your use case (Keep the logic as simple as possible)

**Use a prefix for read model tables**
It's good practice to have a prefix for read model tables, so you instantly know that this data is not the source of truth (write model) and is eventually consistent. Nobody should accidentally write to the read model tables.

For example, instead of `bookings` and `tickets`, use `ops_bookings` and `ops_tickets` for your operations dashboard read models.

This does not make sense if you use a different database for read models, like Elasticsearch or MongoDB—the database separation itself provides the clarity.

**No relational data or foreign keys in read models**
Remember, there's no relational data in the read model — it's a projection of the write model. Writing to multiple tables would add a lot of unnecessary overhead.

You also shouldn't define any foreign keys. This data is eventually consistent, and you don't have a guarantee that constraints will be satisfied at the time of the insert. Often, you'll want to store this data in a different database, likely NoSQL, so you won't be able to define any foreign keys.

**Keep business logic in the write model**: Your read model shouldn't decide if an invoice has been paid based on the paid amount—that logic belongs in the write model. The read model should just store facts (e.g., a `FullyPaid` flag from `InvoicePaymentReceived` event).

**Events are facts**: If you receive a `TicketPrinted` event, don't check if spots are available—that's the write model's responsibility. The read model stores what happened, not what should happen.

**Sanity checks are okay**: You can reject events missing critical data and raise alerts for investigation. Read models shouldn't blindly propagate invalid data.

**Out of order events**: You may receive events out of order (it's theoretically possible to receive `TicketPrinted` before `TicketBookingConfirmed`).

In such a scenario, you can return an error in the `TicketPrinted` handler (nack the message). Once `TicketBookingConfirmed` arrives and `TicketPrinted` is redelivered, you'll be able to process it correctly.

## Eventual consistency

Read models built from events are eventually consistent, not real-time. The delay is typically sub-second and imperceptible to users—a tradeoff for scalability and resilience.

This is a business decision, not purely technical. When discussing with stakeholders, avoid asking "does the data need to be consistent?" (answer: always yes). Instead, ask: "should booking a ticket fail if we can't update the operations dashboard immediately, or is a small delay acceptable?"

## Direct-to-API pattern

Unlike complex database models that shouldn't map directly to API models (tight coupling, hard to evolve), read models are read-only and track no invariants. You can return stored models directly from the API with no post-processing or joins.

This approach:
- Simplifies code and improves performance
- Scales horizontally and geographically
- Provides GraphQL-like benefits without the complexity
- Allows preparing read models with all necessary data upfront

## Example: Invoice Read Model

Consider an invoicing system that emits three events:

```go
type InvoiceIssued struct {
    InvoiceID    string
    CustomerName string
    Amount       decimal.Decimal
    IssuedAt     time.Time
}

type InvoicePaymentReceived struct {
    PaymentID  string
    InvoiceID  string
    PaidAmount decimal.Decimal
    PaidAt     time.Time
    FullyPaid  bool // Business logic from write model
}

type InvoiceVoided struct {
    InvoiceID string
    VoidedAt  time.Time
}
```

The read model aggregates data from these events:

```go
type InvoiceReadModel struct {
    InvoiceID    string
    CustomerName string
    Amount       decimal.Decimal
    IssuedAt     time.Time

    FullyPaid     bool
    PaidAmount    decimal.Decimal // Sum of all payments
    LastPaymentAt time.Time       // Max of all payment times

    Voided   bool
    VoidedAt time.Time
}
```

**Key implementation details:**

1. **Idempotency**: Track processed event IDs to handle duplicate delivery
   - `InvoiceIssued` deduplicated by `InvoiceID`
   - `InvoicePaymentReceived` deduplicated by `PaymentID` (not `InvoiceID`—multiple payments per invoice is valid)

2. **Use event flags, don't calculate**: The `FullyPaid` flag comes from the event, not calculated from payment totals. Business logic belongs in the write model.

3. **Handle missing dependencies**: If an event depends on data from a previous event (like `OnInvoicePaymentReceived` needing the invoice to exist), return an error. The framework will re-deliver the event, and by the time it retries, the dependency may be satisfied.

```go
// Creates invoice if it doesn't exist
func (s *Storage) OnInvoiceIssued(ctx context.Context, event *InvoiceIssued) error {
    if _, exists := s.invoices[event.InvoiceID]; exists {
        return nil // Idempotent: already created
    }

    s.invoices[event.InvoiceID] = InvoiceReadModel{
        InvoiceID:    event.InvoiceID,
        CustomerName: event.CustomerName,
        Amount:       event.Amount,
        IssuedAt:     event.IssuedAt,
    }
    return nil
}

// Accumulates payments for an invoice
func (s *Storage) OnInvoicePaymentReceived(ctx context.Context, event *InvoicePaymentReceived) error {
  if _, processed := s.payments[event.PaymentID]; processed {
      return nil // Idempotent: already processed this payment
  }

  invoice, exists := s.invoices[event.InvoiceID]
  if !exists {
      return fmt.Errorf("invoice not found: %s", event.InvoiceID)
      // Triggers retry - eventual consistency
  }

  s.payments[event.PaymentID] = struct{}{}

	invoice.FullyPaid = event.FullyPaid
	invoice.PaidAmount = invoice.PaidAmount.Add(event.PaidAmount)
	invoice.LastPaymentAt = event.PaidAt

	s.invoices[event.InvoiceID] = invoice
  return nil
}

// Marks invoice as voided
func (s *Storage) OnInvoiceVoided(ctx context.Context, event *InvoiceVoided) error {
  invoice, exists := s.invoices[event.InvoiceID]
  if !exists {
    return fmt.Errorf("invoice not found: %s", event.InvoiceID)
  }

 	invoice.Voided = true
	invoice.VoidedAt = event.VoidedAt

	s.invoices[event.InvoiceID] = invoice
  return nil
}
```
