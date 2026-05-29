import type { State } from './api';

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
  // For actions whose location requirement depends on the current state
  // (e.g. dislatch_asleep needs a target only on the day side; at night
  // the destination state already encodes location).
  needsLocationFrom?: State[];
  confirm?: boolean;
}

export const ACTION_INFO: Record<string, ActionDef> = {
  // Session-creation actions (routed to POST /api/session/start, not /event).
  start_night:            { icon: '🌙', label: 'Start night',         cls: 'primary full-width' },
  start_day:              { icon: '☀️', label: 'Start day',           cls: 'primary full-width' },
  // Feeding cluster (shared between night and day).
  start_feed:             { icon: '🍼', label: 'Feed',                cls: 'feed full-width', needsBreast: true },
  dislatch_awake:         { icon: '👀', label: 'Dislatch (awake)',    cls: '' },
  dislatch_asleep:        { icon: '😴', label: 'Dislatch (asleep)',   cls: 'sleep', needsLocationFrom: ['day_feeding'] },
  switch_breast:          { icon: '🔄', label: 'Switch side',         cls: 'feed' },
  // Night transitions.
  start_transfer:         { icon: '🤞', label: 'Transfer to crib',    cls: '' },
  transfer_success:       { icon: '😴', label: 'Asleep in crib!',     cls: 'sleep' },
  transfer_need_resettle: { icon: '🤚', label: 'Needs resettle',      cls: '' },
  transfer_failed:        { icon: '❌', label: 'Transfer failed',     cls: 'danger' },
  start_resettle:         { icon: '🤚', label: 'Resettle',            cls: 'full-width' },
  settled:                { icon: '😴', label: 'Settled!',             cls: 'sleep' },
  resettle_failed:        { icon: '❌', label: 'Resettle failed',     cls: 'danger' },
  self_soothe_failed:     { icon: '👀', label: 'Still not sleeping',  cls: 'danger' },
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

/** Format an ISO timestamp as a localized "HH:MM AM/PM" clock time. */
export function fmtClockTime(iso: string): string {
  return new Date(iso).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
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

/**
 * Minimum width (as a percent of the bar's total span) for a timeline segment
 * to be rendered. Shared by TimelineBar (per-night) and CycleTimelineBar
 * (24h) so the two views drop the same fraction of sub-pixel transitions —
 * otherwise short feeds/transfers/resettles silently disappear from one bar
 * but not the other.
 */
export const TIMELINE_MIN_SEGMENT_PCT = 0.1;

/** Convert a timestamp to "hours since NIGHT_EPOCH_H". E.g. 9 PM = 3, 1 AM = 7. */
export function toNightHour(ts: string): number {
  const d = new Date(ts);
  let h = d.getHours() + d.getMinutes() / 60;
  if (h < NIGHT_EPOCH_H) h += 24;
  return h - NIGHT_EPOCH_H;
}

/** Format an "hours since NIGHT_EPOCH_H" value as a 12-hour clock label. */
export function fmtEpochHour(h: number): string {
  let clock = Math.round(h + NIGHT_EPOCH_H);
  if (clock >= 24) clock -= 24;
  if (clock < 0) clock += 24;
  const period = clock < 12 ? 'AM' : 'PM';
  const hour12 = clock === 0 ? 12 : clock > 12 ? clock - 12 : clock;
  return `${hour12} ${period}`;
}

/** Format a Date as a short numeric day/month for chart axis labels. */
export function fmtDayMonth(d: Date): string {
  return d.toLocaleDateString(undefined, { month: 'numeric', day: 'numeric' });
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

// Typical chain-transition hours: a night should end around the morning, a day
// around bedtime. Shared by the split-default picker and the "unusually long"
// heuristic so both speak the same language.
export const NIGHT_TRANSITION_HOUR = 7;  // morning wake-up
export const DAY_TRANSITION_HOUR = 20;   // bedtime

// How long the open session may run past its expected transition before the
// Start-day/night nudge fires (e.g. in night mode past ~1pm). Absorbs a
// slept-in or slightly-late morning.
export const OVERRUN_SLACK_HOURS = 6;

// nextOccurrence returns the first local time at `hour`:00 strictly after
// `after`. Used to pre-fill the split sheet's datetime picker at the typical
// transition time (7am morning wake-up for a night, 8pm bedtime for a day).
// It points at the FIRST missed transition regardless of how long the session
// has been open; iterating splits cascades naturally because each trailing
// session has its own start time.
//
// Test cases (validated by inspection; ready to port if Vitest is added):
//   nextOccurrence(7,  Mon 21:00) === Tue 07:00
//   nextOccurrence(7,  Mon 05:00) === Mon 07:00   (same day — 7am still ahead)
//   nextOccurrence(7,  Mon 07:00) === Tue 07:00   (strict-after, not at-or-after)
//   nextOccurrence(7,  Mon 06:59) === Mon 07:00
//   nextOccurrence(20, Mon 07:00) === Mon 20:00
//   nextOccurrence(20, Mon 21:00) === Tue 20:00   (today's 8pm already passed)
//   nextOccurrence(20, Mon 20:00) === Tue 20:00
//   nextOccurrence(7,  Tue 07:00:01) === Wed 07:00  (iterative cascade)
//   Timezone: result keeps the same local hour in the input's local zone.
export function nextOccurrence(hour: number, after: Date): Date {
  const candidate = new Date(after);
  candidate.setHours(hour, 0, 0, 0);
  if (candidate.getTime() <= after.getTime()) {
    candidate.setDate(candidate.getDate() + 1);
  }
  return candidate;
}

// isSessionUnusuallyLong reports whether a session SWALLOWED a full opposite
// period — i.e. it ran more than one whole daytime (for a night) or nighttime
// (for a day) past its first expected transition. That's the "there's a clean
// period to peel off the front → offer Split" signal.
//
// `firstT = nextOccurrence(transitionHour, start)` is the first morning (night)
// or bedtime (day) after the start. A night must run >13h past it (8pm−7am, a
// full daytime); a day >11h (a full nighttime). Measuring from firstT is self-
// aligning: a split's morning-started trailing has its firstT pushed ~24h out,
// so `end − firstT` stays small and it is NOT flagged — splitting it again
// can't recover a clean period, so we don't offer to. Works for open (end=now)
// and closed sessions alike.
export function isSessionUnusuallyLong(isNight: boolean, startedAt: Date, end: Date): boolean {
  const hour = isNight ? NIGHT_TRANSITION_HOUR : DAY_TRANSITION_HOUR;
  const firstT = nextOccurrence(hour, startedAt);
  const oppositePeriodHours = isNight
    ? DAY_TRANSITION_HOUR - NIGHT_TRANSITION_HOUR        // a full daytime (13h)
    : 24 - DAY_TRANSITION_HOUR + NIGHT_TRANSITION_HOUR;  // a full nighttime (11h)
  return end.getTime() - firstT.getTime() > oppositePeriodHours * 3_600_000;
}

// shouldNudgeModeSwitch reports whether the OPEN current session is stuck in the
// wrong mode for the wall clock and has overrun — the Start-day/night nudge.
// Two conditions: (1) it ran past its expected transition by > slack, and (2)
// the current clock phase disagrees with the session's kind (a night during
// daytime → start day; a day during nighttime → start night). The clock check
// keeps the nudge from pointing the wrong way when a session has wrapped back
// into its own phase (e.g. a night that ate a full day, viewed at 11pm).
export function shouldNudgeModeSwitch(isNight: boolean, startedAt: Date, now: Date): boolean {
  const hour = isNight ? NIGHT_TRANSITION_HOUR : DAY_TRANSITION_HOUR;
  const firstT = nextOccurrence(hour, startedAt);
  const overran = now.getTime() - firstT.getTime() > OVERRUN_SLACK_HOURS * 3_600_000;
  if (!overran) return false;
  const isDaytimeNow = now.getHours() >= NIGHT_TRANSITION_HOUR && now.getHours() < DAY_TRANSITION_HOUR;
  return isNight ? isDaytimeNow : !isDaytimeNow;
}

// A split inserts a ~1s "degenerate" boundary session of the opposite kind
// (a day on a night-split, a night on a day-split). The frontend detects it by
// duration to label it honestly instead of rendering nonsensical zero-stats.
// Mirrors the backend's 60s degenerate cutoff in reports (averageCycles).
export const DEGENERATE_SESSION_MAX_MS = 60_000;
