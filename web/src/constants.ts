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
  switch_breast:          { icon: '🔄', label: 'Switch\nSide',        cls: 'feed', needsBreast: true },
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
  poop_start:             { icon: '💩', label: 'Poop!',               cls: '' },
  poop_done:              { icon: '✅', label: 'Diaper\nChange Done', cls: 'primary full-width' },
  end_night:              { icon: '☀️', label: 'End Night',           cls: 'danger full-width', confirm: true },
};

/** Format Go nanosecond duration to human readable */
export function fmtDur(ns: number): string {
  const totalMin = Math.floor(ns / 1e9 / 60);
  const h = Math.floor(totalMin / 60);
  const m = totalMin % 60;
  if (h > 0) return `${h}h${m > 0 ? m + 'm' : ''}`;
  return `${m}m`;
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

/** State colors for timeline segments */
export const STATE_COLORS: Record<string, string> = {
  awake: '#5a2a2a',
  feeding: '#8a6a2a',
  sleeping_on_me: '#2a2a5a',
  sleeping_crib: '#1a3a6a',
  sleeping_stroller: '#2a4a5a',
  resettling: '#3a3a5a',
  strolling: '#3a4a3a',
  transferring: '#444',
  poop: '#5a4a2a',
};
