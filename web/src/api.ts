import { fmtLocalYMDHM } from './constants';

const API = '/api';

// Mirrors internal/domain.State. Keep in sync with internal/domain/states.go.
export type State =
  | 'night_off'
  // Night subgraph
  | 'awake'
  | 'feeding'
  | 'sleeping_on_me'
  | 'transferring'
  | 'resettling'
  | 'sleeping_crib'
  | 'strolling'
  | 'sleeping_stroller'
  | 'self_soothing'
  | 'poop'
  | 'learning'
  | 'check_in'
  | 'chair'
  // Day subgraph
  | 'day_awake'
  | 'day_feeding'
  | 'day_sleeping'
  | 'day_poop';

export type SessionKind = 'night' | 'day';

export type Location = 'crib' | 'stroller' | 'on_me' | 'car';

export interface SessionResponse {
  // Null iff no active session (state === 'night_off').
  kind: SessionKind | null;
  state: State;
  validActions: string[];
  sessionId: number | null;
  suggestBreast?: string;
  currentBreast?: string;
  lastFeedStartedAt?: string;
  lastWokeUpAt?: string;
  lastEvent: {
    action: string;
    fromState: string;
    toState: string;
    metadata?: Record<string, string>;
    timestamp: string;
  } | null;
  // Present when the current session is a Ferber night.
  ferber?: {
    nightNumber: number;
    // Present when the current state is Learning or CheckIn.
    current?: {
      checkInCount: number;
      startedAt: string;
      // Absolute timestamp at which the next check-in becomes available.
      // Present only in Learning state (absent during CheckIn).
      checkInAvailableAt?: string;
      mood: 'quiet' | 'fussy' | 'crying';
    };
  };
  // Present whenever start_night is a valid action: night_off AND day_awake.
  suggestFerberNight?: number;
  chairEnabled?: boolean;
  suggestChair?: boolean;
}

export interface ServerConfig {
  features: {
    ferber: boolean;
    chair: boolean;
  };
}

// --- cycle types ---

export interface SessionMeta {
  id: number;
  kind: SessionKind;
  startedAt: string;
  endedAt?: string;
  ferberEnabled?: boolean;
  ferberNightNumber?: number | null;
  chairEnabled?: boolean;
}

export interface DaySegment {
  kind: 'awake' | 'nap';
  duration: number;               // ns
}

export interface DayStats {
  dayDuration: number;            // ns; clamps to "now" for in-progress
  napCount: number;
  totalNapTime: number;          // ns
  dayFeedCount: number;
  dayTotalFeedTime: number;       // ns
  wakeWindows: number[];          // ns — awake-kind subset of daySegments
  lastWakeWindow: number | null;  // ns
  // Alternating awake/nap rhythm in order. Drives the "Day rhythm" pills.
  daySegments: DaySegment[];
}

export interface FerberStats {
  sessions: number;
  checkIns: number;
  cryTime: number;
  fussTime: number;
  quietTime: number;
  sessionsAbandoned: number;
  avgTimeToSettle: number;
}

// NightStats retains the same shape as today; now only present on the night
// half of a CycleStats.
export interface NightStats {
  nightDuration: number;
  totalSleepTime: number;
  totalFeedTime: number;
  intraSleepFeedTime: number;
  feedTimeLeft: number;
  feedTimeRight: number;
  totalAwakeTime?: number;
  feedCount: number;
  wakeCount: number;
  longestSleepBlock: number;
  sleepBlocks: number[];
  feedTimes: string[] | null;
  realBedtime?: string | null;
  ferber?: FerberStats | null;
}

export interface CycleStats {
  day: DayStats | null;
  night: NightStats | null;
}

export interface CycleSummary {
  day: SessionMeta | null;
  night: SessionMeta | null;
  // All events in the cycle (day + night), timestamp-ordered. Always present;
  // empty for a cycle with no sessions (shouldn't occur, but tolerate).
  events: EventEntry[];
  stats: CycleStats;
  avg: CycleStats | null;
}

