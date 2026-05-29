package domain

import (
	"time"

	"github.com/google/uuid"
)

type Membership struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ClientID  uuid.UUID
	CreatedAt time.Time
}
