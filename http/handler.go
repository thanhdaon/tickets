package http

import (
	"context"
	"tickets/entities"

	"github.com/ThreeDotsLabs/watermill/components/cqrs"
)

type Handler struct {
	eventBus    *cqrs.EventBus
	commandBus  *cqrs.CommandBus
	tickets     TicketRepository
	shows       ShowRepository
	bookings    BookingRepository
	opsBookings OpsBookingRepository
}

type ShowRepository interface {
	AddShow(ctx context.Context, show entities.Show) error
	FindAll(ctx context.Context) ([]entities.Show, error)
}

type TicketRepository interface {
	FindAll(ctx context.Context) ([]entities.Ticket, error)
}

type BookingRepository interface {
	AddBooking(ctx context.Context, booking entities.Booking) error
}

type OpsBookingRepository interface {
	FindAll(ctx context.Context, receiptIssueDate string) ([]entities.OpsBooking, error)
	FindByID(ctx context.Context, bookingID string) (entities.OpsBooking, error)
}
