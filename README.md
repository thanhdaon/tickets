# Tickets

A Go-based event ticketing system demonstrating event-driven architecture patterns including CQRS, Event Sourcing, and the Outbox Pattern.

## Features

- **Ticket Management** - Create, track, and manage event tickets
- **Show Management** - Configure shows with external provider integration (Dead Nation)
- **Booking System** - Book tickets for shows with confirmation workflows
- **Refund Processing** - Handle ticket cancellations with receipt voiding and payment refunds
- **Event-Driven Architecture** - Loosely coupled components communicating via events and commands
- **Observability** - Distributed tracing, metrics, and health checks

## Architecture

### Key Patterns

- **CQRS** (Command Query Responsibility Segregation) - Separate command and event buses
- **Event Sourcing** - Events as the source of truth for state changes
- **Outbox Pattern** - Reliable event publishing with PostgreSQL-based outbox
- **Read Models** - Denormalized views for efficient queries (e.g., ops bookings)

### External Integrations

- **Dead Nation API** - External ticket booking provider
- **Receipts Service** - Issue and void receipts
- **Payments Service** - Process refunds
- **Files API** - Upload ticket files
- **Spreadsheets API** - Track tickets for printing/refunding

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.25.5 |
| HTTP Framework | Echo v4 |
| Event Bus | Watermill |
| Message Broker | Redis Streams |
| Database | PostgreSQL |
| Cache | Redis |
| Tracing | OpenTelemetry + Jaeger |
| Metrics | Prometheus |

## Project Structure

```
tickets/
├── cmd/server/         # Application entrypoint
├── adapters/           # External service adapters (Dead Nation, Payments, Receipts, Files)
├── db/                 # Database repositories and migrations
├── entities/           # Domain entities, events, and commands
├── http/               # HTTP handlers and routing
├── message/
│   ├── command/        # Command bus configuration and handlers
│   ├── event/          # Event bus configuration and handlers
│   └── outbox/         # Outbox pattern implementation
├── observability/      # Tracing and metrics configuration
├── service/            # Service composition and startup
├── tests/              # Component and integration tests
└── docker/             # Docker configuration (Prometheus)
```

## Getting Started

### Prerequisites

- Go 1.25.5+
- Docker & Docker Compose

### Environment Setup

Create `.env.local` for development:

```env
POSTGRES_URL=postgres://user:password@localhost:5432/db?sslmode=disable
REDIS_URL=localhost:6379
GATEWAY_URL=http://localhost:8888
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

Create `.env.test` for testing with similar configuration.

### Running with Docker

```bash
# Start all dependencies (PostgreSQL, Redis, Prometheus, Jaeger)
make docker-up

# Stop and remove volumes
make docker-down
```

### Development

```bash
# Run the server
make dev

# Run tests
make test

# Run tests with verbose output
make test-verbose

# Run linter
make lint
```

## API Endpoints

### Ticket Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/tickets` | List all tickets |
| POST | `/tickets-status` | Update ticket status |
| PUT | `/ticket-refund/:ticket_id` | Initiate ticket refund |

### Show Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/shows` | Create a new show |

### Booking Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/book-tickets` | Book tickets for a show |

### Operations & Monitoring

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/ops/bookings` | List all bookings (with optional date filter) |
| GET | `/ops/bookings/:id` | Get booking by ID |
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

## Events

The system publishes and handles the following events:

- `TicketBookingConfirmed_v1` - Ticket booking confirmed
- `TicketBookingCanceled_v1` - Ticket booking canceled
- `TicketPrinted_v1` - Ticket file generated
- `TicketReceiptIssued_v1` - Receipt issued for ticket
- `TicketRefunded_v1` - Ticket refund completed
- `BookingMade_v1` - Booking created for a show

## Commands

- `RefundTicket` - Initiates the ticket refund process

## Testing

```bash
# Run all tests
make test

# Run with verbose output
make test-verbose
```

Tests cover:
- Component integration tests
- Ticket booking workflows
- Show booking workflows
- Ticket cancellation flows
- Refund processing

## Observability

### Jaeger (Tracing)

Access the Jaeger UI at `http://localhost:16686` to view distributed traces.

### Prometheus (Metrics)

Access Prometheus at `http://localhost:9090`. Metrics are exposed at `/metrics`.

## Docker Services

| Service | Port | Description |
|---------|------|-------------|
| Application | 8080 | Main application |
| Gateway | 8888 | Event-driven gateway |
| PostgreSQL | 5432 | Database |
| Redis | 6379 | Message broker & cache |
| Prometheus | 9090 | Metrics collection |
| Jaeger | 16686 | Tracing UI |
| Jaeger OTLP | 4318 | OTLP collector |
