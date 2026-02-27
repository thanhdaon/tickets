## What Is an Event?

An event is a message that represents **something that already happened** — an immutable fact. Technically, it's the same as a plain message on the wire; the difference is conceptual.

### Tight coupling without events

```go
func PlaceOrder(order Order) {
    SaveOrder(order)
    NotifyUser(order)
    NotifySales(order)
    NotifyWarehouse(order)
    GenerateInvoice(order)
    ChargeCustomer(order)
}
```

`PlaceOrder` is coupled to every downstream concern. Adding a new action requires changing this function.

### Decoupled with events

```go
func PlaceOrder(order Order) {
    SaveOrder(order)
    PublishOrderPlaced(order) // emit fact, don't orchestrate
}
```

`PlaceOrder` only stores the order and announces what happened. Any number of independent subscribers react to `OrderPlaced` without the publisher knowing or caring.

**Benefits:**

- Add new behaviour by subscribing — no changes to the publisher
- Events form an audit log of system activity
- Teams work independently on subscribers

### Event naming

Events should be **past-tense verbs** describing what happened, not what should happen next.

| Good             | Bad                                     |
| ---------------- | --------------------------------------- |
| `OrderPlaced`    | `NotificationOfUserOnOrderShouldBeSent` |
| `UserSignedUp`   | `SendWelcomeEmailIsReadyToSend`         |
| `AlarmTriggered` | `AlarmNeedsToBeEnabled`                 |

### Primitive types vs. complex types in event structs

| Approach                               | Pros                     | Cons                                                   |
| -------------------------------------- | ------------------------ | ------------------------------------------------------ |
| Complex types (`time.Time`, custom)    | Less boilerplate         | Implicit marshalling behaviour, harder to change later |
| Primitive types only (`string`, `int`) | Explicit, easy to change | More boilerplate, separate pub/sub struct needed       |

Rule of thumb: use primitives when you have a clean domain layer; default marshallers are fine for simpler cases.

### Serialisation formats

| Format           | Notes                                                             |
| ---------------- | ----------------------------------------------------------------- |
| JSON             | Universal, human-readable                                         |
| Protocol Buffers | Typed schema, compact binary, code generation, can also emit JSON |
| Avro             | Schema registry support, compact                                  |

---

## Event Headers

Include common metadata in every event for debugging and observability.

```go
type MessageHeader struct {
    ID          string `json:"id"`
    EventName   string `json:"event_name"`
    OccurredAt  string `json:"occurred_at"`
}

type ProductCreated struct {
    Header    MessageHeader `json:"header"`
    ProductID string        `json:"product_id"`
    Name      string        `json:"name"`
}
```

Use a constructor to avoid repeating header creation logic:

```go
func NewMessageHeader(eventName string) MessageHeader {
    return MessageHeader{
        ID:         uuid.NewString(),
        EventName:  eventName,
        OccurredAt: time.Now().Format(time.RFC3339),
    }
}
```

---

## Summary

| Concept           | Detail                                                               |
| ----------------- | -------------------------------------------------------------------- |
| Event             | Immutable fact about something that happened; past-tense name        |
| Event vs. message | Conceptual difference only — same bytes on the wire                  |
| Decoupling        | Publisher emits fact; subscribers react independently                |
| Marshalling       | Marshal struct to `[]byte` payload; prefer explicit primitive types  |
| Headers           | Metadata (ID, event name, timestamp) embedded in every event payload |
