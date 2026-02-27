## Why Message Brokers?

Goroutines are not durable — messages in memory are lost on restart. A **message broker** (Pub/Sub) solves this by acting as a persistent middleman between publishers and subscribers.

```
Publisher --> Broker (Pub/Sub) --> Subscriber
```

Popular options: Apache Kafka, RabbitMQ, AWS SNS/SQS, Redis Streams, NATS, PostgreSQL (via Watermill SQL plugin).

---

## Watermill

[Watermill](https://watermill.io/) is a Go library that abstracts Pub/Sub behind two interfaces:

```go
type Publisher interface {
    Publish(topic string, messages ...*Message) error
    Close() error
}

type Subscriber interface {
    Subscribe(ctx context.Context, topic string) (<-chan *Message, error)
    Close() error
}
```

This mirrors how `net/http` abstracts HTTP — you work with concepts, not protocol details.

---

## Publishing Messages

```go
// Create publisher
rdb := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})
publisher, err := redisstream.NewPublisher(redisstream.PublisherConfig{Client: rdb}, logger)

// Create and publish a message
msg := message.NewMessage(watermill.NewUUID(), []byte(orderID))
err = publisher.Publish("orders", msg)
```

- `NewMessage(uuid, payload)` — UUID is for debugging; payload is `[]byte`
- Messages are delivered FIFO (verify with your specific broker)

---

## Subscribing to Messages

```go
subscriber, err := redisstream.NewSubscriber(redisstream.SubscriberConfig{Client: rdb}, logger)
messages, err := subscriber.Subscribe(context.Background(), "orders")

for msg := range messages {
    fmt.Println("Order:", string(msg.Payload))
    msg.Ack() // REQUIRED — broker holds next message until ack is received
}
```

**Common mistake**: forgetting `msg.Ack()` causes the subscriber to stall after the first message.

---

## Ack / Nack

On failure, call `Nack()` to return the message to the queue for redelivery.

```go
for msg := range messages {
    err := SaveToDatabase(string(msg.Payload))
    if err != nil {
        msg.Nack() // broker redelivers the message
        continue
    }
    msg.Ack()
}
```

| Call     | Meaning                                           |
| -------- | ------------------------------------------------- |
| `Ack()`  | Message processed successfully, broker removes it |
| `Nack()` | Processing failed, broker redelivers              |

---

## Consumer Groups

Without consumer groups, two replicas of the same service receive and process every message twice.

A **consumer group** ensures each message is delivered to **only one subscriber** within the group. Subscribers in the same group share the load (typically round-robin).

Benefits:

- Horizontal scaling — add replicas without duplicate processing
- Durability — broker tracks position per group, replayed on restart

```go
subscriber, err := redisstream.NewSubscriber(
    redisstream.SubscriberConfig{
        Client:        redisClient,
        ConsumerGroup: "notifications",
    },
    logger,
)
```

Different groups receive **independent copies** of each message:

```
orders-placed topic
    --> group "notifications"   → one of its subscribers processes each message
    --> group "spreadsheets"    → one of its subscribers processes each message
```

**Equivalent concepts in other brokers**: Kafka → consumer groups, RabbitMQ → queues, GCP Pub/Sub / NATS → subscriptions, AWS SNS → SQS queues.

---

## Summary

| Concept        | Detail                                                             |
| -------------- | ------------------------------------------------------------------ |
| Message broker | Persistent middleman; solves goroutine durability problem          |
| Watermill      | Go abstraction over Pub/Sub; `Publisher` + `Subscriber` interfaces |
| Publish        | `publisher.Publish(topic, msg)`                                    |
| Subscribe      | `subscriber.Subscribe(ctx, topic)` returns `<-chan *Message`       |
| Ack            | Confirm success; broker delivers next message                      |
| Nack           | Signal failure; broker redelivers                                  |
| Consumer group | Scoped delivery for scaling and durability                         |
