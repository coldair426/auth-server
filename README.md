# README.md

# auth-server

Google, Kakao, Naver 소셜 로그인을 제공하는 중앙 인증 서버입니다.

프론트엔드(`auth-ui`)에서 사용하던 Mock API를 실제 서비스 API로 대체하며, OAuth2 기반 인증과 사용자 가입 기능을 제공합니다.

---

# 주요 기능

- Google 로그인
- Kakao 로그인
- Naver 로그인
- JWT Access Token 발급
- Refresh Token 관리
- 회원(Client) 가입
- 약관 동의 관리

---

# 기술 스택

| 구분 | 기술 |
|--------|--------|
| Language | Go 1.22+ |
| Router | go-chi/chi v5 |
| Database | PostgreSQL |
| DB Driver | pgx v5 |
| Query Generator | sqlc |
| Migration | golang-migrate |
| Authentication | JWT (RS256) |
| OAuth2 | golang.org/x/oauth2 |
| Logging | slog |

---

# 프로젝트 구조

```text
auth-server/
├── cmd/server/main.go
├── internal/
│   ├── auth/
│   ├── client/
│   ├── consent/
│   ├── domain/
│   └── platform/
├── db/
│   ├── migrations/
│   └── queries/
├── sqlc.yaml
├── README.md
└── AI_RULES.md
```

---

# 시작하기

## 환경 변수 설정

```bash
cp .env.example .env
```

필수 환경 변수 입력 후 실행한다.

---

## 실행

```bash
make run
```

또는

```bash
go run ./cmd/server
```

---

# 환경 변수

| 변수명 | 설명 |
|----------|----------|
| DATABASE_URL | PostgreSQL 연결 정보 |
| JWT_PRIVATE_KEY_PATH | JWT 개인키 경로 |
| JWT_PUBLIC_KEY_PATH | JWT 공개키 경로 |
| GOOGLE_CLIENT_ID | Google OAuth Client ID |
| GOOGLE_CLIENT_SECRET | Google OAuth Secret |
| KAKAO_CLIENT_ID | Kakao OAuth Client ID |
| KAKAO_CLIENT_SECRET | Kakao OAuth Secret |
| NAVER_CLIENT_ID | Naver OAuth Client ID |
| NAVER_CLIENT_SECRET | Naver OAuth Secret |
| COOKIE_DOMAIN | 쿠키 도메인 |
| ALLOWED_ORIGINS | CORS 허용 Origin |
| PORT | 서버 포트 |

---

# 개발 명령어

| 명령어 | 설명 |
|----------|----------|
| make run | 서버 실행 |
| make build | 바이너리 빌드 |
| make migrate-up | 마이그레이션 적용 |
| make migrate-down | 마이그레이션 롤백 |
| make sqlc-gen | sqlc 코드 생성 |

---

# API Contract

## GET /clients/{clientId}

```json
{
  "clientId": "",
  "name": "",
  "logoUrl": null,
  "faviconUrl": null,
  "gradientFrom": "",
  "gradientTo": "",
  "textDark": true
}
```

---

## GET /auth/{provider}/url

```json
{
  "url": ""
}
```

---

## POST /auth/{provider}/callback

```json
{
  "accessToken": "",
  "needsJoin": true,
  "isNewUser": false
}
```

---

## POST /auth/join

```http
200 OK
```

---

## POST /auth/refresh

```json
{
  "accessToken": ""
}
```

---

## POST /auth/logout

```http
200 OK
```

---

## GET /users/{userId}/consents

```json
[
  {
    "id": "",
    "userId": "",
    "policyType": "TERMS",
    "version": "1.0.0",
    "serviceId": null,
    "consentedAt": "2025-01-01T00:00:00Z"
  }
]
```

---

## POST /consents

```http
200 OK
```

---

# 토큰 정책

## Access Token

- JWT RS256
- TTL 15분
- `sub` Claim에 사용자 UUID 저장

## Refresh Token

- JWT 사용 안 함
- Random Opaque Token 사용
- TTL 30일
- PostgreSQL Hash 저장
- HttpOnly Cookie 사용

## Refresh Rotation

Refresh 요청 시

- 신규 Refresh Token 발급
- 기존 Refresh Token 폐기

---

# 보안 정책

## Redirect URI 검증

등록된 Redirect URI만 허용

## OAuth State 검증

- 서버 생성
- TTL 저장
- Callback 검증
- 사용 후 삭제

## 민감정보 로그 금지

- Access Token
- Refresh Token
- OAuth Code
- OAuth Secret

---

# Frontend 연동 시 필수 변경 사항

## UUID 타입 변경

기존 Mock API

```typescript
id: number
userId: number
```

변경 후

```typescript
id: string
userId: string
```

---

## 약관 동의 API 호출

회원가입 완료 전 반드시 호출

```http
POST /consents
```

---

# 개발 규칙

AI 기반 코드 생성 및 프로젝트 개발 규칙은 `AI_RULES.md`를 참고한다.