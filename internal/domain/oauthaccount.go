package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Provider string

const (
	ProviderGoogle Provider = "google"
	ProviderKakao  Provider = "kakao"
	ProviderNaver  Provider = "naver"
)

func ParseProvider(s string) (Provider, error) {
	switch p := Provider(s); p {
	case ProviderGoogle, ProviderKakao, ProviderNaver:
		return p, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidProvider, s)
	}
}

type OAuthAccount struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Provider   Provider
	ProviderID string
	Email      *string
	CreatedAt  time.Time
}
