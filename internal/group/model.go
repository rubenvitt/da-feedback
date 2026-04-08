package group

import "time"

type Group struct {
	ID              int
	Name            string
	Slug            string
	Secret          string
	CloseAfterHours *int
	CreatedAt       time.Time
}
