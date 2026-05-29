package domain

import (
	"time"

	"github.com/google/uuid"
)

type Membership struct {
	UserID   uuid.UUID
	ClientID uuid.UUID
	JoinedAt time.Time
}
