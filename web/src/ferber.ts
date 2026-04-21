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

export const MOOD_LABELS: Record<Mood, { emoji: string; word: string }> = {
  quiet:  { emoji: '🙂', word: 'Quiet' },
  fussy:  { emoji: '😣', word: 'Fussing' },
  crying: { emoji: '😭', word: 'Crying' },
};

export function otherMoods(current: Mood): [Mood, Mood] {
  const others = MOODS.filter(m => m !== current) as Mood[];
  return [others[0], others[1]];
}

export function moodWord(m?: string): string | undefined {
  return m && m in MOOD_LABELS ? MOOD_LABELS[m as Mood].word : undefined;
}
