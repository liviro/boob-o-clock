# Boob O'Clock

A 24-hour baby sleep and feed tracker for breastfeeding parents. Built for one-handed use on a small phone screen.

Dark mode only. Single tap to record events. Long-press for time adjustments.

## Why I built this

It started in the thick of newborn nights. At 3am, mid-feed, I genuinely could not remember which side and when I'd last fed on — and every baby tracker I found wanted an account, cloud sync, and a bright white screen in my face. So I vibecoded this for myself in a few evenings, and it's already been useful enough that I wanted to share it.

As the baby got older, more questions came up alongside the old ones. At 3am: "which side last?" During the day: "how long has the baby been awake?" and "was that nap 20 minutes or 45?" So the app grew a day mode — same state-machine approach, now covering the full 24-hour rhythm.

Just your data, on your network, no account and no ads. If Boob O'Clock helped you survive the infant days and nights, you can [buy me a coffee](https://ko-fi.com/polinaturcu) — it means a lot.

## What it tracks

The app models your day and night as a single state machine. Depending on the current state, only the valid next actions are shown:

### Night
- **Feeds** — start, switch breast (auto-flips), dislatch (awake or asleep). Tracks left/right duration separately.
- **Sleep** — on me, in crib, in stroller. Tracks where the baby is sleeping.
- **Transfers** — crib transfer attempts with deferred outcome (tap result when hands are free).
- **Self-soothing** — baby put down awake or stirring in crib, settling without intervention.
- **Resettling** — in-crib settling without a feed.
- **Strolling** — the nuclear option when the crib isn't working.
- **Ferber mode** _(off by default)_ — graduated check-in intervals (classic Ferber table), mood tracking (quiet / fussy / crying), and a countdown on the check-in button so you never check in too early. We tried it; the method was rough on our household, so we ship it disabled. Set `FERBER_ENABLED=true` to opt in (no rebuild needed); toggle off any time — past Ferber data stays in the DB and reappears when you turn it back on.

### Day
- **Day feeds** — same feed tracking as night, with switch-breast suggestion based on the last side.
- **Naps** — tagged with location (crib / stroller / on me / car). Durations roll up to daily nap stats.
- **Wake windows** — automatically derived from the awake/nap rhythm, including the last wake window before bedtime.

**Diaper changes** are reachable anytime, day or night. The tracker moves between day and night with a single **Start day** / **Start night** tap — no separate "end" action.

## What it reports

- **Cycle view**: one card per 24h midnight-to-midnight window, stacked chronologically. Each card shows a color-coded timeline bar of the full day and the following night, tinted day/night sections, and live sleep/wake duration pills (the in-progress segment blinks).
- Per-night summary: night duration, total sleep, total feed time, wake count, feed count, longest sleep block, individual sleep block durations, feed times
- Per-day summary: nap count, total nap time, longest nap, day feed count and duration, wake windows, last wake window before bedtime
- Ferber nights (when enabled) also show sessions, average time to settle, cry time, fuss time, check-ins, abandoned sessions, and quiet time
- Full event log with timestamps
- Feed times scatter plot showing when feeds happen across 24 hours
- Real bedtime chart showing when the baby actually goes down
- Trend charts with moving averages: longest sleep, total sleep, wake count, feed count, total feed time, feed time by breast (L/R), nap count, total nap time
- Ferber trend charts (when any night had Ferber on): cry time per night, check-ins per night, avg time to settle
- Ferber nights are highlighted as sage-green blocks on all non-Ferber trend charts, so you can correlate Ferber periods with broader sleep/feed changes
- CSV export for backup or analysis

## Screenshots

<p align="center">
  <img src="docs/tracker-awake.png" width="250" alt="Tracker — awake state">
  <img src="docs/night-detail.png" width="250" alt="Night detail with timeline">
  <img src="docs/trends.png" width="250" alt="Trend charts with moving averages">
</p>

## Deploy

### Docker Compose (recommended)

