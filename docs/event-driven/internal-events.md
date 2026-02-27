# Internal Events

Internal events are events consumed only by one service or team. They should not be published to the data lake. They don't need backward compatibility guarantees, making them easier to change in the future. Think of them as encapsulation (like public/private methods).

**Common naming conventions**:
- Internal: `private`
- External: `public`, `integration`

## When to Use Internal Events

There is no simple answer. Internal events make your external contract smaller, but you expose less information to other teams for integration or data analytics. It's a tradeoff based on whether this event may change and whether it may be useful for other teams.

Internal events are also a good choice for very technical events that are not related to the domain.

**If you're not sure**: Start with an internal event and expose it later.

## Implementation Approach

Create a separate topic prefix for internal events: `internal-events.svc-tickets.`. This clearly indicates that this topic is used only for internal events of `svc-tickets`. Nobody else should consume it.

## Example: InternalOpsReadModelUpdated

A good use case for internal events is `InternalOpsReadModelUpdated`. We may emit it after each update of the read model and use it for sending SSE updates to the frontend. Nobody else should depend on that event. We can change this event at any point without breaking any contract.
