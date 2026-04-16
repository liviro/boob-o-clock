export const STATE_INFO: Record<string, { icon: string; label: string }> = {
  night_off:         { icon: '🌙', label: 'Night Off' },
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
};

export interface ActionDef {
  icon: string;
  label: string;
  cls: string;
  needsBreast?: boolean;
  confirm?: boolean;
}

export const ACTION_INFO: Record<string, ActionDef> = {
  start_night:            { icon: '🌙', label: 'Start Night',         cls: 'primary full-width' },
  start_feed:             { icon: '🍼', label: 'Feed',                cls: 'feed', needsBreast: true },
  dislatch_awake:         { icon: '👀', label: 'Dislatch\n(awake)',   cls: '' },
  dislatch_asleep:        { icon: '😴', label: 'Dislatch\n(asleep)',  cls: 'sleep' },
  switch_breast:          { icon: '🔄', label: 'Switch\nSide',        cls: 'feed' },
  start_transfer:         { icon: '🤞', label: 'Transfer\nto Crib',   cls: '' },
  transfer_success:       { icon: '😴', label: 'Asleep\nin Crib!',    cls: 'sleep' },
  transfer_need_resettle: { icon: '🤚', label: 'Needs\nResettle',     cls: '' },
  transfer_failed:        { icon: '❌', label: 'Transfer\nFailed',    cls: 'danger' },
  start_resettle:         { icon: '🤚', label: 'Resettle',            cls: '' },
  settled:                { icon: '😴', label: 'Settled!',             cls: 'sleep' },
  resettle_failed:        { icon: '❌', label: 'Resettle\nFailed',    cls: 'danger' },
  baby_woke:              { icon: '👀', label: 'Baby\nWoke',          cls: 'danger' },
  start_strolling:        { icon: '🚶', label: 'Stroller',            cls: '' },
  fell_asleep:            { icon: '💤', label: 'Fell\nAsleep!',       cls: 'sleep' },
  give_up:                { icon: '🏳️', label: 'Give Up',            cls: 'danger' },
  put_down_awake:         { icon: '🙌', label: 'Put Down\nAwake',     cls: '' },
  baby_stirred:           { icon: '🤫', label: 'Baby\nStirred',       cls: '' },
  poop_start:             { icon: '💩', label: 'Poop!',               cls: '' },
  poop_done:              { icon: '✅', label: 'Diaper\nChange Done', cls: 'primary full-width' },
  end_night:              { icon: '☀️', label: 'End Night',           cls: 'danger full-width', confirm: true },
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

/** Convert a timestamp to "hours since NIGHT_EPOCH_H". E.g. 9 PM = 3, 1 AM = 7. */
export function toNightHour(ts: string): number {
  const d = new Date(ts);
  let h = d.getHours() + d.getMinutes() / 60;
  if (h < NIGHT_EPOCH_H) h += 24;
  return h - NIGHT_EPOCH_H;
}

/** Format a night-hour back to a clock time string (e.g. "9 PM"). */
export function fmtHour(nightHour: number): string {
  let h = Math.round(nightHour + NIGHT_EPOCH_H);
  if (h >= 24) h -= 24;
  const ampm = h >= 12 ? 'PM' : 'AM';
  const display = h === 0 ? 12 : h > 12 ? h - 12 : h;
  return `${display} ${ampm}`;
}

/** Format a Date as "M/D" for chart axis labels. */
export function fmtDayMonth(d: Date): string {
  return `${d.getMonth() + 1}/${d.getDate()}`;
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
};