```bash
git clone https://github.com/liviro/boob-o-clock.git
cd boob-o-clock
docker compose up -d
```

That's it. The app is at `http://localhost:8080`.

To update:

```bash
docker compose build --no-cache
docker compose up -d
```

The SQLite database lives in a named Docker volume (`boc-data`) and survives rebuilds. Back it up with:

```bash
docker compose cp boob-o-clock:/data/boob-o-clock.db ./backup.db
```

### Docker (manual)

```bash
docker build -t boob-o-clock .
docker run -d \
  --name boob-o-clock \
  --restart unless-stopped \
  -p 8080:8080 \
  -v boc-data:/data \
  boob-o-clock
```

To change the port, set the `PORT` environment variable:

```bash
docker run -d -e PORT=9090 -p 9090:9090 -v boc-data:/data boob-o-clock
```

### Binary

Requires Go 1.25+ and Node 22+.

```bash
cd web && npm install && cd ..
make build
./boob-o-clock -addr :8080 -db ./boob-o-clock.db
```

### Configuration

| Env | Flag | Default | Description |
|---|---|---|---|
| `PORT` | `-addr :PORT` | `8080` | Listen port |
| `FERBER_ENABLED` | `-ferber` | `false` | Enable Ferber sleep-training mode (see [What it tracks](#what-it-tracks)) |

For Docker Compose, set these under `environment:` in `docker-compose.yml` and run `docker compose up -d` to apply.

### Access from your phone

Open `http://<your-server-ip>:8080` in Safari and tap **Share → Add to Home Screen**. The app launches fullscreen like a native app.

> **Note:** The PWA service worker requires HTTPS on non-localhost. For local network use, accessing via IP on HTTP works fine — you just won't get offline caching. To enable HTTPS, put a reverse proxy (Caddy, nginx) in front with a self-signed or Let's Encrypt cert.

## Develop

```bash
# Install frontend dependencies
cd web && npm install && cd ..

# Run Go backend on :8080 and Vite dev server on :5173
make dev

# Open http://localhost:5173 — hot reload for frontend, API proxied to Go
```

### Seed data

```bash
go run ./cmd/seed -db ./dev.db          # a week of plausible cycles
go run ./cmd/server -addr :8080 -db ./dev.db
```

Generates one orphan historical night followed by full (day, night) cycles and an in-progress today, covering a realistic spread: long stretches, multi-wake rough nights, stroller blocks, resettles, varied nap locations and counts, poop, breast alternation, and Ferber sessions (including one abandoned mid-night falling back to feed-to-sleep).

### Test

```bash
make test              # Go tests (150+ tests across 4 packages)
cd web && npx tsc      # TypeScript type check
cd web && npm run lint # ESLint (react-hooks rules)
```

### Project structure

```
├── cmd/server/          Entry point, wiring, embed
├── internal/
│   ├── domain/          Unified state machine (17 states, 53 transitions, zero deps)
│   ├── store/           SQLite persistence (pure Go, no CGo)
│   ├── reports/         Cycle/day/night stats, timelines, trends, breast tracking, Ferber session derivation
│   ├── api/             REST handlers
│   └── web/             Embedded frontend (go:embed)
└── web/                 Preact + TypeScript + Vite source
```

### API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/config` | Server feature flags (consumed by the frontend on boot) |
| GET | `/api/session/current` | Current state + valid actions |
| POST | `/api/session/start` | Start a new day or night session (chain-advance; optional Ferber config on night) |
| POST | `/api/session/event` | Record an event |
| POST | `/api/session/undo` | Undo last event (chain-aware — reopens the prior session if the last event was a chain-advance) |
| GET | `/api/cycles` | Cycle list with per-cycle day+night stats and moving averages |
| GET | `/api/cycles/{id}` | Full cycle detail (both sessions, combined timeline, stats) |
| GET | `/api/export/csv` | Download all events as CSV |
| GET | `/healthz` | Health check (DB ping) |

## License

MIT
