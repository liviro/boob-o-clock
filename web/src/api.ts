const API = '/api';

// Mirrors internal/domain.State. Keep in sync with internal/domain/states.go.
export type State =
  | 'night_off'
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
  | 'check_in';

export interface SessionResponse {
  state: State;
  validActions: string[];
  nightId: number | null;
  suggestBreast?: string;
  currentBreast?: string;
  lastFeedStartedAt?: string;
  lastEvent: {
    action: string;
    fromState: string;
    toState: string;
    metadata?: Record<string, string>;
    timestamp: string;
  } | null;
  // Present when the current night is a Ferber night.
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
  // Present on NightOff when a recent Ferber sequence exists.
  suggestFerberNight?: number;
}

export interface NightSummary {
  id: number;
  startedAt: string;
  endedAt?: string;
  ferberEnabled?: boolean;
  ferberNightNumber?: number | null;
  stats: NightStats;
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

export interface NightStats {
  nightDuration: number;
  totalSleepTime: number;
  totalFeedTime: number;
  feedTimeLeft: number;
  feedTimeRight: number;
  totalAwakeTime: number;
  feedCount: number;
  wakeCount: number;
  longestSleepBlock: number;
  sleepBlocks: number[];
  feedTimes: string[] | null;
  realBedtime?: string | null;
  ferber?: FerberStats;
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

export interface TrendPoint {
  date: string;
  longestSleep: number;
  totalSleep: number;
  totalFeed: number;
  feedTimeLeft: number;
  feedTimeRight: number;
  wakeCount: number;
  feedCount: number;
  avgLongestSleep: number | null;
  avgTotalSleep: number | null;
  avgTotalFeed: number | null;
  avgFeedTimeLeft: number | null;
  avgFeedTimeRight: number | null;
  avgWakeCount: number | null;
  avgFeedCount: number | null;
  ferberCryTime?: number | null;
  ferberCheckIns?: number | null;
  ferberTimeToSettle?: number | null;
}

export interface NightDetail {
  night: {
    id: number;
    startedAt: string;
    endedAt?: string;
    ferberEnabled?: boolean;
    ferberNightNumber?: number | null;
  };
  events: EventEntry[];
  timeline: TimelineEntry[];
  stats: NightStats;
}

/** Format a Date as RFC3339 with local timezone offset (not UTC). */
function toLocalISO(d: Date): string {
  // getTimezoneOffset() returns minutes *behind* UTC, but ISO 8601 wants minutes *ahead*
  const off = -d.getTimezoneOffset();
  const sign = off >= 0 ? '+' : '-';
  const hh = String(Math.floor(Math.abs(off) / 60)).padStart(2, '0');
  const mm = String(Math.abs(off) % 60).padStart(2, '0');
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
    `T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}` +
    `${sign}${hh}:${mm}`;
}

async function checkResponse(resp: Response): Promise<Response> {
  if (!resp.ok) {
    const err = await resp.json().catch(() => ({}));
    throw new Error(err.error || `Request failed (${resp.status})`);
  }
  return resp;
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

export interface StartNightConfig {
  ferber?: { nightNumber: number };
  timestamp?: Date;
}

export async function postStartNight(config: StartNightConfig): Promise<SessionResponse> {
  const body: Record<string, unknown> = {};
  if (config.ferber) body.ferber = config.ferber;
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

export async function getNights(): Promise<{ nights: NightSummary[] }> {
  const resp = await checkResponse(await fetch(`${API}/nights`));
  return resp.json();
}

export async function getNightDetail(id: number): Promise<NightDetail> {
  const resp = await checkResponse(await fetch(`${API}/nights/${id}`));
  return resp.json();
}

export async function getTrends(): Promise<{ trends: TrendPoint[]; window: number }> {
  const resp = await checkResponse(await fetch(`${API}/trends`));
  return resp.json();
}
