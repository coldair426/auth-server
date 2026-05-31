# auth-server

Google, Kakao, Naver 소셜 로그인을 제공하는 중앙 인증 서버입니다.

OAuth2 기반 인증, JWT 발급, Refresh Token 관리, 회원 가입 및 약관 동의 기능을 제공합니다.

---

## 기술 스택

| 구분 | 기술 |
|------|------|
| Language | Go 1.22+ |
| Router | go-chi/chi v5 |
| Database | PostgreSQL |
| DB Driver | pgx v5 |
| Query Generator | sqlc |
| Migration | golang-migrate |
| Authentication | JWT RS256 |
| OAuth2 | golang.org/x/oauth2 |
| Logging | log/slog |

---

## RSA 키 생성

서버는 JWT 서명에 RSA-2048 키 쌍이 필요합니다.

```bash
mkdir -p keys

# 개인키 생성 (PKCS8 형식)
openssl genpkey -algorithm RSA -out keys/private.pem -pkeyopt rsa_keygen_bits:2048

# 공개키 추출
openssl pkey -in keys/private.pem -pubout -out keys/public.pem
```

생성된 `keys/` 디렉토리는 절대 커밋하지 않습니다 (`.gitignore` 확인).

---

## 환경 변수

`.env.example`을 복사해서 `.env`를 만든 뒤 값을 입력합니다.

```bash
cp .env.example .env
```

| 변수명 | 필수 | 설명 |
|--------|------|------|
| `DATABASE_URL` | ✅ | PostgreSQL 연결 문자열 |
| `JWT_PRIVATE_KEY_PATH` | ✅ | RSA 개인키 파일 경로 |
| `JWT_PUBLIC_KEY_PATH` | ✅ | RSA 공개키 파일 경로 |
| `GOOGLE_CLIENT_ID` | ✅ | Google OAuth 클라이언트 ID |
| `GOOGLE_CLIENT_SECRET` | ✅ | Google OAuth 시크릿 |
| `KAKAO_CLIENT_ID` | ✅ | Kakao OAuth 클라이언트 ID |
| `KAKAO_CLIENT_SECRET` | ✅ | Kakao OAuth 시크릿 |
| `NAVER_CLIENT_ID` | ✅ | Naver OAuth 클라이언트 ID |
| `NAVER_CLIENT_SECRET` | ✅ | Naver OAuth 시크릿 |
| `COOKIE_DOMAIN` | ✅ | 쿠키 도메인 (예: `localhost`) |
| `ALLOWED_ORIGINS` | ✅ | CORS 허용 Origin (쉼표 구분, 예: `http://localhost:3000`) |
| `PORT` | ❌ | 서버 포트 (기본값: `8080`) |

연결 문자열 예시:
```
DATABASE_URL=postgres://auth:secret@localhost:5432/auth_server?sslmode=disable
```

---

## 마이그레이션

