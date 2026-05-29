package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/jackc/pgx/v5"
)

type OAuthAccountRepo struct{ q *Queries }

func NewOAuthAccountRepo(q *Queries) *OAuthAccountRepo { return &OAuthAccountRepo{q: q} }

func (r *OAuthAccountRepo) Upsert(ctx context.Context, a domain.OAuthAccount) (domain.OAuthAccount, error) {
	row, err := r.q.UpsertOAuthAccount(ctx, UpsertOAuthAccountParams{
		ID:             uuidToPg(a.ID),
		UserID:         uuidToPg(a.UserID),
		Provider:       string(a.Provider),
		ProviderUserID: a.ProviderID,
		Email:          textPtrToPg(a.Email),
	})
	if err != nil {
		return domain.OAuthAccount{}, fmt.Errorf("postgres.OAuthAccountRepo.Upsert: %w", err)
	}
	return rowToOAuthAccount(row)
}

func (r *OAuthAccountRepo) FindByProvider(ctx context.Context, provider domain.Provider, providerID string) (domain.OAuthAccount, error) {
	row, err := r.q.GetOAuthAccountByProvider(ctx, GetOAuthAccountByProviderParams{
		Provider:       string(provider),
		ProviderUserID: providerID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OAuthAccount{}, domain.ErrOAuthAccountNotFound
		}
		return domain.OAuthAccount{}, fmt.Errorf("postgres.OAuthAccountRepo.FindByProvider: %w", err)
	}
	return rowToOAuthAccount(row)
}
