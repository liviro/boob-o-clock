# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**Boob O'Clock** — a nighttime baby sleep/feed tracker for breastfeeding parents. Dark-mode-only PWA optimized for one-handed use on an iPhone SE at 3am.

## Architecture

Single Go binary serving a REST API and embedded frontend static files.

- `internal/domain/` — Pure state machine and types. **Zero external dependencies.** This is the source of truth for all state transitions. State is always derived from the event log, never stored separately.
- `internal/store/` — SQLite persistence via `modernc.org/sqlite` (pure Go, no CGo). Stores events and nights.
- `internal/reports/` — Report computation over event data. Pure Go.
- `internal/api/` — HTTP handlers. Thin orchestration: loads session from store, validates via domain, persists, returns JSON.
- `cmd/server/` — Entry point. Wires dependencies, embeds frontend via `go:embed`, serves on configurable port.
- `web/` — Frontend assets (Phase 1: single HTML file; later: Preact PWA build output in `web/dist/`).

## Core Domain: State Machine

10 states, 28 transitions. The transition table lives in `internal/domain/machine.go`. Key properties:
- AWAKE is the hub state — every state can reach it, and it's the only state that can end the night
- TRANSFERRING is instantaneous (deferred outcome — user picks result when hands are free)
- POOP is reachable from 6 states (everything except FEEDING, TRANSFERRING, NIGHT_OFF)
- FEEDING supports a self-transition (switch breast) that logs dislatch + restart

## Commands

```bash
# Run all tests
go test ./...

# Run domain tests verbose
go test ./internal/domain/ -v

# Build binary
go build -o boob-o-clock ./cmd/server

# Run server
./boob-o-clock -addr :8080 -db ./data.db
```

## Conventions

- **TDD**: Write tests first, always. Red-green-refactor.
- **Domain purity**: `internal/domain/` must have zero dependencies outside the standard library.
- **Event sourcing**: Current state is derived from the event log via `DeriveState()`. No separate "current_state" column.
- **Timestamps**: All stored as RFC3339 with timezone offset. Frontend sends local time with offset.
- **Dark mode only**: No light theme. Background #000, designed for nighttime use.
- **Tap targets**: Minimum 48px, prefer 64px for primary actions. Design for iPhone SE (375px wide).
- **Metadata**: Stored as `map[string]string` serialized to JSON. Feed events carry `{"breast": "L"}` or `{"breast": "R"}`.
