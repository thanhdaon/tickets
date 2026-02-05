package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ThreeDotsLabs/go-event-driven/v2/common/log"
	"github.com/jmoiron/sqlx"

	"tickets/entities"
)

type dbExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type OpsBookingReadModel struct {
	db *sqlx.DB
}

func NewOpsBookingReadModel(db *sqlx.DB) OpsBookingReadModel {
	if db == nil {
		panic("db is nil")
	}

	return OpsBookingReadModel{db: db}
}

func (r OpsBookingReadModel) OnBookingMade(ctx context.Context, e *entities.BookingMade_v1) error {
	// this is the first event that should arrive, so we create the read model
	err := r.createReadModel(ctx, entities.OpsBooking{
		BookingID:  e.BookingID,
		Tickets:    map[string]entities.OpsTicket{},
		LastUpdate: time.Now(),
		BookedAt:   e.Header.PublishedAt,
	})
	if err != nil {
		return fmt.Errorf("could not create read model: %w", err)
	}

	return nil
}

func (r OpsBookingReadModel) OnTicketBookingConfirmed(ctx context.Context, e *entities.TicketBookingConfirmed_v1) error {
	return r.updateByBookingID(
		ctx,
		e.BookingID,
		func(rm entities.OpsBooking) (entities.OpsBooking, error) {

			ticket, exists := rm.Tickets[e.TicketID]
			if !exists {
				log.FromContext(ctx).With("ticket_id", e.TicketID).Debug("Creating ticket read model for ticket %s")
			}

			ticket.PriceAmount = e.Price.Amount
			ticket.PriceCurrency = e.Price.Currency
			ticket.CustomerEmail = e.CustomerEmail
			ticket.ConfirmedAt = e.Header.PublishedAt

			rm.Tickets[e.TicketID] = ticket

			return rm, nil
		},
	)
}

func (r OpsBookingReadModel) OnTicketRefunded(ctx context.Context, e *entities.TicketRefunded_v1) error {
	return r.updateByTicketID(
		ctx,
		e.TicketID,
		func(rm entities.OpsTicket) (entities.OpsTicket, error) {
			rm.RefundedAt = e.Header.PublishedAt

			return rm, nil
		},
	)
}

func (r OpsBookingReadModel) OnTicketPrinted(ctx context.Context, e *entities.TicketPrinted_v1) error {
	return r.updateByTicketID(
		ctx,
		e.TicketID,
		func(rm entities.OpsTicket) (entities.OpsTicket, error) {
			rm.PrintedAt = e.Header.PublishedAt
			rm.PrintedFileName = e.FileName

			return rm, nil
		},
	)
}

func (r OpsBookingReadModel) OnTicketReceiptIssued(ctx context.Context, e *entities.TicketReceiptIssued_v1) error {
	return r.updateByTicketID(
		ctx,
		e.TicketID,
		func(rm entities.OpsTicket) (entities.OpsTicket, error) {
			rm.ReceiptIssuedAt = e.IssuedAt
			rm.ReceiptNumber = e.ReceiptNumber

			return rm, nil
		},
	)
}

