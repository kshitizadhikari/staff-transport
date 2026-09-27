# Architecture Decision Records

This file records decisions that materially affect the project.

## ADR-001: Use a Modular Monolith

**Decision:** Use one Go backend with explicit domain modules.

**Reason:** The initial product has tightly related workflows, one organization context, and no demonstrated need for independent service scaling or deployment.

**Revisit when:** distinct modules require independent scaling/deployment or organizational boundaries make service isolation worthwhile.

## ADR-002: Separate Driver Mobile App

**Decision:** Use React Native + Expo for the driver application.

**Reason:** Driver workflows rely on device capabilities including GPS, push notifications, navigation, and resilience to mobile-network conditions.

## ADR-003: PostgreSQL + PostGIS

**Decision:** Use PostgreSQL with PostGIS.

**Reason:** The domain contains coordinates, distances, nearby-driver queries, route-related data, and geographic stops.

## ADR-004: Trip-Centric Domain Model

**Decision:** Model transportation operations as trips with stops and passengers.

**Reason:** A trip can contain many passengers, many stops, and different transportation scenarios while retaining one consistent lifecycle.

## ADR-005: REST First

**Decision:** Start with REST APIs. Add realtime transport only where a concrete live-update requirement exists.

**Reason:** REST keeps the initial system simple. Live driver tracking can later use SSE/WebSocket without forcing realtime architecture onto every feature.

## ADR-006: pnpm Workspaces + Turborepo

**Decision:** Manage the JavaScript/TypeScript monorepo with pnpm workspaces and Turborepo.

**Reason:** The web and driver apps share tooling and will share API types. pnpm provides strict, space-efficient dependency management and Turborepo gives cached, dependency-aware task running. The Go backend remains outside the JS workspace.

## ADR-007: JWT Access Tokens with Redis Refresh Tokens

**Decision:** Issue short-lived JWT access tokens and opaque refresh tokens whose live state and revocation are stored in Redis. Refresh tokens rotate on use.

**Reason:** Access tokens stay stateless and cheap to verify. Redis-backed refresh tokens allow immediate revocation and logout without a per-request database lookup, reusing infrastructure already required by Asynq. Durable issuance may optionally be audited in the `refresh_tokens` table.
