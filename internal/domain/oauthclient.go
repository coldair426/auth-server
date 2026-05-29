package domain

import "github.com/google/uuid"

type OAuthClient struct {
	ID                   uuid.UUID
	Name                 string
	LogoURL              *string
	FaviconURL           *string
	GradientFrom         string
	GradientTo           string
	TextDark             bool
	AllowedRedirectURIs  []string
}
