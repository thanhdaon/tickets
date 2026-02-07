package message

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/go-event-driven/v2/common/log"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/lithammer/shortuuid/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var (
	messagesProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "messages",
			Name:      "processed_total",
			Help:      "The total number of processed messages",
		},
		[]string{"topic", "handler"},
	)

	messagesProcessingFailedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "messages",
			Name:      "processing_failed_total",
			Help:      "The total number of message processing failures",
		},
		[]string{"topic", "handler"},
	)

	messagesProcessingDurationSeconds = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  "messages",
			Name:       "processing_duration_seconds",
			Help:       "The total time spent processing messages",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"topic", "handler"},
	)
)

func useMiddlewares(router *message.Router) {
	router.AddMiddleware(middleware.Recoverer)

	router.AddMiddleware(middleware.Retry{
		MaxRetries:      3,
		InitialInterval: time.Millisecond * 400,
		MaxInterval:     time.Second,
		Multiplier:      2,
		Logger:          router.Logger(),
	}.Middleware)

	router.AddMiddleware(tracingMiddleware)

	router.AddMiddleware(correlationIdMiddleware)

	router.AddMiddleware(logMiddleware)

	router.AddMiddleware(metricsMiddleware)
}

func metricsMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		topic := message.SubscribeTopicFromCtx(msg.Context())
		handler := message.HandlerNameFromCtx(msg.Context())
		labels := prometheus.Labels{"topic": topic, "handler": handler}

		start := time.Now()

		msgs, err := next(msg)

		messagesProcessedTotal.With(labels).Inc()
		messagesProcessingDurationSeconds.With(labels).Observe(time.Since(start).Seconds())

		if err != nil {
			messagesProcessingFailedTotal.With(labels).Inc()
		}

		return msgs, err
	}
}

func logMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {

		ctx := msg.Context()
		logger := log.FromContext(ctx)
		logger = logger.With("message_id", msg.UUID)
		logger = logger.With("payload", string(msg.Payload))
		logger = logger.With("metadata", msg.Metadata)
		logger = logger.With("correlation_id", log.CorrelationIDFromContext(ctx))
		logger = logger.With("handler", message.HandlerNameFromCtx(ctx))

		logger.Info("Handling a message")

		msgs, err := next(msg)

		if err != nil {
			logger.With("error", err).Error("Error while handling a message")
		}

		return msgs, err
	}
}

func correlationIdMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		ctx := msg.Context()

		reqCorrelationID := msg.Metadata.Get("correlation_id")
		if reqCorrelationID == "" {
			reqCorrelationID = shortuuid.New()
		}

		ctx = log.ToContext(ctx, slog.With("correlation_id", reqCorrelationID))
		ctx = log.ContextWithCorrelationID(ctx, reqCorrelationID)

		msg.SetContext(ctx)

		return next(msg)
	}
}

func tracingMiddleware(next message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) (msgs []*message.Message, err error) {
		ctx := msg.Context()

		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(msg.Metadata))

		topic := message.SubscribeTopicFromCtx(ctx)
		handler := message.HandlerNameFromCtx(ctx)

		ctx, span := otel.Tracer("").Start(
			ctx,
			fmt.Sprintf("topic: %s, handler: %s", topic, handler),
			trace.WithAttributes(
				attribute.String("topic", topic),
				attribute.String("handler", handler),
			),
		)
		defer span.End()

		msg.SetContext(ctx)

		msgs, err = next(msg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return msgs, err
	}
}
