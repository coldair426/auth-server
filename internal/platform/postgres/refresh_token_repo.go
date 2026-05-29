package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/jackc/pgx/v5"
)

type RefreshTokenRepo struct{ q *Queries }

func NewRefreshTokenRepo(q *Queries) *RefreshTokenRepo { return &RefreshTokenRepo{q: q} }

func (r *RefreshTokenRepo) Create(ctx context.Context, t domain.RefreshToken) (domain.RefreshToken, error) {
	row, err := r.q.CreateRefreshToken(ctx, CreateRefreshTokenParams{
		ID:        uuidToPg(t.ID),
		UserID:    uuidToPg(t.UserID),
		ClientID:  uuidToPg(t.ClientID),
		TokenHash: t.TokenHash,
		ExpiresAt: timeToPg(t.ExpiresAt),
		UserAgent: textPtrToPg(t.UserAgent),
	})
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("postgres.RefreshTokenRepo.Create: %w", err)
	}
	return rowToRefreshToken(row)
}

func (r *RefreshTokenRepo) FindByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	row, err := r.q.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RefreshToken{}, domain.ErrRefreshTokenNotFound
		}
		return domain.RefreshToken{}, fmt.Errorf("postgres.RefreshTokenRepo.FindByHash: %w", err)
	}
	return rowToRefreshToken(row)
}

func (r *RefreshTokenRepo) RevokeByHash(ctx context.Context, hash string) error {
	if err := r.q.RevokeRefreshToken(ctx, hash); err != nil {
		return fmt.Errorf("postgres.RefreshTokenRepo.RevokeByHash: %w", err)
	}
	return nil
}