func (r OpsBookingReadModel) createReadModel(
	ctx context.Context,
	booking entities.OpsBooking,
) (err error) {
	payload, err := json.Marshal(booking)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO 
		    read_model_ops_bookings (payload, booking_id)
		VALUES
			($1, $2)
		ON CONFLICT (booking_id) DO NOTHING; -- read model may be already updated by another event - we don't want to override
`, payload, booking.BookingID)

	if err != nil {
		return fmt.Errorf("could not create read model: %w", err)
	}

	return nil
}

func (r OpsBookingReadModel) updateByBookingID(
	ctx context.Context,
	bookingID string,
	updateFunc func(ticket entities.OpsBooking) (entities.OpsBooking, error),
) (err error) {
	return updateInTx(
		ctx,
		r.db,
		sql.LevelRepeatableRead,
		func(ctx context.Context, tx *sqlx.Tx) error {
			rm, err := r.findByBookingID(ctx, bookingID, tx)
			if errors.Is(err, sql.ErrNoRows) {
				// events arrived out of order - it should spin until the read model is created
				return fmt.Errorf("read model for booking %s not exist yet", bookingID)
			} else if err != nil {
				return fmt.Errorf("could not find read model: %w", err)
			}

			updatedRm, err := updateFunc(rm)
			if err != nil {
				return err
			}

			return r.updateReadModel(ctx, tx, updatedRm)
		},
	)
}

func (r OpsBookingReadModel) updateByTicketID(
	ctx context.Context,
	ticketID string,
	updateFunc func(ticket entities.OpsTicket) (entities.OpsTicket, error),
) (err error) {
	return updateInTx(
		ctx,
		r.db,
		sql.LevelRepeatableRead,
		func(ctx context.Context, tx *sqlx.Tx) error {
			rm, err := r.findByTicketID(ctx, ticketID, tx)
			if errors.Is(err, sql.ErrNoRows) {
				// events arrived out of order - it should spin until the read model is created
				return fmt.Errorf("read model for ticket %s not exist yet", ticketID)
			} else if err != nil {
				return fmt.Errorf("could not find read model: %w", err)
			}

			ticket := rm.Tickets[ticketID]

			updatedRm, err := updateFunc(ticket)
			if err != nil {
				return err
			}

			rm.Tickets[ticketID] = updatedRm

			return r.updateReadModel(ctx, tx, rm)
		},
	)
}

func (r OpsBookingReadModel) updateReadModel(
	ctx context.Context,
	tx *sqlx.Tx,
	rm entities.OpsBooking,
) error {
	rm.LastUpdate = time.Now()

	payload, err := json.Marshal(rm)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO 
			read_model_ops_bookings (payload, booking_id)
		VALUES
			($1, $2)
		ON CONFLICT (booking_id) DO UPDATE SET payload = excluded.payload;
		`, payload, rm.BookingID)
	if err != nil {
		return fmt.Errorf("could not update read model: %w", err)
	}

	return nil
}

func (r OpsBookingReadModel) findByTicketID(
	ctx context.Context,
	ticketID string,
	db dbExecutor,
) (entities.OpsBooking, error) {
	var payload []byte

	err := db.QueryRowContext(
		ctx,
		"SELECT payload FROM read_model_ops_bookings WHERE payload::jsonb -> 'tickets' ? $1",
		ticketID,
	).Scan(&payload)
	if err != nil {
		return entities.OpsBooking{}, err
	}

	return r.unmarshalReadModelFromDB(payload)
}

func (r OpsBookingReadModel) findByBookingID(
	ctx context.Context,
	bookingID string,
	db dbExecutor,
) (entities.OpsBooking, error) {
	var payload []byte

	err := db.QueryRowContext(
		ctx,
		"SELECT payload FROM read_model_ops_bookings WHERE booking_id = $1",
		bookingID,
	).Scan(&payload)
	if err != nil {
		return entities.OpsBooking{}, err
	}

	return r.unmarshalReadModelFromDB(payload)
}

func (r OpsBookingReadModel) unmarshalReadModelFromDB(payload []byte) (entities.OpsBooking, error) {
	var dbReadModel entities.OpsBooking
	if err := json.Unmarshal(payload, &dbReadModel); err != nil {
		return entities.OpsBooking{}, err
	}

	if dbReadModel.Tickets == nil {
		dbReadModel.Tickets = map[string]entities.OpsTicket{}
	}

	return dbReadModel, nil
}

func (r OpsBookingReadModel) FindAll(ctx context.Context, receiptIssueDate string) ([]entities.OpsBooking, error) {
	var payloads [][]byte

	var query string
	var args []interface{}

	if receiptIssueDate != "" {
		query = `
			SELECT payload
			FROM read_model_ops_bookings
			WHERE booking_id IN (
				SELECT booking_id
				FROM read_model_ops_bookings,
					jsonb_each(payload->'tickets') AS ticket
				WHERE DATE(ticket.value->>'receipt_issued_at') = $1
			)
			ORDER BY payload->>'booked_at' DESC
		`
		args = []interface{}{receiptIssueDate}
	} else {
		query = "SELECT payload FROM read_model_ops_bookings ORDER BY payload->>'booked_at' DESC"
		args = []interface{}{}
	}

	err := r.db.SelectContext(ctx, &payloads, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not find all bookings: %w", err)
	}

	result := make([]entities.OpsBooking, 0, len(payloads))
	for _, payload := range payloads {
		booking, err := r.unmarshalReadModelFromDB(payload)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal booking: %w", err)
		}
		result = append(result, booking)
	}

	return result, nil
}

func (r OpsBookingReadModel) FindByID(ctx context.Context, bookingID string) (entities.OpsBooking, error) {
	booking, err := r.findByBookingID(ctx, bookingID, r.db)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entities.OpsBooking{}, fmt.Errorf("booking not found: %s", bookingID)
		}
		return entities.OpsBooking{}, fmt.Errorf("could not find booking by ID: %w", err)
	}

	return booking, nil
}
