package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdHTTP "net/http"

	"github.com/ThreeDotsLabs/go-event-driven/v2/common/log"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/sync/errgroup"

	"tickets/db"
	ticketsHttp "tickets/http"
	"tickets/message"
	"tickets/message/command"
	"tickets/message/event"
	"tickets/message/outbox"
	"tickets/observability"
)

type Service struct {
	db            *sqlx.DB
	echoRouter    *echo.Echo
	msgsRouter    *message.Router
	dataLake      db.DataLake
	opsBookings   db.OpsBookingReadModel
	traceProvider *tracesdk.TracerProvider
}

func New(
	sqldb *sqlx.DB,
	rdb *redis.Client,
	spreadsheetsAPI message.SpreadsheetsAPI,
	receiptsService message.ReceiptsService,
	paymentsService message.PaymentsService,
	fileAPI message.FileAPI,
	deadNationAPI event.DeadNationAPI) (Service, error) {
	traceProvider := observability.ConfigureTraceProvider()

	logger := watermill.NewSlogLogger(slog.Default())

	publisher := log.CorrelationPublisherDecorator{
		Publisher: observability.TracingPublisherDecorator{
			Publisher: message.NewRedisPublisher(rdb, logger),
		},
	}

	eventBus := event.NewEventBus(publisher)
	epConfig := event.NewProcessorConfig(rdb, logger)
	cpConfig := command.NewProcessorConfig(rdb, logger)

	commandBus := command.NewCommandBus(publisher)

	tickets := db.NewTicketRepository(sqldb)
	shows := db.NewShowRepository(sqldb)
	bookings := db.NewBookingRepository(sqldb)
	eHandlers := event.NewHandlers(fileAPI, spreadsheetsAPI, receiptsService, tickets, shows, deadNationAPI, eventBus)
	cHandlers := command.NewHandlers(receiptsService, paymentsService, eventBus)

	opsBookings := db.NewOpsBookingReadModel(sqldb)

	subscriber := outbox.NewPostgresSubscriber(sqldb, logger)
	eventsSplitterSubscriber := message.NewRedisSubscriber(rdb, "svc-tickets.events_splitter", logger)
	dataLakeSubscriber := message.NewRedisSubscriber(rdb, "svc-tickets.store_to_data_lake", logger)
	dataLake := db.NewDataLake(sqldb)

	echoRouter := ticketsHttp.NewHttpRouter(eventBus, commandBus, tickets, shows, bookings, opsBookings)
	msgsRouter, err := message.NewRouter(subscriber, publisher, epConfig, cpConfig, eHandlers, cHandlers, opsBookings, logger, sqldb, eventsSplitterSubscriber, dataLakeSubscriber, dataLake)
	if err != nil {
		return Service{}, fmt.Errorf("failed to create message router: %w", err)
	}

	return Service{
		db:            sqldb,
		echoRouter:    echoRouter,
		msgsRouter:    msgsRouter,
		dataLake:      dataLake,
		opsBookings:   opsBookings,
		traceProvider: traceProvider,
	}, nil
}

func (s Service) Run(ctx context.Context) error {
	if err := db.InitializeDatabaseSchema(s.db); err != nil {
		return fmt.Errorf("failed to initialize database schema: %w", err)
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return s.msgsRouter.Run(ctx)
	})

	g.Go(func() error {
		<-s.msgsRouter.Running()

		err := s.echoRouter.Start(":8080")
		if err != nil && !errors.Is(err, stdHTTP.ErrServerClosed) {
			return err
		}

		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		return s.echoRouter.Shutdown(ctx)
	})

	g.Go(func() error {
		<-ctx.Done()
		return s.traceProvider.Shutdown(context.Background())
	})

	return g.Wait()
}
