package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OAuthClientRepo satisfies both auth.OAuthClientRepository and client.ClientRepository
// (both interfaces declare the identical FindByID signature).
type OAuthClientRepo struct{ q *Queries }

func NewOAuthClientRepo(q *Queries) *OAuthClientRepo { return &OAuthClientRepo{q: q} }

func (r *OAuthClientRepo) FindByID(ctx context.Context, id uuid.UUID) (domain.OAuthClient, error) {
	row, err := r.q.GetOAuthClientByID(ctx, uuidToPg(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OAuthClient{}, domain.ErrClientNotFound
		}
		return domain.OAuthClient{}, fmt.Errorf("postgres.OAuthClientRepo.FindByID: %w", err)
	}
	return rowToOAuthClient(row)
}
