# Runbook

How to run the entire staff transport project locally.

## Prerequisites

- Go 1.26+
- Node 20+ (Node 24 recommended, see `.nvmrc`)
- Docker
- `golang-migrate` CLI (optional, only for the `make migrate-*` targets)

Enable pnpm once if it is not already available:

```bash
corepack enable
corepack prepare pnpm@10.15.0 --activate
```

## 1. Install dependencies

```bash
pnpm install
cd backend && go mod download && cd ..
```

## 2. Create environment files

```bash
cp .env.example .env
cp apps/web/.env.example apps/web/.env.local
cp apps/driver-mobile/.env.example apps/driver-mobile/.env
```

The backend reads the repository-root `.env`. The web and driver apps read
their own env files.

## 3. Start infrastructure (PostgreSQL/PostGIS + Redis)

The backend runs as Docker Compose services, so it reaches PostgreSQL and
Redis by service name (`postgres`, `redis`). PostgreSQL is published on host
port `5434` and Redis on `6379` for host tools such as the web app.

```bash
make infra-up
```

## 4. Apply database migrations

Migrations run in a one-off `migrate` container on the compose network, so no
local `golang-migrate` install or `DATABASE_URL` export is needed.

```bash
make migrate-up
make migrate-down     # roll back the latest migration
```

Create a local manager account for logging in (idempotent; re-run to reset the
password):

```bash
make seed-manager
```

Defaults are `manager@example.com` / `changeme123`. Override with
`SEED_MANAGER_NAME`, `SEED_MANAGER_EMAIL`, and `SEED_MANAGER_PASSWORD` in `.env`.

## 5. Run the backend

Run each in its own terminal (the first run builds the image):

```bash
make backend-run      # API on :8080
make backend-worker   # Asynq background worker
```

To build and start the whole stack (db, redis, api, worker) in the background:

```bash
make stack-up
make backend-logs     # tail api + worker logs
make stack-down
```

Verify:

```bash
curl localhost:8080/health
curl localhost:8080/ready
curl -s localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"manager@example.com","password":"changeme123"}'
```

## 6. Run the web app

```bash
pnpm --filter web dev     # http://localhost:3000
```

## 7. Run the driver mobile app

```bash
pnpm --filter driver-mobile start
```

Then press `a` (Android), `i` (iOS), or `w` (web), or scan the QR code with
Expo Go.

`EXPO_PUBLIC_API_URL=http://localhost:8080/api/v1` works for web and emulators.
A physical device needs your machine's LAN IP instead of `localhost`.

## 8. Quality checks

```bash
make backend-check        # go fmt + vet + test
pnpm lint
pnpm typecheck
pnpm build
```

## Teardown

```bash
make stack-down       # stop api + worker + db + redis
# or
make infra-down
```

## Command summary

```text
pnpm install
cp .env.example .env && cp apps/web/.env.example apps/web/.env.local && cp apps/driver-mobile/.env.example apps/driver-mobile/.env
make infra-up
make migrate-up
make backend-run      # terminal 1
make backend-worker   # terminal 2
pnpm --filter web dev
pnpm --filter driver-mobile start
```

Shortcut: `make stack-up` builds and starts db, redis, api, and worker in one
command after `make migrate-up`.
