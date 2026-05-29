package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserRepo struct{ q *Queries }

func NewUserRepo(q *Queries) *UserRepo { return &UserRepo{q: q} }

func (r *UserRepo) Create(ctx context.Context, u domain.User) (domain.User, error) {
	row, err := r.q.CreateUser(ctx, uuidToPg(u.ID))
	if err != nil {
		return domain.User{}, fmt.Errorf("postgres.UserRepo.Create: %w", err)
	}
	return rowToUser(row)
}

func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q.GetUserByID(ctx, uuidToPg(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("postgres.UserRepo.FindByID: %w", err)
	}
	return rowToUser(row)
}
