# Project: auth-server

A centralized social-login authentication server. Frontend (`auth-ui`, Next.js) is
already built against mock APIs; this backend replaces those mocks 1:1. Treat the
frontend contract below as the SINGLE SOURCE OF TRUTH — do not invent endpoints,
fields, or flows that the frontend does not use.

## Tech stack (LOCKED — do not substitute)
- Language: Go 1.22+
- Router: go-chi/chi v5 (stdlib net/http compatible)
- DB driver: jackc/pgx v5
- Query codegen: sqlc (write SQL, generate type-safe Go)
- Migrations: golang-migrate (.sql files)
- JWT: golang-jwt/jwt v5, algorithm RS256
- UUID: google/uuid (use uuid.NewV7() for all primary keys)
- OAuth2: golang.org/x/oauth2 (custom Endpoint for kakao/naver)
- Config: environment variables (no config files)
- Logging: log/slog (stdlib structured logging)

## Architecture: Go-idiomatic clean architecture
Principle, NOT Java-style 4-layer folders. Rules:
- Dependency direction points INWARD only: handler -> service -> domain.
- `domain` has ZERO external dependencies (no chi, no pgx, no sql).
- Repository INTERFACES are declared in the feature package that consumes them
  (the service side). Concrete implementations live in `platform/postgres`.
- Accept interfaces, return structs. Do not pre-abstract.
- Package = cohesive responsibility (feature-based), not technical layer.

## Directory layout (create exactly this)
```
auth-server/
├── cmd/server/main.go          # entrypoint; dependency wiring (composition root)
├── internal/
│   ├── domain/                 # pure entities + rules, zero deps
│   │   ├── user.go
│   │   ├── oauthaccount.go
│   │   ├── oauthclient.go
│   │   ├── refreshtoken.go
│   │   ├── membership.go
│   │   └── consent.go
│   ├── auth/                   # login/callback/refresh/logout/join feature
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go       # interfaces only (declared by service)
│   ├── client/                 # GET /clients/{clientId}
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   ├── consent/                # consent feature
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── repository.go
│   └── platform/               # cross-cutting / outward adapters
│       ├── postgres/           # sqlc-generated code + repo implementations
│       ├── cache/              # in-memory caches
│       ├── jwt/                # RS256 issue/verify
│       ├── oauth/              # provider integrations (google/kakao/naver)
│       ├── httpx/              # response helpers, error envelope, middleware
│       └── config/             # env loading
├── db/
│   ├── migrations/             # golang-migrate .sql files
│   └── queries/                # sqlc .sql source
├── sqlc.yaml
├── go.mod
└── README.md
```

## Frontend API contract (replace mocks 1:1 — DO NOT change shapes)

1. GET /clients/{clientId}
   -> 200 { clientId, name, logoUrl (string|null), faviconUrl (string|null),
   gradientFrom (hex), gradientTo (hex), textDark (bool, optional) }

2. GET /auth/{provider}/url?clientId=X&redirectUri=Y   provider in {naver,kakao,google}
   -> 200 { url }            # provider authorization URL (server generates+stores state)

3. POST /auth/{provider}/callback   body { code, state }
   -> 200 { accessToken, needsJoin (bool), isNewUser (bool) }
   # Also sets cookies: access_token (readable by FE middleware) + refresh token (HttpOnly)

4. POST /auth/join   body { clientId }      (auth required via access token)
   -> 200 (empty body)      # creates membership(user, client)

5. POST /auth/refresh   body {}   (refresh token from HttpOnly cookie, withCredentials)
   -> 200 { accessToken }   # rotates refresh token, resets access_token cookie

6. POST /auth/logout   body {}   (refresh cookie)
   -> 200 (empty)           # revokes refresh token, clears cookies

7. GET /users/{userId}/consents
   -> 200 [ { id, userId, policyType, version, serviceId (string|null), consentedAt (ISO) } ]

8. POST /consents   body { consents: [{ policyType, version }], serviceId? }  (auth required)
   -> 200 (empty)

Enums: provider = naver|kakao|google ; policyType = TERMS|PRIVACY|THIRD_PARTY
Error envelope: { "message": string, "code": string }

## Token rules
- Access JWT: RS256, TTL 15 min. Claims MUST include `sub` = user UUID (string).
  FE middleware decodes (no signature check) and reads `sub`/`userId`. Put UUID in `sub`.
- Refresh token: opaque random (NOT a JWT), TTL 30 days, stored HASHED in PostgreSQL.
  Sent as HttpOnly cookie: `HttpOnly; Secure; SameSite=None; Path=/auth`.
- access_token cookie: set so FE middleware can read it. Same JWT value.
- Rotation: every /auth/refresh issues a NEW refresh token and revokes the old one.

## Storage strategy
- PostgreSQL = source of truth for EVERYTHING (users, clients, tokens, consents).
- In-memory cache layer:
    - oauth_clients lookup: read-through cache (rarely changes). Invalidate on write.
    - refresh-token validation: ALWAYS verified against PostgreSQL (security first).
      Cache may accelerate reads but a revoke MUST invalidate cache immediately.
- Cache must survive being empty (cold start) by falling back to PostgreSQL.

## Security requirements (enforce all)
- redirectUri allowlist: validate the requested redirectUri against
  oauth_clients.allowed_redirect_uris for that clientId. Reject if not listed.
- OAuth `state`: generated server-side at /auth/{provider}/url, stored with short TTL,
  verified+consumed at callback (CSRF protection).
- Refresh tokens stored only as a hash (e.g. SHA-256); never store raw tokens.
- Never log tokens, codes, or secrets.
- Provider app keys (client id/secret for google/kakao/naver) come from env only.

## Frontend alignment notes (flag these, do not silently diverge)
- FE consent mock typed userId/id as `number`; backend uses UUID. The FE must change
  these to `string`. Document this in README under "Frontend changes required".
- Consent on join is currently NOT wired in FE. Backend decision: expose POST /consents
  as the consent recording endpoint (per contract). FE must call it during join.

## Go conventions for this repo
- Errors: wrap with fmt.Errorf("...: %w", err); define sentinel errors in domain.
- Validation at boundaries: validate inputs in handler/service entry, fail fast.
- No global state except composition in main.go.
- Context: pass context.Context as first arg through service/repo calls.
- Tests: table-driven; service layer testable with mocked repository interfaces.
