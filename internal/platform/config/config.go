package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL        string
	JWTPrivateKeyPath  string
	JWTPublicKeyPath   string
	GoogleClientID     string
	GoogleClientSecret string
	KakaoClientID      string
	KakaoClientSecret  string
	NaverClientID      string
	NaverClientSecret  string
	CookieDomain       string
	AllowedOrigins     []string
	Port               string
}

func Load() (*Config, error) {
	c := &Config{
		Port: getEnvOrDefault("PORT", "8080"),
	}

	required := []struct {
		dst  *string
		key  string
	}{
		{&c.DatabaseURL, "DATABASE_URL"},
		{&c.JWTPrivateKeyPath, "JWT_PRIVATE_KEY_PATH"},
		{&c.JWTPublicKeyPath, "JWT_PUBLIC_KEY_PATH"},
		{&c.GoogleClientID, "GOOGLE_CLIENT_ID"},
		{&c.GoogleClientSecret, "GOOGLE_CLIENT_SECRET"},
		{&c.KakaoClientID, "KAKAO_CLIENT_ID"},
		{&c.KakaoClientSecret, "KAKAO_CLIENT_SECRET"},
		{&c.NaverClientID, "NAVER_CLIENT_ID"},
		{&c.NaverClientSecret, "NAVER_CLIENT_SECRET"},
		{&c.CookieDomain, "COOKIE_DOMAIN"},
	}

	var missing []string
	for _, r := range required {
		v := os.Getenv(r.key)
		if v == "" {
			missing = append(missing, r.key)
			continue
		}
		*r.dst = v
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		return nil, errors.New("missing required env var: ALLOWED_ORIGINS")
	}
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, o)
		}
	}

	return c, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
