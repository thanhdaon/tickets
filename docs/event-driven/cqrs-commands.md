# CQRS Commands

## Commands vs Events

| Dimension           | Event                       | Command                                    |
| ------------------- | --------------------------- | ------------------------------------------ |
| Represents          | Something that **happened** | Something that **should happen**           |
| Naming              | Past tense (`OrderPlaced`)  | Imperative (`CapturePayment`)              |
| Consumers           | Many (fan-out)              | One (point-to-point)                       |
| Emitter cares about | Nothing — fire and forget   | A specific action being performed          |
| Result to emitter   | None (async)                | None (async) — use HTTP/RPC if sync needed |

### When to use a command

Use a command when you want to trigger a specific operation asynchronously and need exactly one service to handle it.

```
Orders Service --[OrderPlaced event]--> Analytics Service
                                    --> Notifications Service
                                    --> Inventory Service

Orders Service --[CapturePayment command]--> Payments Service
```

> CQRS does **not** require commands to be asynchronous. The only CQRS requirement is separating writes from reads in code. Async commands are one use case, not the definition.

---

## CommandBus

Works the same as `EventBus` — publish a command struct without manual marshaling:

```go
err := commandBus.Send(ctx, SendNotification{
    NotificationID: id,
    Email:          email,
    Message:        body,
})
```

---

## CommandProcessor

Mirror of `EventProcessor` — subscribes to a topic, unmarshals, dispatches to handlers:

```go
commandProcessor, err := cqrs.NewCommandProcessorWithConfig(
    router,
    cqrs.CommandProcessorConfig{
        GenerateSubscribeTopic: func(params cqrs.CommandProcessorGenerateSubscribeTopicParams) (string, error) {
            return "commands", nil
        },
        SubscriberConstructor: func(params cqrs.CommandProcessorSubscriberConstructorParams) (message.Subscriber, error) {
            return sub, nil
        },
        Marshaler: cqrs.JSONMarshaler{
            GenerateName: cqrs.StructName,
        },
        Logger: logger,
    },
)

err = commandProcessor.AddHandlers(
    cqrs.NewCommandHandler(
        "send_notification",
        func(ctx context.Context, cmd *SendNotification) error {
            return sender.SendNotification(ctx, cmd.NotificationID, cmd.Email, cmd.Message)
        },
    ),
)
```

---

## Summary

| Concept                  | Key Point                                                               |
| ------------------------ | ----------------------------------------------------------------------- |
| Command                  | Intent for a specific action; exactly one consumer                      |
| Event                    | Immutable fact; many consumers, emitter doesn't care about reaction     |
| Passive-aggressive event | Event named like a command (`NotificationShouldBeSent`) — avoid this    |
| CommandBus               | `Send(ctx, cmd)` — same pattern as `EventBus.Publish`                   |
| CommandProcessor         | Same config shape as `EventProcessor`; single-consumer topic            |
| Async vs sync            | Use commands for async; use HTTP/RPC when you need a synchronous result |
