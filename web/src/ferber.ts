// Classic Ferber graduated-extinction interval table (minutes).
// Rows are nights (1-indexed); columns are check-in numbers within a session.
// Night 8+ uses Night 7's row; check-in 4+ uses the third column.
const FERBER_INTERVALS_MIN: Record<number, readonly [number, number, number]> = {
  1: [3,  5,  10],
  2: [5,  10, 12],
  3: [10, 12, 15],
  4: [12, 15, 17],
  5: [15, 17, 20],
  6: [17, 20, 25],
  7: [20, 25, 30],
};

export function intervalMinutes(nightNumber: number, checkInNumber: number): number {
  const row = FERBER_INTERVALS_MIN[Math.min(Math.max(nightNumber, 1), 7)];
  const col = Math.min(Math.max(checkInNumber, 1), 3) - 1;
  return row[col];
}

export const MOODS = ['quiet', 'fussy', 'crying'] as const;
export type Mood = typeof MOODS[number];

export function otherMoods(current: Mood): [Mood, Mood] {
  const others = MOODS.filter(m => m !== current) as Mood[];
  return [others[0], others[1]];
}

export function moodWord(m?: string): string | undefined {
  switch (m) {
    case 'quiet':  return 'Quiet';
    case 'fussy':  return 'Fussing';
    case 'crying': return 'Crying';
    default:       return undefined;
  }
}

export interface FerberSessionContext {
  /** The Ferber session's entry event timestamp (ISO string). */
  sessionStartIso: string;
  /** Timestamp of the last relevant event for timer derivation (ISO).
   *  Either the LEARNING entry or the most recent end_check_in. */
  lastTickIso: string;
  /** Number of check-ins so far within this session. */
  checkInCount: number;
  /** Current mood in LEARNING (undefined while in CHECK_IN). */
  currentMood: Mood;
  /** The night's Ferber night number (for interval lookup). */
  nightNumber: number;
}

/** Seconds until the next check-in becomes available. Negative when past due. */
export function secondsUntilNextCheckIn(ctx: FerberSessionContext, nowMs: number): number {
  const intervalSec = intervalMinutes(ctx.nightNumber, ctx.checkInCount + 1) * 60;
  const elapsedSec = (nowMs - new Date(ctx.lastTickIso).getTime()) / 1000;
  return Math.round(intervalSec - elapsedSec);
}
