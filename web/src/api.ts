const API = '/api';

export interface SessionResponse {
  state: string;
  validActions: string[];
  nightId: number | null;
  suggestBreast?: string;
  currentBreast?: string;
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
  if (timestamp) body.timestamp = timestamp.toISOString();

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
