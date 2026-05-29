package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MembershipRepo struct{ q *Queries }

func NewMembershipRepo(q *Queries) *MembershipRepo { return &MembershipRepo{q: q} }

func (r *MembershipRepo) FindByUserAndClient(ctx context.Context, userID, clientID uuid.UUID) (domain.Membership, error) {
	row, err := r.q.GetMembership(ctx, GetMembershipParams{
		UserID:   uuidToPg(userID),
		ClientID: uuidToPg(clientID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Membership{}, domain.ErrMembershipNotFound
		}
		return domain.Membership{}, fmt.Errorf("postgres.MembershipRepo.FindByUserAndClient: %w", err)
	}
	return rowToMembership(row)
}

func (r *MembershipRepo) Create(ctx context.Context, m domain.Membership) error {
	if err := r.q.CreateMembership(ctx, CreateMembershipParams{
		UserID:   uuidToPg(m.UserID),
		ClientID: uuidToPg(m.ClientID),
	}); err != nil {
		return fmt.Errorf("postgres.MembershipRepo.Create: %w", err)
	}
	return nil
}
