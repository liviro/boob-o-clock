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
