package message

import (
	"context"
	"fmt"
	"tickets/entities"
	"tickets/message/command"
	"tickets/message/event"
	"tickets/message/outbox"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/jmoiron/sqlx"
)

type SpreadsheetsAPI interface {
	AppendRow(ctx context.Context, sheetName string, row []string) error
}

type ReceiptsService interface {
	IssueReceipt(context.Context, entities.IssueReceiptRequest) (entities.IssueReceiptResponse, error)
	VoidReceipt(ctx context.Context, ticketID, idempotencyKey string) error
}

type FileAPI interface {
	UploadFile(ctx context.Context, fileID, fileContent string) error
}

type PaymentsService interface {
	RefundPayment(ctx context.Context, ticketID, idempotencyKey string) error
}

type OpsBookingReadModel interface {
	OnBookingMade(context.Context, *entities.BookingMade_v1) error
	OnTicketBookingConfirmed(context.Context, *entities.TicketBookingConfirmed_v1) error
	OnTicketRefunded(context.Context, *entities.TicketRefunded_v1) error
	OnTicketPrinted(context.Context, *entities.TicketPrinted_v1) error
	OnTicketReceiptIssued(context.Context, *entities.TicketReceiptIssued_v1) error
}

type DataLake interface {
	StoreEvent(ctx context.Context, eventID string, eventHeader entities.MessageHeader, eventName string, payload []byte) error
}

type Router struct {
	router *message.Router
}

func NewRouter(
	subscriber message.Subscriber,
	publisher message.Publisher,
	eventProcessorConfig cqrs.EventProcessorConfig,
	commandProcessorConfig cqrs.CommandProcessorConfig,
	eventHandlers event.Handlers,
	commandHandlers command.Handlers,
	opsBookings OpsBookingReadModel,
	logger watermill.LoggerAdapter,
	db *sqlx.DB,
	eventsSplitterSubscriber message.Subscriber,
	dataLakeSubscriber message.Subscriber,
	dataLake DataLake,
) (*Router, error) {
	router := message.NewDefaultRouter(logger)
	outbox.AddForwarderHandler(subscriber, publisher, router, logger)

	router.AddConsumerHandler(
		"events_splitter",
		"events",
		eventsSplitterSubscriber,
		func(msg *message.Message) error {
			eventName := eventProcessorConfig.Marshaler.NameFromMessage(msg)
			if eventName == "" {
				return fmt.Errorf("cannot get event name from message")
			}

			return publisher.Publish("events."+eventName, msg)
		},
	)

	router.AddConsumerHandler(
		"store_to_data_lake",
		"events",
		dataLakeSubscriber,
		func(msg *message.Message) error {
			eventName := eventProcessorConfig.Marshaler.NameFromMessage(msg)
			if eventName == "" {
				return fmt.Errorf("cannot get event name from message")
			}

			var e struct {
				Header entities.MessageHeader `json:"header"`
			}

			if err := eventProcessorConfig.Marshaler.Unmarshal(msg, &e); err != nil {
				return fmt.Errorf("cannot unmarshal event: %w", err)
			}

			return dataLake.StoreEvent(msg.Context(), e.Header.ID, e.Header, eventName, msg.Payload)
		},
	)

	useMiddlewares(router)

	ep, err := cqrs.NewEventProcessorWithConfig(router, eventProcessorConfig)
	if err != nil {
		panic(err)
	}

	if _, err := ep.AddHandler(cqrs.NewEventHandler("StoreTicket", eventHandlers.StoreTicket)); err != nil {
		return nil, fmt.Errorf("failed to add StoreTicket handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("AppendToTracker", eventHandlers.AppendToTracker)); err != nil {
		return nil, fmt.Errorf("failed to add AppendToTracker handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("PrintTicket", eventHandlers.PrintTicket)); err != nil {
		return nil, fmt.Errorf("failed to add PrintTicket handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("TicketRefundToSheet", eventHandlers.TicketRefundToSheet)); err != nil {
		return nil, fmt.Errorf("failed to add TicketRefundToSheet handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("IssueReceipt", eventHandlers.IssueReceipt)); err != nil {
		return nil, fmt.Errorf("failed to add IssueReceipt handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("RemoveCanceledTicket", eventHandlers.RemoveCanceledTicket)); err != nil {
		return nil, fmt.Errorf("failed to add RemoveCanceledTicket handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("BookPlaceInDeadNation", eventHandlers.BookPlaceInDeadNation)); err != nil {
		return nil, fmt.Errorf("failed to add BookPlaceInDeadNation handler: %w", err)
	}

	if _, err := ep.AddHandler(cqrs.NewEventHandler("ops_read_model.OnBookingMade", opsBookings.OnBookingMade)); err != nil {
		return nil, fmt.Errorf("failed to add OnBookingMade handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("ops_read_model.OnTicketBookingConfirmed", opsBookings.OnTicketBookingConfirmed)); err != nil {
		return nil, fmt.Errorf("failed to add OnTicketBookingConfirmed handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("ops_read_model.OnTicketRefunded", opsBookings.OnTicketRefunded)); err != nil {
		return nil, fmt.Errorf("failed to add OnTicketRefunded handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("ops_read_model.OnTicketPrinted", opsBookings.OnTicketPrinted)); err != nil {
		return nil, fmt.Errorf("failed to add OnTicketPrinted handler: %w", err)
	}
	if _, err := ep.AddHandler(cqrs.NewEventHandler("ops_read_model.OnTicketReceiptIssued", opsBookings.OnTicketReceiptIssued)); err != nil {
		return nil, fmt.Errorf("failed to add OnTicketReceiptIssued handler: %w", err)
	}

	cp, err := cqrs.NewCommandProcessorWithConfig(router, commandProcessorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create command processor: %w", err)
	}

	if _, err := cp.AddHandler(cqrs.NewCommandHandler("RefundTicket", commandHandlers.RefundTicketHandler)); err != nil {
		return nil, fmt.Errorf("failed to add RefundTicket handler: %w", err)
	}

	return &Router{router}, nil
}

func (r Router) Run(ctx context.Context) error {
	return r.router.Run(ctx)
}

func (r Router) Running() chan struct{} {
	return r.router.Running()
}
