# CQRS Events

## The CQRS Pattern

CQRS separates reads from writes and pairs well with event-driven architectures, but it's equally valuable in synchronous systems.

The three building blocks used here:

| Component          | Responsibility                                                        |
| ------------------ | --------------------------------------------------------------------- |
| **EventBus**       | Publish events — handles marshaling and topic selection automatically |
| **EventHandler**   | Your business logic — receives a typed event struct                   |
| **EventProcessor** | Subscribe to topics, unmarshal messages, dispatch to handlers         |

### Flow

```
Event → EventBus → Marshaler → Publisher → Message Broker
                                                  ↓
                              Subscriber → EventProcessor → Marshaler → EventHandler
```

---

## EventBus

Hides marshaling and topic routing behind a single `Publish` call:

```go
err := eventBus.Publish(ctx, TicketBookingConfirmed{
    Header:        NewMessageHeader(),
    TicketID:      ticket.TicketID,
    Price:         ticket.Price,
    CustomerEmail: ticket.CustomerEmail,
})
```

### Configuration

```go
eventBus, err := cqrs.NewEventBusWithConfig(publisher, cqrs.EventBusConfig{
    GeneratePublishTopic: func(params cqrs.GenerateEventPublishTopicParams) (string, error) {
        return params.EventName, nil // use event name as topic
    },
    Marshaler: cqrs.JSONMarshaler{
        GenerateName: cqrs.StructName, // "DocumentPrinted" not "main.DocumentPrinted"
    },
    Logger: logger,
})
```

**`GenerateName`**: By default, `JSONMarshaler` uses the full qualified type name (e.g. `main.DocumentPrinted`). Use `cqrs.StructName` to strip the package prefix.

---

## EventProcessor

Subscribes to topics, unmarshals messages, and dispatches to the matching handler:

```go
ep, err := cqrs.NewEventProcessorWithConfig(
    router,
    cqrs.EventProcessorConfig{
        SubscriberConstructor: func(params cqrs.EventProcessorSubscriberConstructorParams) (message.Subscriber, error) {
            return sub, nil
        },
        GenerateSubscribeTopic: func(params cqrs.EventProcessorGenerateSubscribeTopicParams) (string, error) {
            return params.EventName, nil
        },
        Marshaler: cqrs.JSONMarshaler{
            GenerateName: cqrs.StructName,
        },
        Logger: logger,
    },
)

err = ep.AddHandlers(handlers...)
```

The `GenerateSubscribeTopic` must match the topic used by the EventBus.

---

## EventHandler

Implement a typed handler function; the framework infers the event type to subscribe to:

```go
cqrs.NewEventHandler(
    "ArticlePublishedHandler",
    func(ctx context.Context, event *ArticlePublished) error {
        fmt.Printf("Article %s published\n", event.ArticleID)
        return nil
    },
)
```

### Injecting Dependencies

Use a struct to hold dependencies, then pass methods as handler functions:

```go
type ArticlesHandler struct {
    notificationsService NotificationsService
}

func (h ArticlesHandler) NotifyUser(ctx context.Context, event *ArticlePublished) error {
    h.notificationsService.NotifyUser(event.ArticleID)
    return nil
}

func NewArticlesHandlers(svc NotificationsService) []cqrs.EventHandler {
    h := ArticlesHandler{notificationsService: svc}
    return []cqrs.EventHandler{
        cqrs.NewEventHandler("NotifyUserOnArticlePublished", h.NotifyUser),
    }
}
```

A single struct can have multiple handlers for the same or different events.

---

## Publisher Decorator

The decorator pattern wraps an interface to inject cross-cutting behavior without modifying the underlying implementation. Useful for adding correlation IDs, logging, or metrics to every published message.

```go
type CorrelationPublisherDecorator struct {
    message.Publisher
}

func (c CorrelationPublisherDecorator) Publish(topic string, messages ...*message.Message) error {
    for _, msg := range messages {
        msg.Metadata.Set("correlation_id", CorrelationIDFromContext(msg.Context()))
    }
    return c.Publisher.Publish(topic, messages...)
}
```

**Important**: when wrapping a concrete type (e.g. `*redisstream.Publisher`), declare a `message.Publisher` variable first:

```go
var pub message.Publisher
pub, err = redisstream.NewPublisher(...)
pub = CorrelationPublisherDecorator{pub}
```

---

## CQRS with Consumer Groups

Use `SubscriberConstructor` to create a unique consumer group per handler — enabling independent scaling and durable delivery:

```go
SubscriberConstructor: func(params cqrs.EventProcessorSubscriberConstructorParams) (message.Subscriber, error) {
    return redisstream.NewSubscriber(redisstream.SubscriberConfig{
        Client:        rdb,
        ConsumerGroup: "svc-tickets." + params.HandlerName,
    }, logger)
},
```

**Warning**: renaming a handler changes its consumer group name, creating a new group and potentially missing unprocessed messages. Cover this with alerting on consumer group lag.

Include the service name in the consumer group to avoid handler name collisions across services:

```
"svc-users." + handlerName
```

---

## Summary

| Concept                    | Key Point                                                              |
| -------------------------- | ---------------------------------------------------------------------- |
| EventBus                   | Single `Publish(ctx, event)` — no manual marshaling or topic selection |
| EventProcessor             | Subscribes, unmarshals, dispatches to typed handlers                   |
| EventHandler               | Typed function; event type determines subscription topic               |
| `cqrs.StructName`          | Strips package prefix from event names                                 |
| Publisher Decorator        | Wraps `message.Publisher` to inject metadata without changing handlers |
| Per-handler consumer group | `SubscriberConstructor` creates isolated group per handler             |
| Handler rename risk        | Changing handler name changes consumer group — can drop messages       |
