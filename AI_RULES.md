# AI Rules

## 목적

이 문서는 AI(Codex, Claude Code, Cursor, Copilot 등)가 본 프로젝트에서 코드를 생성하거나 수정할 때 반드시 따라야 하는 규칙을 정의한다.

README.md는 프로젝트 설명서이며, 본 문서는 개발 규칙서이다.

---

# 언어 및 출력 규칙

## 기본 언어

- 모든 설명은 한국어로 작성한다.
- 모든 문서는 한국어로 작성한다.
- 코드 주석은 한국어로 작성한다.

단, 아래 항목은 영어를 유지한다.

- 패키지명
- 변수명
- 함수명
- 타입명
- 인터페이스명
- 데이터베이스 컬럼명

---

## Git Commit

모든 Commit 메시지는 한국어로 작성한다.

예시

```text
feat: 카카오 로그인 기능 추가
fix: JWT 검증 오류 수정
refactor: 인증 서비스 책임 분리
docs: README 개선
```

---

## Pull Request

- 제목은 한국어로 작성한다.
- 본문은 한국어로 작성한다.

---

# 기술 스택

다음 기술 스택은 변경하지 않는다.

| 영역 | 기술 |
|--------|--------|
| Language | Go 1.22+ |
| Router | go-chi/chi v5 |
| Database | PostgreSQL |
| DB Driver | jackc/pgx v5 |
| Query Generator | sqlc |
| Migration | golang-migrate |
| JWT | golang-jwt/jwt v5 (RS256) |
| UUID | google/uuid |
| OAuth2 | golang.org/x/oauth2 |
| Config | Environment Variables |
| Logging | log/slog |

---

# 아키텍처 규칙

## 의존성 방향

```text
handler
    ↓
service
    ↓
domain
```

---

## Domain

Domain은 순수 비즈니스 규칙만 담당한다.

허용

- Entity
- Value Object
- Domain Rule
- Domain Error

금지

- HTTP 의존성
- SQL 의존성
- OAuth 의존성
- 외부 라이브러리 의존성

---

## Handler

Handler는 HTTP 계층만 담당한다.

허용

- Request 파싱
- DTO 검증
- Service 호출
- Response 생성

금지

- SQL 실행
- 비즈니스 로직 구현
- 트랜잭션 처리

---

## Service

Service는 비즈니스 로직을 담당한다.

허용

- 도메인 규칙 검증
- Repository 호출
- OAuth Provider 호출
- 트랜잭션 경계 설정

금지

- HTTP 응답 생성
- SQL 작성

---

## Repository

Repository는 데이터 저장소 접근만 담당한다.

허용

- 데이터 조회
- 데이터 저장
- 데이터 수정
- 데이터 삭제

금지

- 비즈니스 로직
- HTTP 처리

Repository Interface는 Service 패키지에서 선언한다.

구현체는 `platform/postgres`에 위치한다.

---

# SQL 규칙

- SQL은 `db/queries` 에만 작성한다.
- sqlc를 사용한다.
- Go 코드에 Raw SQL 문자열을 작성하지 않는다.
- SELECT * 사용을 지양한다.
- 필요한 컬럼만 조회한다.
- Migration은 golang-migrate 형식을 따른다.

---

# 코딩 규칙

## UUID

모든 Primary Key는 UUID v7 사용

```go
uuid.NewV7()
```

---

## Context

모든 Service 및 Repository 함수는 첫 번째 인자로 Context를 받는다.

```go
func FindByID(ctx context.Context, id uuid.UUID)
```

---

## Validation

입력값 검증은 경계에서 수행한다.

- Handler
- Service 진입부

Fail Fast 원칙을 따른다.

---

## Error Handling

panic 사용 금지

에러는 반환한다.

```go
return fmt.Errorf("failed to create user: %w", err)
```

Sentinel Error 사용 가능

```go
var ErrUserNotFound = errors.New("user not found")
```

---

## Logging

log/slog 사용

구조화 로그 사용

허용

- User ID
- Client ID
- Request ID

금지

- Access Token
- Refresh Token
- OAuth Code
- OAuth Secret
- Client Secret

---

# 테스트 규칙

- Table Driven Test 사용
- Service Layer 우선 테스트
- Repository Mock 사용
- Happy Path 작성
- Failure Case 작성
- 외부 API는 Mock 처리

---

# AI 작업 절차

코드 생성 전 수행

1. 요구사항 요약
2. 구현 계획 설명
3. 변경 파일 설명
4. 코드 작성

---

# AI 수정 규칙

기존 코드 수정 시 설명

- 변경 이유
- 영향 범위
- 호환성 영향
- 마이그레이션 필요 여부

---

# 금지 사항

- API 추측 생성 금지
- 사용하지 않는 패키지 추가 금지
- TODO 코드 남기기 금지
- Mock 구현을 운영 코드에 포함 금지
- 전역 상태 생성 금지
- 과도한 추상화 금지
- 미래 요구사항을 위한 설계 금지(YAGNI)
- 기술 스택 변경 금지
