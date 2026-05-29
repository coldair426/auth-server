# auth-server

Centralized social-login authentication server (Google / Kakao / Naver).

## Quick start

```bash
cp .env.example .env   # fill in required vars
make run
```

## Required environment variables

| Variable | Description |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_PRIVATE_KEY_PATH` | Path to RS256 private key (PEM) |
| `JWT_PUBLIC_KEY_PATH` | Path to RS256 public key (PEM) |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth2 app credentials |
| `KAKAO_CLIENT_ID` / `KAKAO_CLIENT_SECRET` | Kakao OAuth2 app credentials |
| `NAVER_CLIENT_ID` / `NAVER_CLIENT_SECRET` | Naver OAuth2 app credentials |
| `COOKIE_DOMAIN` | Domain for Set-Cookie headers |
| `ALLOWED_ORIGINS` | Comma-separated CORS allowed origins |
| `PORT` | HTTP listen port (default `8080`) |

## Makefile targets

| Target | Description |
|---|---|
| `make run` | `go run ./cmd/server` |
| `make build` | Compile binary to `bin/server` |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back one migration |
| `make sqlc-gen` | Regenerate sqlc code from `db/queries/` |

## Frontend changes required

The following changes are **required** in `auth-ui` before it can integrate with this backend:

1. **UUID instead of number for user/consent IDs** — The mock API typed `userId` and consent `id` as `number`.
   This backend uses UUID strings (`string`). Update all TypeScript interfaces and API call sites accordingly.

2. **POST /consents on join** — The frontend join flow does not currently call `POST /consents`.
   Consent recording must be wired in before membership creation is considered complete.
