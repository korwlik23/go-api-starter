# go-api-starter

Go + Gin API starter และ OpenAPI source of truth สำหรับ `go-lang-starter` suite

> สถานะปัจจุบัน: เริ่ม API foundation แล้ว แต่ application server, migrations และ
> feature modules ยังอยู่ระหว่างพัฒนา

## Stack

- Go module: `github.com/korwlik23/go-api-starter`
- HTTP transport: Gin
- Persistence: GORM
- Migrations: Goose
- Contract validation: kin-openapi และ oapi-codegen runtime
- Primary database: PostgreSQL
- First-class local profile: MariaDB/XAMPP
- Compatibility profile: Oracle MySQL (ห้ามประกาศ supported จน integration tests ผ่าน)

## Responsibilities

Repository นี้เป็นเจ้าของ:

- API runtime, workers, migrations และ bootstrap commands
- bounded-context business logic และ persistence adapters
- OpenAPI source/bundles สำหรับ Admin, Public และ Site server
- database-profile compatibility tests
- authentication, opaque server-side sessions, CSRF/CORS และ rate limiting
- permission-based authorization แบบ default deny โดยไม่ hardcode role name
- locale registry และ translation catalog ที่ system permission จัดการได้

Admin และ Site ต้องใช้ generated clients จาก immutable OpenAPI artifacts ห้าม import
runtime source จาก repository นี้โดยตรง

## Development prerequisites

- Go `1.26.5`
- PostgreSQL หรือ dedicated MariaDB/XAMPP test database
- Docker สำหรับ disposable integration profiles

ติดตั้งและตรวจ module dependencies:

```powershell
go mod download
go mod verify
```

เมื่อ application bootstrap พร้อมแล้ว คำสั่งมาตรฐานจะเป็น:

```powershell
go test ./...
go build ./cmd/api
go run ./cmd/api
```

ระหว่างสถานะ foundation ให้ยึดคำสั่ง verification ที่ระบุใน commit/plan ของ task
ปัจจุบัน เพราะ `cmd/api` ยังไม่ถูกสร้าง

## Configuration and secrets

- local config ใช้ environment variables
- `.env` และ private key files ถูก ignore
- `.env*.example` ต้องมี dummy values เท่านั้น
- ห้าม log session ID, password, token, raw cookie, database password หรือ PII
- production ต้อง fail fast เมื่อ secret/config สำคัญหายหรือเป็น placeholder

## Planned structure

```text
cmd/                   executable entry points
internal/app/          lifecycle and dependency composition
internal/platform/     config, HTTP, telemetry, database, middleware
internal/modules/      bounded contexts such as identity and localization
internal/generated/    generated OpenAPI code; never edit manually
openapi/               source contracts and immutable bundles
migrations/            engine-specific Goose migrations
tests/                 integration, contract, migration, and security tests
```
