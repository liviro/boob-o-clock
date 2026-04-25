export const STATE_INFO: Record<string, { icon: string; label: string }> = {
  night_off:         { icon: '🌙', label: 'Night Off' },
  // Night subgraph
  awake:             { icon: '👀', label: 'Awake' },
  feeding:           { icon: '🍼', label: 'Feeding' },
  sleeping_on_me:    { icon: '🤱', label: 'Sleeping on Me' },
  transferring:      { icon: '🤞', label: 'Transferring...' },
  resettling:        { icon: '🤚', label: 'Resettling' },
  sleeping_crib:     { icon: '😴', label: 'Sleeping in Crib' },
  strolling:         { icon: '🚶', label: 'Strolling' },
  sleeping_stroller: { icon: '💤', label: 'Sleeping in Stroller' },
  self_soothing:     { icon: '🤫', label: 'Self-Soothing' },
  poop:              { icon: '💩', label: 'Diaper Change' },
  learning:          { icon: '🌱', label: 'Learning' },
  check_in:          { icon: '👣', label: 'Checking In' },
  chair:             { icon: '🪑', label: 'Chair' },
  // Day subgraph
  day_awake:         { icon: '👀', label: 'Awake' },
  day_feeding:       { icon: '🍼', label: 'Feeding' },
  day_sleeping:      { icon: '💤', label: 'Napping' },
  day_poop:          { icon: '💩', label: 'Diaper Change' },
};

export interface ActionDef {
  icon: string;
  label: string;
  cls: string;
  needsBreast?: boolean;
  needsMood?: boolean;
  needsLocation?: boolean;
  confirm?: boolean;
}

export const ACTION_INFO: Record<string, ActionDef> = {
  // Session-creation actions (routed to POST /api/session/start, not /event).
  start_night:            { icon: '🌙', label: 'Start night',         cls: 'primary full-width' },
  start_day:              { icon: '☀️', label: 'Start day',           cls: 'primary full-width' },
  // Feeding cluster (shared between night and day).
  start_feed:             { icon: '🍼', label: 'Feed',                cls: 'feed full-width', needsBreast: true },
  dislatch_awake:         { icon: '👀', label: 'Dislatch (awake)',    cls: '' },
  dislatch_asleep:        { icon: '😴', label: 'Dislatch (asleep)',   cls: 'sleep' },
  switch_breast:          { icon: '🔄', label: 'Switch side',         cls: 'feed' },
  // Night transitions.
  start_transfer:         { icon: '🤞', label: 'Transfer to crib',    cls: '' },
  transfer_success:       { icon: '😴', label: 'Asleep in crib!',     cls: 'sleep' },
  transfer_need_resettle: { icon: '🤚', label: 'Needs resettle',      cls: '' },
  transfer_failed:        { icon: '❌', label: 'Transfer failed',     cls: 'danger' },
  start_resettle:         { icon: '🤚', label: 'Resettle',            cls: 'full-width' },
  settled:                { icon: '😴', label: 'Settled!',             cls: 'sleep' },
  resettle_failed:        { icon: '❌', label: 'Resettle failed',     cls: 'danger' },
  baby_woke:              { icon: '👀', label: 'Baby woke',           cls: 'danger' },
  start_strolling:        { icon: '🚶', label: 'Stroller',            cls: '' },
  fell_asleep:            { icon: '💤', label: 'Fell asleep!',        cls: 'sleep' },
  give_up:                { icon: '🏳️', label: 'Give up',            cls: 'danger' },
  put_down_awake:         { icon: '🙌', label: 'Put down awake',      cls: 'full-width' },
  baby_stirred:           { icon: '🤫', label: 'Baby stirred',        cls: '' },
  // Day-specific.
  start_sleep:            { icon: '😴', label: 'Nap',                 cls: 'sleep full-width', needsLocation: true },
  // Shared poop.
  poop_start:             { icon: '💩', label: 'Poop!',               cls: '' },
  poop_done:              { icon: '✅', label: 'Diaper change done',  cls: 'primary full-width' },
  // Ferber.
  put_down_awake_ferber:  { icon: '🌱', label: 'Put down awake',      cls: 'full-width', needsMood: true },
  baby_stirred_ferber:    { icon: '🌱', label: 'Baby stirred',        cls: '',           needsMood: true },
  mood_change:            { icon: '😐', label: 'Mood',                cls: '' },
  check_in:               { icon: '👣', label: 'Check in',            cls: 'primary' },
  end_check_in:           { icon: '🌱', label: 'Resume learning',     cls: '' },
  exit_ferber:            { icon: '🏳️', label: 'Give up',            cls: 'danger' },
  // Chair.
  sit_chair:              { icon: '🪑', label: 'Sit in chair',        cls: 'full-width' },
  exit_chair:             { icon: '🏳️', label: 'Give up',            cls: 'danger' },
};

