package domain

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ClientID  uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	UserAgent *string
	CreatedAt time.Time
}

func (t RefreshToken) IsRevoked() bool { return t.RevokedAt != nil }
func (t RefreshToken) IsExpired() bool { return time.Now().After(t.ExpiresAt) }
