package domain

import "errors"

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrOAuthAccountNotFound = errors.New("oauth account not found")
	ErrClientNotFound       = errors.New("oauth client not found")
	ErrInvalidRedirectURI   = errors.New("redirect URI not in allowlist")
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenRevoked  = errors.New("refresh token revoked")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
	ErrMembershipNotFound   = errors.New("membership not found")
	ErrConsentNotFound      = errors.New("consent not found")
	ErrInvalidProvider      = errors.New("invalid provider")
	ErrInvalidPolicyType    = errors.New("invalid policy type")
)
