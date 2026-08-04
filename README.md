# go-api-starter

Go + Gin API starter และ OpenAPI source of truth สำหรับ `go-lang-starter` suite

> สถานะปัจจุบัน: มี API process/config/HTTP/database foundation และ personal
> Back Office baseline พร้อมต่อยอดเป็น project ใหม่ได้ โดย SaaS tenancy/billing
> ยังแยกไว้สำหรับ `go-lang-starter-saas`

## Stack

- Go module: `github.com/korwlik23/go-api-starter`
- HTTP transport: Gin
- Persistence: GORM
- Migrations: Goose
- Contract validation: kin-openapi และ oapi-codegen runtime
- Primary database: PostgreSQL
- First-class local profile: MariaDB/XAMPP
- Compatibility profile: Oracle MySQL 8.4.11 (ผ่าน disposable integration tests แล้ว)

## Responsibilities

Repository นี้เป็นเจ้าของ:

- API runtime, workers, migrations และ bootstrap commands
- bounded-context business logic และ persistence adapters
- OpenAPI source/bundles สำหรับ Admin, Public และ Site server
- database-profile compatibility tests
- authentication, opaque server-side sessions, CSRF/CORS และ rate limiting
- permission-based authorization แบบ default deny โดยไม่ hardcode role name
- locale registry และ translation catalog ที่ system permission จัดการได้
- system settings, in-app notifications และ durable operations jobs
- profile/password self-service พร้อม session invalidation และ permission guards
- catalog import/export, backup manifest verification และ local mail provider boundary

ฟีเจอร์ที่ตั้งใจให้เป็น provider/module ต่อเติมภายหลัง ได้แก่ registration/email
verification delivery, SMTP, S3/MinIO และ legal/demo content seed; ไม่ฝัง SaaS
tenancy, billing, subscription หรือ Stripe ไว้ใน starter ส่วนตัว

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

คำสั่งมาตรฐาน:

```powershell
go test ./...
go build ./cmd/api
go run ./cmd/api
```

`cmd/api` เป็น process-lifecycle entry point ของ runtime composition ส่วน `cmd/migrate`,
`cmd/bootstrap` และ `cmd/backup` ใช้สำหรับ schema, local foundation และ backup manifest
ตามลำดับ

## Database profiles

เลือก config เริ่มต้นจาก `.env.postgres.example`, `.env.mariadb-xampp.example` หรือ
`.env.mysql.example` โดยรองรับ profile ต่อไปนี้:

- `DB_DRIVER=postgres`, `DB_FLAVOR=` สำหรับ PostgreSQL
- `DB_DRIVER=mysql`, `DB_FLAVOR=mariadb` สำหรับ MariaDB และ dedicated XAMPP database
- `DB_DRIVER=mysql`, `DB_FLAVOR=mysql` เป็น Oracle MySQL compatibility profile

`DB_HOST` รับ hostname, IPv4 หรือ IPv6 แบบไม่ใส่ `[]` และไม่รวม port ส่วน
`DB_CONNECT_TIMEOUT` ต้องมากกว่าศูนย์ ระบบจะใช้ deadline ที่สั้นกว่าระหว่างค่านี้กับ
deadline ของ request/caller

`DB_MIGRATION_LOCK_TIMEOUT` คือเวลาสูงสุดที่ migration runner รอ advisory lock
ก่อนหยุดแบบ fail closed ค่านี้ต้องมากกว่าศูนย์ และ starter กำหนดค่าเริ่มต้นไว้ที่ `2m`
เพื่อป้องกันหลาย deployment รัน schema migration พร้อมกัน

Advisory lock นี้เป็น session-level coordination สำหรับ database primary/server เดียว
จึงต้องใช้ session pooling และให้ migration runners ชี้ primary เดียวกัน หากใช้
transaction pooling, multi-primary หรือ topology ที่ lock ไม่ครอบทั้งระบบ ต้องเปลี่ยน
เป็น distributed migration coordinator ก่อน deploy

Migration ของแต่ละ module ต้องมี bundle ครบใน `postgres`, `mariadb` และ `mysql`
ชื่อไฟล์ใช้รูปแบบ `000001_descriptive_name.sql` และต้องมี version/basename ตรงกันทุก
dialect แม้ SQL ภายในต่างกันได้ migration composition ต้องเรียก
`ValidateMigrationDialectParity` ก่อนเชื่อมต่อฐานข้อมูล ส่วน
`MigrationCatalogChecksum` ใช้ SHA-256 ของ catalog และ exact file bytes ที่เรียงแบบ
deterministic เพื่อตรวจ artifact drift

### Transaction boundary

Application use case ให้พึ่ง `database.Transactor` และส่ง `context.Context` เดิมลงไปยัง
repository ส่วน repository ให้พึ่ง `database.ConnectionProvider` เพื่อรับ root connection
หรือ transaction connection ที่ผูกกับ context โดยไม่ต้องรู้ว่า caller เปิด transaction อยู่หรือไม่

`TransactionManager` ควบคุม `Begin`, `Commit` และ `Rollback` โดยตรง nested call ของ
manager instance เดียวกันจะ reuse transaction เดิมโดยไม่สร้าง savepoint หาก nested error
หรือ panic ถูก caller จับกลืน outer transaction จะกลายเป็น rollback-only เพื่อป้องกัน partial
commit ส่วน context จาก manager คนละ instance หรือ context ที่หลุดออกมาใช้หลัง transaction
สิ้นสุดแล้วจะถูกปฏิเสธแบบ fail closed callback จึงต้องทำงานให้เสร็จภายใน boundary เท่านั้น

manager จะคืน stable infrastructure errors โดยไม่เปิดเผย raw driver detail แต่ callback error
จะถูก preserve ไว้ ดังนั้น persistence adapter ต้อง map SQL/driver errors เป็น stable
repository หรือ domain errors ก่อนคืนออกจาก callback

TLS เป็นแบบ fail closed:

- PostgreSQL: `disable`, `require`, `verify-ca`, `verify-full`
- MariaDB/MySQL: `disable`, `require`, `verify-full`

`DB_TIME_ZONE` ต้องเป็น IANA timezone ที่ถูกต้อง สำหรับ PostgreSQL ค่านี้กำหนดทั้ง
session timezone และตำแหน่งเวลาที่ใช้ scan `timestamp` ส่วน MariaDB/MySQL บังคับค่า
เป็น `UTC` และตั้ง session `time_zone=+00:00` เพื่อให้ `TIMESTAMP` มีความหมายตรงกัน
ทุก environment การแสดง timezone ของผู้ใช้ให้ทำใน application/presentation layer

ทดสอบกับ disposable databases ผ่าน Docker:

```powershell
.\scripts\test-db.ps1 -Profile postgres
.\scripts\test-db.ps1 -Profile mariadb
```

runner จะสร้าง/ใช้เฉพาะฐานข้อมูลชื่อ `go_lang_starter_test_*`, ตรวจ destructive guard,
รัน integration tests และหยุดเฉพาะ service ที่ runner เป็นผู้เปิด โดยไม่ลบ volume

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
