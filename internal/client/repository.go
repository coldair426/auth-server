package client

import (
	"context"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
)

type ClientRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (domain.OAuthClient, error)
}
