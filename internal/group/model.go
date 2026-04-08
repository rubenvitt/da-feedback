package group

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("group not found")

type Group struct {
	ID              int
	Name            string
	Slug            string
	Secret          string
	CloseAfterHours *int
	CreatedAt       time.Time
}
