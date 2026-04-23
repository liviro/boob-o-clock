import { TimelineEntry } from '../api';
import { STATE_COLORS } from '../constants';

interface Props {
  timeline: TimelineEntry[];
  totalDurationNs: number;
}

/**
 * Legend entries grouped by category: sleep, active, transitional.
 *
 * Day state variants share colors with their night counterparts (except
 * `day_sleeping`, which has its own teal). Duplicate labels are acceptable
 * here because a TimelineBar instance renders either day OR night events in
 * a given context — never both — so the filter by "present states" shows at
 * most one variant per category.
 */
const LEGEND_GROUPS: { key: string; label: string }[][] = [
  [
    { key: 'sleeping_crib', label: 'Crib' },
    { key: 'sleeping_on_me', label: 'On Me' },
    { key: 'sleeping_stroller', label: 'Stroller' },
    { key: 'day_sleeping', label: 'Nap' },
  ],
  [
    { key: 'feeding', label: 'Feed' },
    { key: 'day_feeding', label: 'Feed' },
    { key: 'awake', label: 'Awake' },
    { key: 'day_awake', label: 'Awake' },
  ],
  [
    { key: 'resettling', label: 'Resettle' },
    { key: 'self_soothing', label: 'Self-Soothe' },
    { key: 'transferring', label: 'Transfer' },
    { key: 'strolling', label: 'Stroll' },
    { key: 'poop', label: 'Diaper' },
    { key: 'day_poop', label: 'Diaper' },
  ],
];

export function TimelineBar({ timeline, totalDurationNs }: Props) {
  const totalMs = totalDurationNs / 1e6;
  const present = new Set(timeline.map(e => e.state));

  const visibleGroups = LEGEND_GROUPS
    .map(group => group.filter(({ key }) => present.has(key)))
    .filter(group => group.length > 0);

  return (
    <div>
      <div class="timeline-bar">
        {timeline.map((entry, i) => {
          const pct = totalMs > 0 ? (entry.duration / 1e6 / totalMs * 100) : 0;
          if (pct < 0.5) return null;
          return (
            <div
              key={i}
              class={`tl-segment ${entry.state}`}
              style={{ width: `${pct}%`, background: STATE_COLORS[entry.state] || '#333' }}
            />
          );
        })}
      </div>
      <div class="timeline-legend">
        {visibleGroups.map((group, gi) => (
          <div key={gi} class="legend-group">
            {group.map(({ key, label }) => (
              <div key={key} class="legend-item">
                <div class="legend-dot" style={{ background: STATE_COLORS[key] }} />
                {label}
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
