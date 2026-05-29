package postgres

import (
	"fmt"
	"time"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// --- primitive converters ---

func uuidToPg(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func uuidFromPg(p pgtype.UUID) (uuid.UUID, error) {
	if !p.Valid {
		return uuid.UUID{}, fmt.Errorf("null UUID")
	}
	return uuid.UUID(p.Bytes), nil
}

func timeToPg(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func timeFromPg(p pgtype.Timestamptz) (time.Time, error) {
	if !p.Valid {
		return time.Time{}, fmt.Errorf("null timestamp")
	}
	return p.Time, nil
}

func textPtrToPg(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func textPtrFromPg(p pgtype.Text) *string {
	if !p.Valid {
		return nil
	}
	s := p.String
	return &s
}

func boolFromPg(p pgtype.Bool) bool {
	return p.Bool
}

// --- row -> domain mappers ---

func rowToUser(row User) (domain.User, error) {
	id, err := uuidFromPg(row.ID)
	if err != nil {
		return domain.User{}, fmt.Errorf("user.id: %w", err)
	}
	createdAt, err := timeFromPg(row.CreatedAt)
	if err != nil {
		return domain.User{}, fmt.Errorf("user.created_at: %w", err)
	}
	updatedAt, err := timeFromPg(row.UpdatedAt)
	if err != nil {
		return domain.User{}, fmt.Errorf("user.updated_at: %w", err)
	}
	return domain.User{
		ID:        id,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func rowToOAuthAccount(row UserOauthAccount) (domain.OAuthAccount, error) {
	id, err := uuidFromPg(row.ID)
	if err != nil {
		return domain.OAuthAccount{}, fmt.Errorf("oauth_account.id: %w", err)
	}
	userID, err := uuidFromPg(row.UserID)
	if err != nil {
		return domain.OAuthAccount{}, fmt.Errorf("oauth_account.user_id: %w", err)
	}
	provider, err := domain.ParseProvider(row.Provider)
	if err != nil {
		return domain.OAuthAccount{}, fmt.Errorf("oauth_account.provider: %w", err)
	}
	createdAt, err := timeFromPg(row.CreatedAt)
	if err != nil {
		return domain.OAuthAccount{}, fmt.Errorf("oauth_account.created_at: %w", err)
	}
	return domain.OAuthAccount{
		ID:         id,
		UserID:     userID,
		Provider:   provider,
		ProviderID: row.ProviderUserID,
		Email:      textPtrFromPg(row.Email),
		CreatedAt:  createdAt,
	}, nil
}

func rowToOAuthClient(row OauthClient) (domain.OAuthClient, error) {
	id, err := uuidFromPg(row.ClientID)
	if err != nil {
		return domain.OAuthClient{}, fmt.Errorf("oauth_client.client_id: %w", err)
	}
	uris := row.AllowedRedirectUris
	if uris == nil {
		uris = []string{}
	}
	return domain.OAuthClient{
		ID:                  id,
		Name:                row.Name,
		LogoURL:             textPtrFromPg(row.LogoUrl),
		FaviconURL:          textPtrFromPg(row.FaviconUrl),
		GradientFrom:        row.GradientFrom,
		GradientTo:          row.GradientTo,
		TextDark:            boolFromPg(row.TextDark),
		AllowedRedirectURIs: uris,
	}, nil
}

func rowToRefreshToken(row RefreshToken) (domain.RefreshToken, error) {
	id, err := uuidFromPg(row.ID)
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("refresh_token.id: %w", err)
	}
	userID, err := uuidFromPg(row.UserID)
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("refresh_token.user_id: %w", err)
	}
	clientID, err := uuidFromPg(row.ClientID)
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("refresh_token.client_id: %w", err)
	}
	expiresAt, err := timeFromPg(row.ExpiresAt)
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("refresh_token.expires_at: %w", err)
	}
	createdAt, err := timeFromPg(row.CreatedAt)
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("refresh_token.created_at: %w", err)
	}
	var revokedAt *time.Time
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		revokedAt = &t
	}
	return domain.RefreshToken{
		ID:        id,
		UserID:    userID,
		ClientID:  clientID,
		TokenHash: row.TokenHash,
		ExpiresAt: expiresAt,
		RevokedAt: revokedAt,
		UserAgent: textPtrFromPg(row.UserAgent),
		CreatedAt: createdAt,
	}, nil
}

func rowToMembership(row UserProjectMembership) (domain.Membership, error) {
	userID, err := uuidFromPg(row.UserID)
	if err != nil {
		return domain.Membership{}, fmt.Errorf("membership.user_id: %w", err)
	}
	clientID, err := uuidFromPg(row.ClientID)
	if err != nil {
		return domain.Membership{}, fmt.Errorf("membership.client_id: %w", err)
	}
	joinedAt, err := timeFromPg(row.JoinedAt)
	if err != nil {
		return domain.Membership{}, fmt.Errorf("membership.joined_at: %w", err)
	}
	return domain.Membership{
		UserID:   userID,
		ClientID: clientID,
		JoinedAt: joinedAt,
	}, nil
}

func rowToConsent(row UserConsent) (domain.Consent, error) {
	id, err := uuidFromPg(row.ID)
	if err != nil {
		return domain.Consent{}, fmt.Errorf("consent.id: %w", err)
	}
	userID, err := uuidFromPg(row.UserID)
	if err != nil {
		return domain.Consent{}, fmt.Errorf("consent.user_id: %w", err)
	}
	policyType, err := domain.ParsePolicyType(row.PolicyType)
	if err != nil {
		return domain.Consent{}, fmt.Errorf("consent.policy_type: %w", err)
	}
	consentedAt, err := timeFromPg(row.ConsentedAt)
	if err != nil {
		return domain.Consent{}, fmt.Errorf("consent.consented_at: %w", err)
	}
	return domain.Consent{
		ID:          id,
		UserID:      userID,
		PolicyType:  policyType,
		Version:     row.Version,
		ServiceID:   textPtrFromPg(row.ServiceID),
		ConsentedAt: consentedAt,
	}, nil
}