[golang-migrate](https://github.com/golang-migrate/migrate) CLI가 필요합니다.

```bash
# 마이그레이션 적용
make migrate-up

# 마이그레이션 롤백 (1단계)
make migrate-down

# 시드 데이터 삽입 (테스트용 OAuth 클라이언트)
make seed
```

`DATABASE_URL` 환경 변수가 설정된 상태에서 실행합니다.

---

## 로컬 실행

### Docker Compose (권장)

```bash
# 키 생성 후
docker compose up --build

# 마이그레이션 (최초 1회)
DATABASE_URL=postgres://auth:secret@localhost:5432/auth_server?sslmode=disable make migrate-up
DATABASE_URL=postgres://auth:secret@localhost:5432/auth_server?sslmode=disable make seed
```

### 직접 실행

```bash
# 1. PostgreSQL 실행 (로컬에 설치된 경우)
# 2. 환경 변수 설정
export $(cat .env | xargs)

# 3. 마이그레이션
make migrate-up

# 4. 서버 실행
make run
```

---

## 개발 명령어

| 명령어 | 설명 |
|--------|------|
| `make run` | 서버 실행 |
| `make build` | 바이너리 빌드 (`bin/server`) |
| `make migrate-up` | 마이그레이션 적용 |
| `make migrate-down` | 마이그레이션 롤백 (1단계) |
| `make seed` | 시드 데이터 삽입 |
| `make sqlc-gen` | sqlc 코드 재생성 |

---

## 테스트

```bash
# 단위 테스트
go test ./...

# 통합 테스트 (Postgres 필요)
TEST_DATABASE_URL="postgres://auth:secret@localhost:5432/auth_test?sslmode=disable" \
  go test -tags=integration ./internal/integration/ -v
```

---

## API Contract

### GET /clients/{clientId}

```json
{
  "clientId": "uuid",
  "name": "앱 이름",
  "logoUrl": null,
  "faviconUrl": null,
  "gradientFrom": "#6366f1",
  "gradientTo": "#8b5cf6",
  "textDark": false
}
```

---

### GET /auth/{provider}/url

Query: `clientId`, `redirectUri`

```json
{
  "url": "https://accounts.google.com/o/oauth2/auth?..."
}
```

---

### POST /auth/{provider}/callback

Request:
```json
{ "code": "...", "state": "..." }
```

Response:
```json
{
  "accessToken": "eyJ...",
  "needsJoin": true,
  "isNewUser": false
}
```

쿠키 설정: `access_token` (15분), `refresh` (30일, HttpOnly, Path=/auth)

---

### POST /auth/join *(인증 필요)*

Request:
```json
{ "clientId": "uuid" }
```

Response: `200 OK`

---

### POST /auth/refresh

쿠키 `refresh` 필요.

Response:
```json
{ "accessToken": "eyJ..." }
```

기존 refresh token은 폐기되고 새 토큰이 발급됩니다 (Rotation).

---

### POST /auth/logout

쿠키 `refresh` 필요.

Response: `200 OK` (쿠키 삭제)

---

### GET /users/{userId}/consents

```json
[
  {
    "id": "uuid",
    "userId": "uuid",
    "policyType": "TERMS",
    "version": "1.0.0",
    "serviceId": null,
    "consentedAt": "2025-01-01T00:00:00Z"
  }
]
```

---

### POST /consents *(인증 필요)*

Request:
```json
{
  "items": [
    { "policyType": "TERMS", "version": "1.0.0" },
    { "policyType": "PRIVACY", "version": "1.0.0" }
  ],
  "serviceId": "optional-service-id"
}
```

Response: `200 OK`

---

## 오류 응답 형식

모든 오류는 동일한 형식으로 반환됩니다.

```json
{
  "message": "오류 설명",
  "code": "ERROR_CODE"
}
```

| HTTP 상태 | code | 사유 |
|-----------|------|------|
| 400 | `INVALID_REQUEST` | 잘못된 입력값, 허용되지 않은 redirectURI |
| 401 | `UNAUTHORIZED` | 토큰 없음·만료·폐기 |
| 404 | `NOT_FOUND` | 클라이언트·사용자 없음 |
| 500 | `INTERNAL_ERROR` | 서버 내부 오류 |

---

## 토큰 정책

### Access Token

- 방식: JWT RS256
- 유효기간: 15분
- `sub` claim: 사용자 UUID

### Refresh Token

- 방식: Opaque Token (random 32 bytes)
- 유효기간: 30일
- 저장: SHA-256 해시값만 DB에 저장
- 전달: HttpOnly 쿠키 (`refresh`, Path=/auth)

### Refresh Rotation

Refresh 요청 시 기존 토큰은 즉시 폐기되고 새 토큰이 발급됩니다.  
폐기된 토큰을 재사용하면 401이 반환됩니다.

---

## 보안 정책

- Redirect URI: 클라이언트별 허용 목록에 등록된 URI만 허용
- OAuth State: 서버에서 생성, TTL 10분, 1회 사용 후 삭제
- 민감정보 로그 금지: Access Token, Refresh Token, OAuth Code, Secret

---

## Frontend 변경 필요 사항

### 1. ID 타입 변경: `number` → `string`

기존 Mock API는 숫자 ID를 사용했지만, 이 서버는 UUID(string)를 사용합니다.

```typescript
// 변경 전 (Mock API)
id: number
userId: number

// 변경 후 (auth-server)
id: string       // UUID v7
userId: string   // UUID v7
```

`/users/{userId}/consents`, `/consents` 응답의 `id`, `userId` 필드 모두 해당합니다.

---

### 2. 회원 가입 시 `POST /consents` 반드시 호출

`POST /auth/{provider}/callback` 응답에서 `needsJoin: true`인 경우, 가입 플로우에서 반드시 아래 순서로 호출해야 합니다.

```
1. POST /consents    ← 약관 동의 먼저 기록
2. POST /auth/join   ← 그 다음 멤버십 가입
```

`POST /consents` 없이 `POST /auth/join`만 호출하면 약관 동의 데이터가 누락됩니다.

---

## 프로젝트 구조

```
auth-server/
├── cmd/server/main.go          # 진입점 + 의존성 조립
├── internal/
│   ├── auth/                   # 인증 서비스 + 핸들러
│   ├── client/                 # 클라이언트 서비스 + 핸들러
│   ├── consent/                # 동의 서비스 + 핸들러
│   ├── domain/                 # 도메인 엔티티 + 오류
│   ├── integration/            # 통합 테스트 (build tag: integration)
│   └── platform/
│       ├── cache/              # 인메모리 캐시
│       ├── config/             # 환경 변수 로드
│       ├── httpx/              # HTTP 헬퍼 + 미들웨어
│       ├── jwt/                # JWT 발급/검증
│       ├── oauth/              # OAuth2 제공자
│       └── postgres/           # DB 레포지토리
├── db/
│   ├── migrations/             # golang-migrate SQL 파일
│   ├── queries/                # sqlc SQL 쿼리
│   └── seed.sql                # 로컬 개발용 시드
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

개발 규칙은 `AI_RULES.md`를 참고합니다.
