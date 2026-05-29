package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrTokenNotFound = errors.New("refresh token not found")
var ErrTokenRevoked = errors.New("refresh token revoked")
var ErrTokenExpired = errors.New("refresh token expired")

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}