export interface TimelineEntry {
  state: string;
  start: string;
  duration: number;
  metadata?: Record<string, string>;
}

export interface EventEntry {
  action: string;
  fromState: string;
  toState: string;
  metadata?: Record<string, string>;
  timestamp: string;
}

export interface CycleDetail {
  cycle: {
    day: SessionMeta | null;
    night: SessionMeta | null;
  };
  events: EventEntry[];
  // Night-only timeline segments.
  timeline: TimelineEntry[];
  // Day-only timeline segments (same shape as `timeline`, scoped to the day
  // session's events and end time).
  dayTimeline: TimelineEntry[];
  stats: CycleStats;
}

// --- helpers ---

/** Format a Date as RFC3339 with local timezone offset (not UTC). */
function toLocalISO(d: Date): string {
  const off = -d.getTimezoneOffset();
  const sign = off >= 0 ? '+' : '-';
  const pad = (n: number) => String(n).padStart(2, '0');
  const hh = pad(Math.floor(Math.abs(off) / 60));
  const mm = pad(Math.abs(off) % 60);
  return `${fmtLocalYMDHM(d)}:${pad(d.getSeconds())}${sign}${hh}:${mm}`;
}

async function checkResponse(resp: Response): Promise<Response> {
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({}));
    throw new Error(err.error || `Request failed (${resp.status})`);
  }
  return resp;
}

// --- fetch functions ---

export async function getConfig(): Promise<ServerConfig> {
  const resp = await checkResponse(await fetch(`${API}/config`));
  return resp.json();
}

export async function getCurrentSession(): Promise<SessionResponse> {
  const resp = await checkResponse(await fetch(`${API}/session/current`));
  return resp.json();
}

export async function postEvent(
  action: string,
  metadata?: Record<string, string>,
  timestamp?: Date
): Promise<SessionResponse> {
  const body: Record<string, unknown> = { action };
  if (metadata) body.metadata = metadata;
  if (timestamp) body.timestamp = toLocalISO(timestamp);

  const resp = await checkResponse(await fetch(`${API}/session/event`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }));
  return resp.json();
}

// NightModeChoice is the picker's discriminated-union output. Mutually
// exclusive by construction: a single mode value carries any mode-specific
// data (e.g. ferber nightNumber), so the picker can't emit "both modes set".
export type NightModeChoice =
  | { mode: 'plain' }
  | { mode: 'ferber'; nightNumber: number }
  | { mode: 'chair' };

export interface StartSessionConfig {
  kind: SessionKind;
  // Only meaningful when kind === 'night'. Defaults to plain when omitted.
  choice?: NightModeChoice;
  timestamp?: Date;
}

export async function postStartSession(config: StartSessionConfig): Promise<SessionResponse> {
  const body: Record<string, unknown> = { kind: config.kind };
  if (config.kind === 'night' && config.choice) {
    if (config.choice.mode === 'ferber') {
      body.ferber = { nightNumber: config.choice.nightNumber };
    } else if (config.choice.mode === 'chair') {
      body.chair = true;
    }
  }
  if (config.timestamp) body.timestamp = toLocalISO(config.timestamp);

  const resp = await checkResponse(await fetch(`${API}/session/start`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }));
  return resp.json();
}

export async function postUndo(): Promise<SessionResponse> {
  const resp = await checkResponse(await fetch(`${API}/session/undo`, { method: 'POST' }));
  return resp.json();
}

export async function getCycles(): Promise<{ cycles: CycleSummary[]; window: number }> {
  const resp = await checkResponse(await fetch(`${API}/cycles`));
  return resp.json();
}

export async function getCycleDetail(sessionId: number): Promise<CycleDetail> {
  const resp = await checkResponse(await fetch(`${API}/cycles/${sessionId}`));
  return resp.json();
}
