package domain

import "github.com/google/uuid"

type OAuthClient struct {
	ID                  uuid.UUID
	Name                string
	LogoURL             *string
	FaviconURL          *string
	GradientFrom        string
	GradientTo          string
	TextDark            bool
	AllowedRedirectURIs []string
}

func (c OAuthClient) ValidateRedirectURI(uri string) error {
	for _, allowed := range c.AllowedRedirectURIs {
		if allowed == uri {
			return nil
		}
	}
	return ErrInvalidRedirectURI
}
