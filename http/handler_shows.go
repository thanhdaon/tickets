package http

import (
	"net/http"
	"tickets/entities"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type PostShowsRequest struct {
	DeadNationID    uuid.UUID `json:"dead_nation_id"`
	NumberOfTickets int       `json:"number_of_tickets"`
	StartTime       time.Time `json:"start_time"`
	Title           string    `json:"title"`
	Venue           string    `json:"venue"`
}

type PostShowsResponse struct {
	ShowID uuid.UUID `json:"show_id"`
}

func (h Handler) GetShows(c echo.Context) error {
	shows, err := h.shows.FindAll(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, shows)
}

func (h Handler) PostShows(c echo.Context) error {
	var request PostShowsRequest

	if err := c.Bind(&request); err != nil {
		return err
	}

	showID := uuid.New()

	show := entities.Show{
		ShowID:          showID,
		DeadNationID:    request.DeadNationID,
		NumberOfTickets: request.NumberOfTickets,
		StartTime:       request.StartTime,
		Title:           request.Title,
		Venue:           request.Venue,
	}

	if err := h.shows.AddShow(c.Request().Context(), show); err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, PostShowsResponse{ShowID: showID})
}
