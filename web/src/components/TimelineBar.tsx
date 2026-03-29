import { TimelineEntry } from '../api';
import { STATE_COLORS } from '../constants';

interface Props {
  timeline: TimelineEntry[];
  totalDurationNs: number;
}

const LEGEND = [
  { key: 'sleeping_crib', label: 'Crib' },
  { key: 'sleeping_stroller', label: 'Stroller' },
  { key: 'sleeping_on_me', label: 'On Me' },
  { key: 'feeding', label: 'Feed' },
  { key: 'awake', label: 'Awake' },
  { key: 'resettling', label: 'Resettle' },
];

export function TimelineBar({ timeline, totalDurationNs }: Props) {
  const totalMs = totalDurationNs / 1e6;

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
        {LEGEND.map(({ key, label }) => (
          <div key={key} class="legend-item">
            <div class="legend-dot" style={{ background: STATE_COLORS[key] }} />
            {label}
          </div>
        ))}
      </div>
    </div>
  );
}
