package postgres

import (
	"context"
	"fmt"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
)

type ConsentRepo struct{ q *Queries }

func NewConsentRepo(q *Queries) *ConsentRepo { return &ConsentRepo{q: q} }

func (r *ConsentRepo) Insert(ctx context.Context, c domain.Consent) error {
	if err := r.q.InsertConsent(ctx, InsertConsentParams{
		ID:         uuidToPg(c.ID),
		UserID:     uuidToPg(c.UserID),
		PolicyType: string(c.PolicyType),
		Version:    c.Version,
		ServiceID:  textPtrToPg(c.ServiceID),
	}); err != nil {
		return fmt.Errorf("postgres.ConsentRepo.Insert: %w", err)
	}
	return nil
}

func (r *ConsentRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Consent, error) {
	rows, err := r.q.ListConsentsByUserID(ctx, uuidToPg(userID))
	if err != nil {
		return nil, fmt.Errorf("postgres.ConsentRepo.ListByUserID: %w", err)
	}
	out := make([]domain.Consent, 0, len(rows))
	for _, row := range rows {
		c, err := rowToConsent(row)
		if err != nil {
			return nil, fmt.Errorf("postgres.ConsentRepo.ListByUserID: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}
