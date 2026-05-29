package domain

import (
	"time"

	"github.com/google/uuid"
)

type Provider string

const (
	ProviderGoogle Provider = "google"
	ProviderKakao  Provider = "kakao"
	ProviderNaver  Provider = "naver"
)

type OAuthAccount struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Provider   Provider
	ProviderID string
	CreatedAt  time.Time
}
