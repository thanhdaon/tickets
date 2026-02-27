## Why Router?

`Publisher` and `Subscriber` are low-level interfaces. The **Router** is Watermill's high-level API — analogous to an HTTP router — that wires together subscribers, publishers, and handler functions.

It handles:

- Subscriber/publisher orchestration
- Automatic `Ack`/`Nack` based on handler return value
- A consistent structure for all message processing logic

---

## Handlers (Subscribe → Process → Publish)

```go
router := message.NewDefaultRouter(logger)

router.AddHandler(
    "handler_name",       // unique name for debugging
    "subscriber_topic",   // input topic
    subscriber,
    "publisher_topic",    // output topic
    publisher,
    func(msg *message.Message) ([]*message.Message, error) {
        newMsg := message.NewMessage(watermill.NewUUID(), []byte("response"))
        return []*message.Message{newMsg}, nil
    },
)

err = router.Run(context.Background()) // blocking
```

- Handler returns `nil` error → message is **Acked**
- Handler returns an error → message is **Nacked** (redelivered)
- All returned messages are published to the output topic
- No need to call `Ack()` or `Nack()` manually

---

## Consumer Handlers (Subscribe → Process, no publish)

For handlers that only consume messages without publishing, use `AddConsumerHandler`:

```go
router.AddConsumerHandler(
    "handler_name",
    "subscriber_topic",
    subscriber,
    func(msg *message.Message) error {
        fmt.Println("Received:", string(msg.Payload))
        return nil
    },
)
```

`AddConsumerHandler` is a simpler interface — no publisher or output topic needed. Same Ack/Nack semantics apply.

> Note: previously called `AddNoPublisherHandler` in older Watermill versions.

---

## Summary

| Method               | Use case                                         |
| -------------------- | ------------------------------------------------ |
| `AddHandler`         | Subscribe → transform → publish to another topic |
| `AddConsumerHandler` | Subscribe → side effects only (no publishing)    |

| Behaviour         | Detail                                     |
| ----------------- | ------------------------------------------ |
| `return nil`      | Message Acked automatically                |
| `return err`      | Message Nacked, broker redelivers          |
| `router.Run(ctx)` | Blocking — add all handlers before calling |
