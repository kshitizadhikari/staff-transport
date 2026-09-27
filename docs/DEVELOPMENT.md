# Development Guide

## Repository Layout

```text
apps/web
apps/driver-mobile
backend
infra
docs
```

## Local Dependencies

The first local environment should provide:

- PostgreSQL with PostGIS
- Redis

Application services can be run directly during development or as containers once their Dockerfiles exist.

## Environment

The backend reads the repository-root `.env`. The web and driver apps read
their own environment files.

```bash
cp .env.example .env
cp apps/web/.env.example apps/web/.env.local
cp apps/driver-mobile/.env.example apps/driver-mobile/.env
```

Never commit real secrets.

## Package Management

The JavaScript/TypeScript monorepo uses pnpm workspaces with Turborepo
(ADR-006).

```bash
pnpm install        # install all workspace dependencies
pnpm build          # build all apps
pnpm lint           # lint all apps
pnpm typecheck      # typecheck all apps
pnpm dev            # run all apps in dev mode
```

Use `pnpm --filter <app> <script>` to target one app.

## Backend Development

Recommended checks:

```bash
go fmt ./...
go test ./...
go vet ./...
```

Add linting once the backend structure stabilizes.

## Web Development

Recommended checks:

```bash
npm run lint
npm run typecheck
npm run build
```

Exact scripts may vary by workspace tooling.

## Mobile Development

Use Expo tooling for local builds and device testing.

Test GPS functionality on a real device; an emulator is not sufficient for validating all location/background behavior.

## Database Changes

All schema changes must use migrations.

Do not depend on automatic production schema mutation by the ORM.

Migrations live in `backend/migrations` and use golang-migrate naming:

```text
0001_init.up.sql
0001_init.down.sql
```

Apply and roll back migrations with the `golang-migrate` CLI:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5434/staff_transport?sslmode=disable'
make migrate-up
make migrate-down
```

Every migration must be reversible and safe to run on a database that already
contains data.

## Feature Development Order

For a business feature such as trip creation:

```text
1. domain requirements
2. database migration
3. backend service
4. backend API
5. API tests
6. web UI / mobile UI
7. integration testing
```

## Branch Naming

Recommended:

```text
feature/trip-creation
feature/driver-trip-execution
fix/trip-capacity-validation
chore/upgrade-dependencies
```

## Commit Naming

Use clear imperative/conventional-style commits where practical:

```text
feat: add trip creation API
feat: add driver stop execution flow
fix: prevent overlapping driver assignments
chore: add local redis service
```

## Pull Request Expectations

Every PR should explain:

- what changed
- why it changed
- affected roles/modules
- database changes
- API changes
- tests performed
- known limitations

## Local Quality Gate

Before considering work complete:

```text
[ ] Formatting passes
[ ] Unit tests pass
[ ] Relevant integration tests pass
[ ] Type checking passes
[ ] Lint passes
[ ] Build passes
[ ] Authorization reviewed
[ ] Error cases reviewed
[ ] Timezone behavior reviewed
[ ] Duplicate/retry behavior reviewed
```

## Timezone Testing

Test scheduled trips around timezone boundaries and daylight-saving changes even if the initial deployment timezone does not currently observe DST. The data model should not depend on a machine's local timezone.
