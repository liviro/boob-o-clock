import { EventEntry, SessionMeta } from '../api';
import { STATE_COLORS, CYCLE_EPOCH_H, TIMELINE_MIN_SEGMENT_PCT } from '../constants';

interface Props {
  // Sessions that may contain this row's events — used only to cap the final
  // segment (a closed session stops at its ended_at; an open one at `now`).
  // Pass every session in view; the bar finds the one holding the last event.
  sessions: SessionMeta[];
  events: EventEntry[];
  // Events from earlier days — lets the bar render state inherited across
  // midnight (e.g., sleep trailing into the morning). The bar keeps only the
  // single most-recent one as a left-edge seed.
  prevEvents?: EventEntry[];
  // The day this row represents; the bar anchors its 24h window at this day's
  // local midnight. Lets a row render for a day with no session of its own.
  anchorDateIso?: string;
  label?: string;
}

const CYCLE_DURATION_MS = 24 * 60 * 60 * 1000;

// Prev-day events older than this are treated as stale (migration gap or
// long pause) — don't prepend, show blank rather than fabricate state.
const PREV_SEED_LOOKBACK_MS = 12 * 60 * 60 * 1000;

export function CycleTimelineBar({ sessions, events, prevEvents, anchorDateIso, label }: Props) {
  const bar = buildSegments(sessions, events, prevEvents, anchorDateIso);

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
  sessions: SessionMeta[],
  events: EventEntry[],
  prevEvents: EventEntry[] | undefined,
  anchorDateIsoOverride: string | undefined,
): BarData {
  const anchorDateIso = anchorDateIsoOverride ?? sessions[0]?.startedAt;
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
      segEndMs = resolveFinalSegmentEnd(evt, sessions, cycleEndMs);
    }

    const start = Math.max(segStartMs, cycleStartMs);
    const end = Math.min(segEndMs, cycleEndMs);
    if (end <= start) continue;

    const leftPct = ((start - cycleStartMs) / CYCLE_DURATION_MS) * 100;
    const widthPct = ((end - start) / CYCLE_DURATION_MS) * 100;
    if (widthPct < TIMELINE_MIN_SEGMENT_PCT) continue;

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

// resolveFinalSegmentEnd picks the end time for the last event's segment by
// finding the session that contains it: its ended_at if closed (which
// buildSegments then clips to this row's midnight — so a session spanning into
// tomorrow paints to midnight here and the next day's row continues it), or
// `now` capped at the cycle boundary if in-progress. A closed session that
// genuinely ended mid-day stops at ended_at, leaving an honest blank after.
function resolveFinalSegmentEnd(
  evt: EventEntry,
  sessions: SessionMeta[],
  cycleEndMs: number,
): number {
  const evtTs = new Date(evt.timestamp).getTime();
  for (const s of sessions) {
    const startMs = new Date(s.startedAt).getTime();
    const endMs = s.endedAt ? new Date(s.endedAt).getTime() : Number.POSITIVE_INFINITY;
    if (evtTs >= startMs && evtTs < endMs) {
      return s.endedAt ? new Date(s.endedAt).getTime() : Math.min(Date.now(), cycleEndMs);
    }
  }
  return cycleEndMs;
}
