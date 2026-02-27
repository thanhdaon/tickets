# Migrating Read Models

## Migration Steps

To migrate a read model, follow these steps:

1. **Query events from the data lake** — Process events one by one, from oldest to newest
2. **Map event versions** — Your read model may use newer event versions than what's stored in the data lake
3. **Call repository methods directly** — No need to publish messages; call read model repository methods directly (similar to what you'd implement in a standard read model)

## Tips

**Long-running migrations**: For lengthy migrations, implement a resume mechanism. Store the last processed event timestamp and resume from that point if interrupted.

**No data lake?** You can reverse-engineer a read model from your write model. Query production tables (e.g., `bookings`) and build the read model from that data.

**Keep it simple**: Call repository methods directly rather than going through message handlers—you're rebuilding state, not processing live events.

**Cut-off dates**: You may not need to migrate from the oldest data. Choose a cut-off date based on your use case. When building from newer data, version mapping may not be needed.

**Add logging**: Migrations often take longer than expected or fail unexpectedly. Log progress, event counts, and elapsed time to aid debugging and monitoring.
