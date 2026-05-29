package auth

import (
	"context"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, u domain.User) (domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

type OAuthAccountRepository interface {
	Upsert(ctx context.Context, a domain.OAuthAccount) (domain.OAuthAccount, error)
	FindByProvider(ctx context.Context, provider domain.Provider, providerID string) (domain.OAuthAccount, error)
}

type OAuthClientRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (domain.OAuthClient, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t domain.RefreshToken) (domain.RefreshToken, error)
	FindByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
	RevokeByHash(ctx context.Context, hash string) error
}

type MembershipRepository interface {
	FindByUserAndClient(ctx context.Context, userID, clientID uuid.UUID) (domain.Membership, error)
	Create(ctx context.Context, m domain.Membership) error
}
