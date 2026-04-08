package evening

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("evening not found")

type Evening struct {
	ID               int
	GroupID          int
	Date             time.Time
	Topic            *string
	Notes            *string
	ParticipantCount *int
	CreatedAt        time.Time
}
