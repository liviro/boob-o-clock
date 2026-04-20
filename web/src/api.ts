const API = '/api';

export interface SessionResponse {
  state: string;
  validActions: string[];
  nightId: number | null;
  ferberEnabled?: boolean;
  ferberNightNumber?: number | null;
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
}

export interface NightSummary {
  id: number;
  startedAt: string;
  endedAt?: string;
  ferberEnabled?: boolean;
  ferberNightNumber?: number | null;
  stats: NightStats;
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
}

export interface NightDetail {
  night: { id: number; startedAt: string; endedAt?: string };
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

export interface FerberDefaults {
  enabled: boolean;
  nightNumber: number;
}

export async function getFerberDefaults(): Promise<FerberDefaults> {
  const resp = await fetch(`${API}/ferber/defaults`);
  if (!resp.ok) throw new Error(`GET ${API}/ferber/defaults: ${resp.status}`);
  return resp.json();
}
