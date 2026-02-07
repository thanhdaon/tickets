package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"tickets/entities"
)

type ShowRepository struct {
	db *sqlx.DB
}

func NewShowRepository(db *sqlx.DB) ShowRepository {
	if db == nil {
		panic("db is nil")
	}

	return ShowRepository{db: db}
}

func (s ShowRepository) AddShow(ctx context.Context, show entities.Show) error {
	_, err := s.db.NamedExecContext(ctx, `
        INSERT INTO
            shows (show_id, dead_nation_id, number_of_tickets, start_time, title, venue)
        VALUES (:show_id, :dead_nation_id, :number_of_tickets, :start_time, :title, :venue)
        `, show)
	if err != nil {
		return fmt.Errorf("could not add show: %w", err)
	}

	return nil
}

func (s ShowRepository) FindAll(ctx context.Context) ([]entities.Show, error) {
	var shows []entities.Show
	err := s.db.SelectContext(ctx, &shows, `SELECT * FROM shows ORDER BY start_time`)
	if err != nil {
		return nil, fmt.Errorf("could not find shows: %w", err)
	}

	return shows, nil
}

func (s ShowRepository) ShowByID(ctx context.Context, showID uuid.UUID) (entities.Show, error) {
	var show entities.Show
	err := s.db.GetContext(ctx, &show, `SELECT * FROM shows WHERE show_id = $1`, showID)
	if err != nil {
		return entities.Show{}, err
	}

	return show, nil
}
