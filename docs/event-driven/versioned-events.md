# Versioned Events

## The Contract Problem

Publishing events to Pub/Sub creates a contract with other teams. Anyone in the company can consume these events. Once events are in the data lake, data science teams may depend on them.

**Key principle**: Design events carefully upfront - you may not get a chance to change them later.

## Backward-Compatible Changes

**Best strategy**: Add new fields without removing old ones.

- Keeps existing consumers working
- Tradeoff: payload grows over time
- Works for most real-world scenarios

## Non-Backward-Compatible Changes

Sometimes adding fields isn't an option:
- Event payload is already too large
- Domain logic changed when the event is emitted
- Required fields are no longer available

### Migration Strategy

1. Create a new event with new name and format (keep the old one)
2. Start emitting new events
3. Update consumers to use new events
4. Remove old events (may never happen - that's okay)

**Note**: Finding all event consumers is harder than finding API users. Events are consumed "indirectly" making usage tracking difficult.

## Event Versioning

Add version suffixes to all event types to support future migrations:

```go
// Instead of:
type TicketBookingConfirmed struct { ... }

// Use:
type TicketBookingConfirmed_v1 struct { ... }
```

**Benefits**:
- `TicketBookingConfirmed_v1` and `TicketBookingConfirmed_v2` are treated as separate events
- No risk of accidentally using wrong version
- V2 may have different meaning and trigger conditions than V1

**For existing systems**: Events without version numbers can be assumed to be V1.

## Avoid Over-Engineering Events

Teams often create events with all possible fields (sometimes 50+ fields). This creates maintenance burden when other teams start depending on them.

**Better approach**: Start minimal, add fields as needed with backward compatibility in mind.
