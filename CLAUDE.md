# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**Boob O'Clock** — a nighttime baby sleep/feed tracker for breastfeeding parents. Dark-mode-only PWA optimized for one-handed use on an iPhone SE at 3am.

## Architecture

Single Go binary serving a REST API and embedded frontend static files.

- `internal/domain/` — Pure state machine and types. **Zero external dependencies.** This is the source of truth for all state transitions. State is always derived from the event log, never stored separately.
- `internal/store/` — SQLite persistence via `modernc.org/sqlite` (pure Go, no CGo). Stores events and nights.
- `internal/reports/` — Report computation over event data. Pure Go.
- `internal/api/` — HTTP handlers. Orchestration: loads session from store, validates via domain, persists, enriches response with report data (e.g. breast suggestion), returns JSON.
- `internal/web/` — Embedded frontend assets via `go:embed`. Static files live in `internal/web/static/`.
- `cmd/server/` — Entry point. Wires dependencies, serves API + embedded frontend on configurable port.

## Core Domain: State Machine

11 states, 32 transitions. The transition table lives in `internal/domain/machine.go`. Key properties:
- AWAKE is the hub state — every state can reach it, and it's the only state that can end the night
- TRANSFERRING is instantaneous (deferred outcome — user picks result when hands are free)
- SELF_SOOTHING is reachable from SLEEPING_CRIB (baby stirred) and AWAKE (put down awake)
- POOP is reachable from 7 states (everything except FEEDING, TRANSFERRING, NIGHT_OFF)
- FEEDING supports a self-transition (switch breast) that logs dislatch + restart

## Commands

```bash
# Build everything (frontend + Go binary)
make build

# Run all Go tests
make test

# Dev mode: Go on :8080, Vite on :5173 (with API proxy)
make dev

# Or individually:
go test ./...                              # Go tests
go test ./internal/domain/ -v              # domain tests verbose
cd web && npx tsc --noEmit                 # TypeScript type check
cd web && npm run build                    # build frontend only
go build -o boob-o-clock ./cmd/server      # build Go binary only
./boob-o-clock -addr :8080 -db ./data.db   # run server

# Docker
docker build -t boob-o-clock .
docker run -p 8080:8080 -v boc-data:/data boob-o-clock
```

## Frontend

Preact + TypeScript + Vite. Source in `web/src/`, builds to `internal/web/static/`.
- `web/src/api.ts` — typed API client
- `web/src/constants.ts` — state/action definitions, formatters
- `web/src/hooks/useSession.ts` — session state management hook
- `web/src/components/` — reusable UI components
- `web/src/pages/` — Tracker and History pages

## Conventions

- **TDD**: Write tests first, always. Red-green-refactor.
- **Domain purity**: `internal/domain/` must have zero dependencies outside the standard library.
- **Event sourcing**: Current state is derived from the event log via `DeriveState()`. No separate "current_state" column.
- **Timestamps**: All stored as RFC3339 with timezone offset. Frontend sends local time with offset.
- **Dark mode only**: No light theme. Background #000, designed for nighttime use.
- **Tap targets**: Minimum 48px, prefer 64px for primary actions. Design for iPhone SE (375px wide).
- **Metadata**: Stored as `map[string]string` serialized to JSON. Feed events carry `{"breast": "L"}` or `{"breast": "R"}`.
- **Versioning**: Version is defined in `web/package.json`. When bumping: update `package.json`, run `npm install --package-lock-only` in `web/` to sync `package-lock.json`, commit and push both, then create a GitHub release with `gh release create`.
