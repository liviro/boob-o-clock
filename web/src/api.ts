const API = '/api';

export interface SessionResponse {
  state: string;
  validActions: string[];
  nightId: number | null;
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
  totalAwakeTime: number;
  feedCount: number;
  wakeCount: number;
  longestSleepBlock: number;
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

export interface NightDetail {
  night: { id: number; startedAt: string; endedAt?: string };
  events: EventEntry[];
  timeline: TimelineEntry[];
  stats: NightStats;
}

export async function getCurrentSession(): Promise<SessionResponse> {
  const resp = await fetch(`${API}/session/current`);
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

  const resp = await fetch(`${API}/session/event`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  if (!resp.ok) {
    const err = await resp.json();
    throw new Error(err.error || 'Request failed');
  }
  return resp.json();
}

export async function postUndo(): Promise<SessionResponse> {
  const resp = await fetch(`${API}/session/undo`, { method: 'POST' });
  if (!resp.ok) {
    const err = await resp.json();
    throw new Error(err.error || 'Undo failed');
  }
  return resp.json();
}

export async function getNights(): Promise<{ nights: NightSummary[] }> {
  const resp = await fetch(`${API}/nights`);
  return resp.json();
}

export async function getNightDetail(id: number): Promise<NightDetail> {
  const resp = await fetch(`${API}/nights/${id}`);
  return resp.json();
}
