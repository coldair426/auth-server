package postgres

import (
	"testing"
	"time"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func ptr[T any](v T) *T { return &v }

func fixedTime() time.Time {
	return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
}

// ---- rowToUser ----

func TestRowToUser(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	now := fixedTime()

	tests := []struct {
		name    string
		row     User
		want    domain.User
		wantErr bool
	}{
		{
			name: "valid",
			row:  User{ID: uuidToPg(id), CreatedAt: timeToPg(now), UpdatedAt: timeToPg(now)},
			want: domain.User{ID: id, CreatedAt: now, UpdatedAt: now},
		},
		{
			name:    "null id",
			row:     User{ID: pgtype.UUID{}, CreatedAt: timeToPg(now), UpdatedAt: timeToPg(now)},
			wantErr: true,
		},
		{
			name:    "null created_at",
			row:     User{ID: uuidToPg(id), CreatedAt: pgtype.Timestamptz{}, UpdatedAt: timeToPg(now)},
			wantErr: true,
		},
		{
			name:    "null updated_at",
			row:     User{ID: uuidToPg(id), CreatedAt: timeToPg(now), UpdatedAt: pgtype.Timestamptz{}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rowToUser(tt.row)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// ---- rowToOAuthAccount ----

func TestRowToOAuthAccount(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	now := fixedTime()
	email := "user@example.com"

	tests := []struct {
		name    string
		row     UserOauthAccount
		want    domain.OAuthAccount
		wantErr bool
	}{
		{
			name: "valid with email",
			row: UserOauthAccount{
				ID:             uuidToPg(id),
				UserID:         uuidToPg(userID),
				Provider:       "google",
				ProviderUserID: "g-12345",
				Email:          pgtype.Text{String: email, Valid: true},
				CreatedAt:      timeToPg(now),
			},
			want: domain.OAuthAccount{
				ID:         id,
				UserID:     userID,
				Provider:   domain.ProviderGoogle,
				ProviderID: "g-12345",
				Email:      &email,
				CreatedAt:  now,
			},
		},
		{
			name: "valid without email",
			row: UserOauthAccount{
				ID:             uuidToPg(id),
				UserID:         uuidToPg(userID),
				Provider:       "kakao",
				ProviderUserID: "k-999",
				Email:          pgtype.Text{},
				CreatedAt:      timeToPg(now),
			},
			want: domain.OAuthAccount{
				ID:         id,
				UserID:     userID,
				Provider:   domain.ProviderKakao,
				ProviderID: "k-999",
				Email:      nil,
				CreatedAt:  now,
			},
		},
		{
			name: "invalid provider",
			row: UserOauthAccount{
				ID:             uuidToPg(id),
				UserID:         uuidToPg(userID),
				Provider:       "twitter",
				ProviderUserID: "t-1",
				CreatedAt:      timeToPg(now),
			},
			wantErr: true,
		},
		{
			name: "null id",
			row: UserOauthAccount{
				ID:             pgtype.UUID{},
				UserID:         uuidToPg(userID),
				Provider:       "naver",
				ProviderUserID: "n-1",
				CreatedAt:      timeToPg(now),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rowToOAuthAccount(tt.row)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr {
				if got.ID != tt.want.ID || got.UserID != tt.want.UserID ||
					got.Provider != tt.want.Provider || got.ProviderID != tt.want.ProviderID {
					t.Errorf("got %+v, want %+v", got, tt.want)
				}
				if (got.Email == nil) != (tt.want.Email == nil) {
					t.Errorf("email nil mismatch: got %v, want %v", got.Email, tt.want.Email)
				}
				if got.Email != nil && *got.Email != *tt.want.Email {
					t.Errorf("email value: got %q, want %q", *got.Email, *tt.want.Email)
				}
			}
		})
	}
}

// ---- rowToOAuthClient ----

func TestRowToOAuthClient(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000004")
	now := fixedTime()
	logo := "https://example.com/logo.png"

	tests := []struct {
		name    string
		row     OauthClient
		wantErr bool
		check   func(t *testing.T, got domain.OAuthClient)
	}{
		{
			name: "full fields",
			row: OauthClient{
				ClientID:            uuidToPg(id),
				Name:                "Test App",
				LogoUrl:             pgtype.Text{String: logo, Valid: true},
				FaviconUrl:          pgtype.Text{},
				GradientFrom:        "#6366f1",
				GradientTo:          "#8b5cf6",
				TextDark:            pgtype.Bool{Bool: true, Valid: true},
				AllowedRedirectUris: []string{"http://localhost:3000/cb"},
				CreatedAt:           timeToPg(now),
				UpdatedAt:           timeToPg(now),
			},
			check: func(t *testing.T, got domain.OAuthClient) {
				if got.ID != id {
					t.Errorf("ID: got %v, want %v", got.ID, id)
				}
				if got.Name != "Test App" {
					t.Errorf("Name: got %q", got.Name)
				}
				if got.LogoURL == nil || *got.LogoURL != logo {
					t.Errorf("LogoURL: got %v", got.LogoURL)
				}
				if got.FaviconURL != nil {
					t.Errorf("FaviconURL: expected nil, got %v", got.FaviconURL)
				}
				if !got.TextDark {
					t.Error("TextDark: expected true")
				}
				if len(got.AllowedRedirectURIs) != 1 || got.AllowedRedirectURIs[0] != "http://localhost:3000/cb" {
					t.Errorf("AllowedRedirectURIs: %v", got.AllowedRedirectURIs)
				}
			},
		},
		{
			name: "nil redirect URIs normalized to empty slice",
			row: OauthClient{
				ClientID:            uuidToPg(id),
				Name:                "App",
				GradientFrom:        "#000",
				GradientTo:          "#fff",
				AllowedRedirectUris: nil,
				CreatedAt:           timeToPg(now),
				UpdatedAt:           timeToPg(now),
			},
			check: func(t *testing.T, got domain.OAuthClient) {
				if got.AllowedRedirectURIs == nil {
					t.Error("AllowedRedirectURIs: expected empty slice, got nil")
				}
				if len(got.AllowedRedirectURIs) != 0 {
					t.Errorf("AllowedRedirectURIs: expected empty, got %v", got.AllowedRedirectURIs)
				}
			},
		},
		{
			name:    "null client_id",
			row:     OauthClient{ClientID: pgtype.UUID{}, Name: "x", CreatedAt: timeToPg(now), UpdatedAt: timeToPg(now)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rowToOAuthClient(tt.row)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// ---- rowToRefreshToken ----

func TestRowToRefreshToken(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000005")
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000006")
	clientID := uuid.MustParse("00000000-0000-0000-0000-000000000007")
	now := fixedTime()
	agent := "Mozilla/5.0"

	tests := []struct {
		name    string
		row     RefreshToken
		wantErr bool
		check   func(t *testing.T, got domain.RefreshToken)
	}{
		{
			name: "not revoked",
			row: RefreshToken{
				ID:        uuidToPg(id),
				UserID:    uuidToPg(userID),
				ClientID:  uuidToPg(clientID),
				TokenHash: "abc123",
				ExpiresAt: timeToPg(now.Add(30 * 24 * time.Hour)),
				RevokedAt: pgtype.Timestamptz{},
				UserAgent: pgtype.Text{String: agent, Valid: true},
				CreatedAt: timeToPg(now),
			},
			check: func(t *testing.T, got domain.RefreshToken) {
				if got.IsRevoked() {
					t.Error("expected not revoked")
				}
				if got.RevokedAt != nil {
					t.Error("RevokedAt: expected nil")
				}
				if got.UserAgent == nil || *got.UserAgent != agent {
					t.Errorf("UserAgent: got %v", got.UserAgent)
				}
				if got.TokenHash != "abc123" {
					t.Errorf("TokenHash: %q", got.TokenHash)
				}
			},
		},
		{
			name: "revoked",
			row: RefreshToken{
				ID:        uuidToPg(id),
				UserID:    uuidToPg(userID),
				ClientID:  uuidToPg(clientID),
				TokenHash: "xyz",
				ExpiresAt: timeToPg(now.Add(time.Hour)),
				RevokedAt: timeToPg(now),
				CreatedAt: timeToPg(now),
			},
			check: func(t *testing.T, got domain.RefreshToken) {
				if !got.IsRevoked() {
					t.Error("expected revoked")
				}
				if got.RevokedAt == nil {
					t.Error("RevokedAt: expected non-nil")
				}
			},
		},
		{
			name: "no user agent",
			row: RefreshToken{
				ID:        uuidToPg(id),
				UserID:    uuidToPg(userID),
				ClientID:  uuidToPg(clientID),
				TokenHash: "tok",
				ExpiresAt: timeToPg(now.Add(time.Hour)),
				UserAgent: pgtype.Text{},
				CreatedAt: timeToPg(now),
			},
			check: func(t *testing.T, got domain.RefreshToken) {
				if got.UserAgent != nil {
					t.Errorf("UserAgent: expected nil, got %v", got.UserAgent)
				}
			},
		},
		{
			name:    "null id",
			row:     RefreshToken{ID: pgtype.UUID{}, UserID: uuidToPg(userID), ClientID: uuidToPg(clientID), ExpiresAt: timeToPg(now), CreatedAt: timeToPg(now)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rowToRefreshToken(tt.row)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// ---- rowToMembership ----

func TestRowToMembership(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000008")
	clientID := uuid.MustParse("00000000-0000-0000-0000-000000000009")
	now := fixedTime()

	tests := []struct {
		name    string
		row     UserProjectMembership
		want    domain.Membership
		wantErr bool
	}{
		{
			name: "valid",
			row:  UserProjectMembership{UserID: uuidToPg(userID), ClientID: uuidToPg(clientID), JoinedAt: timeToPg(now)},
			want: domain.Membership{UserID: userID, ClientID: clientID, JoinedAt: now},
		},
		{
			name:    "null user_id",
			row:     UserProjectMembership{UserID: pgtype.UUID{}, ClientID: uuidToPg(clientID), JoinedAt: timeToPg(now)},
			wantErr: true,
		},
		{
			name:    "null client_id",
			row:     UserProjectMembership{UserID: uuidToPg(userID), ClientID: pgtype.UUID{}, JoinedAt: timeToPg(now)},
			wantErr: true,
		},
		{
			name:    "null joined_at",
			row:     UserProjectMembership{UserID: uuidToPg(userID), ClientID: uuidToPg(clientID), JoinedAt: pgtype.Timestamptz{}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rowToMembership(tt.row)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// ---- rowToConsent ----

func TestRowToConsent(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	userID := uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	now := fixedTime()
	svcID := "client-abc"

	tests := []struct {
		name    string
		row     UserConsent
		wantErr bool
		check   func(t *testing.T, got domain.Consent)
	}{
		{
			name: "TERMS with service_id",
			row: UserConsent{
				ID:          uuidToPg(id),
				UserID:      uuidToPg(userID),
				PolicyType:  "TERMS",
				Version:     "1.0",
				ServiceID:   pgtype.Text{String: svcID, Valid: true},
				ConsentedAt: timeToPg(now),
			},
			check: func(t *testing.T, got domain.Consent) {
				if got.PolicyType != domain.PolicyTerms {
					t.Errorf("PolicyType: %v", got.PolicyType)
				}
				if got.Version != "1.0" {
					t.Errorf("Version: %q", got.Version)
				}
				if got.ServiceID == nil || *got.ServiceID != svcID {
					t.Errorf("ServiceID: %v", got.ServiceID)
				}
			},
		},
		{
			name: "PRIVACY without service_id",
			row: UserConsent{
				ID:          uuidToPg(id),
				UserID:      uuidToPg(userID),
				PolicyType:  "PRIVACY",
				Version:     "2.0",
				ServiceID:   pgtype.Text{},
				ConsentedAt: timeToPg(now),
			},
			check: func(t *testing.T, got domain.Consent) {
				if got.PolicyType != domain.PolicyPrivacy {
					t.Errorf("PolicyType: %v", got.PolicyType)
				}
				if got.ServiceID != nil {
					t.Errorf("ServiceID: expected nil, got %v", got.ServiceID)
				}
			},
		},
		{
			name: "THIRD_PARTY",
			row: UserConsent{
				ID:          uuidToPg(id),
				UserID:      uuidToPg(userID),
				PolicyType:  "THIRD_PARTY",
				Version:     "1.0",
				ConsentedAt: timeToPg(now),
			},
			check: func(t *testing.T, got domain.Consent) {
				if got.PolicyType != domain.PolicyThirdParty {
					t.Errorf("PolicyType: %v", got.PolicyType)
				}
			},
		},
		{
			name: "invalid policy_type",
			row: UserConsent{
				ID:          uuidToPg(id),
				UserID:      uuidToPg(userID),
				PolicyType:  "UNKNOWN",
				Version:     "1.0",
				ConsentedAt: timeToPg(now),
			},
			wantErr: true,
		},
		{
			name:    "null id",
			row:     UserConsent{ID: pgtype.UUID{}, UserID: uuidToPg(userID), PolicyType: "TERMS", Version: "1", ConsentedAt: timeToPg(now)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rowToConsent(tt.row)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// ---- ParseProvider / ParsePolicyType (domain validation constructors) ----

func TestParseProvider(t *testing.T) {
	tests := []struct {
		input   string
		want    domain.Provider
		wantErr bool
	}{
		{"google", domain.ProviderGoogle, false},
		{"kakao", domain.ProviderKakao, false},
		{"naver", domain.ProviderNaver, false},
		{"twitter", "", true},
		{"", "", true},
		{"Google", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := domain.ParseProvider(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePolicyType(t *testing.T) {
	tests := []struct {
		input   string
		want    domain.PolicyType
		wantErr bool
	}{
		{"TERMS", domain.PolicyTerms, false},
		{"PRIVACY", domain.PolicyPrivacy, false},
		{"THIRD_PARTY", domain.PolicyThirdParty, false},
		{"terms", "", true},
		{"COOKIES", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := domain.ParsePolicyType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
