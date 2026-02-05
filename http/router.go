package http

import (
	"strings"
	"tickets/db"

	libHttp "github.com/ThreeDotsLabs/go-event-driven/v2/common/http"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

func NewHttpRouter(eventBus *cqrs.EventBus, commandBus *cqrs.CommandBus, tickets db.TicketRepository, shows db.ShowRepository, bookings db.BookingRepository, opsBookings db.OpsBookingReadModel) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = libHttp.HandleError

	e.Use(RequestIDMiddleware())
	e.Use(BodyDumpMiddleware(func(c echo.Context) bool {
		return strings.HasPrefix(c.Request().URL.Path, "/static") || c.Request().URL.Path == "/" || !strings.HasPrefix(c.Request().URL.Path, "/api")
	}))
	e.Use(RequestLoggerMiddleware())
	e.Use(CorrelationIDMiddleware())
	e.Use(otelecho.Middleware("tickets"))
	e.Use(TraceIDMiddleware())

	handler := Handler{
		eventBus:    eventBus,
		commandBus:  commandBus,
		tickets:     tickets,
		shows:       shows,
		bookings:    bookings,
		opsBookings: opsBookings,
	}

	api := e.Group("/api")
	api.POST("/tickets-status", handler.PostTicketsStatus)
	api.GET("/tickets", handler.GetTickets)
	api.POST("/shows", handler.PostShows)
	api.POST("/book-tickets", handler.PostBookTickets)
	api.PUT("/ticket-refund/:ticket_id", handler.PutTicketRefund)

	api.GET("/ops/bookings", handler.GetOpsBookings)
	api.GET("/ops/bookings/:id", handler.GetOpsBookingByID)

	e.GET("/health", handler.GetHealthCheck)
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	frontendHandler := newFrontendHandler()
	e.GET("/*", frontendHandler.GetStaticFiles)

	return e
}