/** Get single-line label for an action */
export function actionLabel(action: string): string {
  const ai = ACTION_INFO[action];
  return ai ? ai.label.replace(/\n/g, ' ') : action;
}

/** Format Go nanosecond duration to human readable */
export function fmtDur(ns: number): string {
  const totalMin = Math.floor(ns / 1e9 / 60);
  const h = Math.floor(totalMin / 60);
  const m = totalMin % 60;
  if (h > 0) return `${h}h${m > 0 ? ' ' + m + 'm' : ''}`;
  return `${m}m`;
}

/** Format "time ago" from a millisecond delta as "Xm ago" or "Xh Ym ago" (minute-rounded). */
export function fmtAgo(ms: number): string {
  const totalMin = Math.max(0, Math.floor(ms / 60000));
  const h = Math.floor(totalMin / 60);
  const m = totalMin % 60;
  if (h === 0) return `${m}m ago`;
  if (m === 0) return `${h}h ago`;
  return `${h}h ${m}m ago`;
}

/** Format elapsed seconds to timer display */
export function fmtTimer(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}:${pad(m)}:${pad(s)}`;
  return `${m}:${pad(s)}`;
}

function pad(n: number): string {
  return n.toString().padStart(2, '0');
}

/** Clock hour treated as the night's start. Times before this wrap to the next day. */
export const NIGHT_EPOCH_H = 18; // 6 PM

/**
 * Clock hour treated as a cycle's start. 0 = midnight: each 24h bar shows
 * one calendar day (midnight → midnight). Easier to read than a 7am boundary
 * because night sleep that extends past morning doesn't feel "split"
 * relative to a familiar calendar day — and the previous cycle's sleep tail
 * naturally prepends to the next bar, forming a continuous midnight-to-wake
 * sleep block on the left.
 */
export const CYCLE_EPOCH_H = 0;

/** Convert a timestamp to "hours since NIGHT_EPOCH_H". E.g. 9 PM = 3, 1 AM = 7. */
export function toNightHour(ts: string): number {
  const d = new Date(ts);
  let h = d.getHours() + d.getMinutes() / 60;
  if (h < NIGHT_EPOCH_H) h += 24;
  return h - NIGHT_EPOCH_H;
}

/** Format a Date as "M/D" for chart axis labels. */
export function fmtDayMonth(d: Date): string {
  return `${d.getMonth() + 1}/${d.getDate()}`;
}

/** Format a Date as "YYYY-MM-DDTHH:MM" in local time. Shared between the
 *  datetime-local picker input and the RFC3339 builder. */
export function fmtLocalYMDHM(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** State colors for timeline segments */
export const STATE_COLORS: Record<string, string> = {
  awake: '#7a3030',
  feeding: '#a09020',
  sleeping_on_me: '#3535a0',
  sleeping_crib: '#2060a0',
  sleeping_stroller: '#207080',
  resettling: '#6a40a0',
  strolling: '#408040',
  transferring: '#666',
  self_soothing: '#4a6090',
  poop: '#8a6030',
  learning: '#5a8060',
  check_in: '#888888',
  chair: '#9a5a7a',

  // Day subgraph. Awake/feeding/poop reuse night colors so a 24h timeline
  // bar shows visually continuous AWAKE / feeding spans across chain
  // boundaries. DaySleeping gets a distinct teal to separate naps from
  // the night's blue sleep family.
  day_awake: '#7a3030',
  day_feeding: '#a09020',
  day_sleeping: '#408080',
  day_poop: '#8a6030',
};

export const LOCATION_LABELS: Record<string, { icon: string; label: string }> = {
  crib:     { icon: '🛏️', label: 'Crib' },
  stroller: { icon: '🍃', label: 'Stroller' },
  on_me:    { icon: '🤱', label: 'On me' },
  car:      { icon: '🚗', label: 'Car' },
};
