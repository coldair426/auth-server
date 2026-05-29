package consent

import (
	"context"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
)

type ConsentRepository interface {
	Insert(ctx context.Context, c domain.Consent) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Consent, error)
}
