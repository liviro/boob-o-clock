import { EventEntry, SessionMeta } from '../api';
import { STATE_COLORS, CYCLE_EPOCH_H } from '../constants';

interface Props {
  day: SessionMeta | null;
  night: SessionMeta | null;
  events: EventEntry[];
  // Previous cycle's events — lets the bar render state inherited across
  // the cycle boundary (e.g., sleep trailing into the morning).
  prevEvents?: EventEntry[];
  // Overrides the day/night-derived anchor; lets a row render without a session.
  anchorDateIso?: string;
  label?: string;
}

const CYCLE_DURATION_MS = 24 * 60 * 60 * 1000;

// Prev cycle events older than this are treated as stale (migration gap or
// long pause) — don't prepend, show blank rather than fabricate state.
const PREV_SEED_LOOKBACK_MS = 12 * 60 * 60 * 1000;

export function CycleTimelineBar({ day, night, events, prevEvents, anchorDateIso, label }: Props) {
  const bar = buildSegments(day, night, events, prevEvents, anchorDateIso);

  return (
    <div class="cycle-timeline-row">
      {label && <div class="cycle-timeline-label">{label}</div>}
      <div class="cycle-timeline-bar">
        {bar.segments.map((seg, i) => (
          <div
            key={i}
            class="tl-segment"
            style={{
              left: `${seg.leftPct.toFixed(2)}%`,
              width: `${seg.widthPct.toFixed(2)}%`,
              background: STATE_COLORS[seg.state] || '#333',
            }}
            title={seg.state}
          />
        ))}
      </div>
    </div>
  );
}

interface Segment {
  state: string;
  leftPct: number;
  widthPct: number;
}

interface BarData {
  segments: Segment[];
}

function buildSegments(
  day: SessionMeta | null,
  night: SessionMeta | null,
  events: EventEntry[],
  prevEvents: EventEntry[] | undefined,
  anchorDateIsoOverride: string | undefined,
): BarData {
  const anchorDateIso = anchorDateIsoOverride ?? day?.startedAt ?? night?.startedAt;
  if (!anchorDateIso) return { segments: [] };

  const cycleStart = new Date(anchorDateIso);
  cycleStart.setHours(CYCLE_EPOCH_H, 0, 0, 0);
  const cycleStartMs = cycleStart.getTime();
  const cycleEndMs = cycleStartMs + CYCLE_DURATION_MS;

  const renderEvents = events.slice();
  if (prevEvents && prevEvents.length > 0) {
    renderEvents.unshift(...prevEventTailFromCycleStart(prevEvents, cycleStartMs));
  }

  if (renderEvents.length === 0) return { segments: [] };

  const segments: Segment[] = [];
  for (let i = 0; i < renderEvents.length; i++) {
    const evt = renderEvents[i];
    const segStartMs = new Date(evt.timestamp).getTime();
    let segEndMs: number;
    if (i + 1 < renderEvents.length) {
      segEndMs = new Date(renderEvents[i + 1].timestamp).getTime();
    } else {
      segEndMs = resolveFinalSegmentEnd(evt, day, night, cycleEndMs);
    }

    const start = Math.max(segStartMs, cycleStartMs);
    const end = Math.min(segEndMs, cycleEndMs);
    if (end <= start) continue;

    const leftPct = ((start - cycleStartMs) / CYCLE_DURATION_MS) * 100;
    const widthPct = ((end - start) / CYCLE_DURATION_MS) * 100;
    if (widthPct < 0.1) continue;

    segments.push({
      state: evt.toState,
      leftPct,
      widthPct,
    });
  }

  return { segments };
}

// Returns the prefix of prevEvents to prepend: the seed event (latest one at
// or before cycleStart, if fresh enough to represent real state continuity)
// plus any events that fired after cycleStart.
function prevEventTailFromCycleStart(prevEvents: EventEntry[], cycleStartMs: number): EventEntry[] {
  let firstAfterIdx = prevEvents.length;
  for (let i = 0; i < prevEvents.length; i++) {
    if (new Date(prevEvents[i].timestamp).getTime() > cycleStartMs) {
      firstAfterIdx = i;
      break;
    }
  }
  const seedIdx = firstAfterIdx - 1;
  if (seedIdx >= 0) {
    const seedTs = new Date(prevEvents[seedIdx].timestamp).getTime();
    if (cycleStartMs - seedTs <= PREV_SEED_LOOKBACK_MS) {
      return prevEvents.slice(seedIdx);
    }
  }
  return prevEvents.slice(firstAfterIdx);
}

// resolveFinalSegmentEnd picks the end time for the last event's segment:
// the containing session's ended_at (if closed), now (if in-progress), or
// the cycle boundary as a last resort.
function resolveFinalSegmentEnd(
  evt: EventEntry,
  day: SessionMeta | null,
  night: SessionMeta | null,
  cycleEndMs: number,
): number {
  const evtTs = new Date(evt.timestamp).getTime();
  for (const s of [day, night]) {
    if (!s) continue;
    const startMs = new Date(s.startedAt).getTime();
    const endMs = s.endedAt ? new Date(s.endedAt).getTime() : Number.POSITIVE_INFINITY;
    if (evtTs >= startMs && evtTs < endMs) {
      return s.endedAt ? new Date(s.endedAt).getTime() : Math.min(Date.now(), cycleEndMs);
    }
  }
  return cycleEndMs;
}
