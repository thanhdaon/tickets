package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"tickets/adapters"
	"tickets/message"
	"tickets/service"

	"github.com/ThreeDotsLabs/go-event-driven/v2/common/clients"
	"github.com/ThreeDotsLabs/go-event-driven/v2/common/log"
	"github.com/jmoiron/sqlx"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	log.Init(slog.LevelDebug)

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	logger := slog.Default()
	rdb := message.NewRedisClient(os.Getenv("REDIS_ADDR"))
	defer func() {
		if err := rdb.Close(); err != nil {
			logger.Error("failed to close redis client", "error", err)
		}
	}()

	traceDB, err := otelsql.Open("postgres", os.Getenv("POSTGRES_URL"),
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
		otelsql.WithDBName("db"))
	if err != nil {
		panic(err)
	}
	db := sqlx.NewDb(traceDB, "postgres")
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("failed to close database", "error", err)
		}
	}()

	traceHTTPClient := &http.Client{
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				return fmt.Sprintf("HTTP %s %s %s", r.Method, r.URL.String(), operation)
			}),
		),
	}

	apiClients, err := clients.NewClientsWithHttpClient(
		os.Getenv("GATEWAY_ADDR"),
		func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Correlation-ID", log.CorrelationIDFromContext(ctx))
			return nil
		},
		traceHTTPClient,
	)
	if err != nil {
		panic(err)
	}

	spreadsheetsAPI := adapters.NewSpreadsheetsAPIClient(apiClients)
	receiptsService := adapters.NewReceiptsServiceClient(apiClients)
	paymentsService := adapters.NewPaymentsServiceClient(apiClients)
	fileAPI := adapters.NewFilesAPIClient(apiClients)
	deadnationAPI := adapters.NewDeadNationClient(apiClients)

	svc, err := service.New(db, rdb, spreadsheetsAPI, receiptsService, paymentsService, fileAPI, deadnationAPI)
	if err != nil {
		panic(err)
	}
	if err := svc.Run(ctx); err != nil {
		panic(err)
	}
}
