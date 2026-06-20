import { useState, useEffect, useCallback, useMemo } from 'preact/hooks';
import { getCycles, getCycleDetail, CycleStats, CycleSummary, CycleDetail, DayStats, DaySegment, NightStats, SessionMeta, EventEntry } from '../api';
import { fmtDur, fmtClockTime, toNightHour, ACTION_INFO, actionLabel, isSessionUnusuallyLong, DEGENERATE_SESSION_MAX_MS } from '../constants';
import { SplitSessionSheet } from '../components/SplitSessionSheet';
import { TimelineBar } from '../components/TimelineBar';
import { CycleTimelineBar } from '../components/CycleTimelineBar';
import { TrendChart } from '../components/TrendChart';
import { ScatterChart } from '../components/ScatterChart';
import { NightHourChart } from '../components/NightHourChart';
import { FeedDurationChart, FeedSpan } from '../components/FeedDurationChart';
import { ErrorToast } from '../components/ErrorToast';
import { useIsLandscape } from '../hooks/useIsLandscape';
import { useConfig } from '../hooks/useConfig';

type View = 'cycles' | 'trends';

const DISPLAY_LIMIT = 30;

const nsToMinutes = (ns: number) => Math.round(ns / 1e9 / 60);

const NS_PER_HOUR = 3.6e12;

// sessionUnusuallyLong gates the badge/banner: true when the session ran well
// past the transition it should have ended on. Open sessions are measured to
// now. See isSessionUnusuallyLong for why this beats a raw-duration threshold
// (it spares the degenerate-bounded trailing a split leaves behind).
function sessionUnusuallyLong(s: SessionMeta | null | undefined): boolean {
  if (!s) return false;
  const end = s.endedAt ? new Date(s.endedAt) : new Date();
  return isSessionUnusuallyLong(s.kind === 'night', new Date(s.startedAt), end);
}

// isDegenerate reports whether a session is the ~1s boundary marker a split
// inserts. Its stats are all-zero and its derived pills are nonsensical, so the
// UI replaces its body with an honest label (SplitMarkerNote) instead.
function isDegenerate(s: SessionMeta | null | undefined): boolean {
  if (!s || !s.endedAt) return false; // an open session is never degenerate
  return new Date(s.endedAt).getTime() - new Date(s.startedAt).getTime() <= DEGENERATE_SESSION_MAX_MS;
}

// SplitMarkerNote replaces the stats body of a degenerate half. The marker is
// the opposite kind of the session that was split; its events stayed in that
// overrun session (they can't cross the day/night subgraphs), so it has none
// of its own to show.
function SplitMarkerNote({ kind }: { kind: 'day' | 'night' }) {
  const overrun = kind === 'day' ? 'night' : 'day';
  return (
    <div class="split-marker-note">
      Split marker from an overrun {overrun}: the {kind}'s events stay logged under the {overrun}, so there's nothing to show here.
    </div>
  );
}

// Sum a per-side feed-time field across a cycle's day + night halves. Returns
// null when the stats block is absent or both halves are missing; a partial
// cycle still plots, treating the missing half as 0 feeds on that side.
function sumFeedSide(
  stats: CycleStats | null | undefined,
  pick: (s: DayStats | NightStats) => number,
): number | null {
  if (stats == null) return null;
  if (stats.day == null && stats.night == null) return null;
  return (stats.day ? pick(stats.day) : 0) + (stats.night ? pick(stats.night) : 0);
}

// computeMovingAvg returns the trailing `window`-wide mean at each index.
// Requires ALL values within the window to be non-null (no partial averaging)
// so the resulting line only appears where backing data is complete.
function computeMovingAvg(values: (number | null)[], window: number): (number | null)[] {
  return values.map((_, i) => {
    if (i + 1 < window) return null;
    const slice = values.slice(i + 1 - window, i + 1);
    if (slice.some(v => v === null)) return null;
    const sum = (slice as number[]).reduce((a, b) => a + b, 0);
    return sum / window;
  });
}

// Switch-breast self-transitions stay in `feeding` and are absorbed naturally
// by the forward walk. Timestamp equality is character-exact because both
// `feedTimes` and `events[].timestamp` come from the same Go time.Time
// MarshalJSON output.
function feedSpansFor(c: CycleSummary): FeedSpan[] {
  const feedTimes = c.stats.night?.feedTimes ?? [];
  const events = c.events;
  const spans: FeedSpan[] = [];
  for (const startStr of feedTimes) {
    const i = events.findIndex(e =>
      e.toState === 'feeding' && e.timestamp === startStr,
    );
    if (i < 0) continue;
    let endStr: string | null = null;
    let j = i;
    while (j < events.length && events[j].toState === 'feeding') {
      const next = events[j + 1];
      if (!next) break;
      endStr = next.timestamp;
      j++;
    }
    if (endStr == null) continue;
    spans.push({
      startHour: toNightHour(startStr),
      endHour: toNightHour(endStr),
    });
  }
  return spans;
}

export function History() {
  const [cycles, setCycles] = useState<CycleSummary[]>([]);
  const [detail, setDetail] = useState<CycleDetail | null>(null);
  const [detailId, setDetailId] = useState<number | null>(null);
  const [view, setView] = useState<View>('cycles');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const clearError = useCallback(() => setError(null), []);
  const isLandscape = useIsLandscape();

  useEffect(() => { loadCycles(); }, []);

  async function loadCycles() {
    setLoading(true);
    setDetail(null);
    try {
      const data = await getCycles();
      // API returns cycles in chronological order (oldest first). Reverse so
      // the newest is rendered at the top.
      setCycles((data.cycles || []).slice().reverse());
    } catch {
      setError('Failed to load cycles');
    } finally {
      setLoading(false);
    }
  }

  async function showDetail(sessionId: number) {
    try {
      const data = await getCycleDetail(sessionId);
      setDetail(data);
      setDetailId(sessionId);
    } catch {
      setError('Failed to load cycle details');
    }
  }

  async function refetchDetail() {
    if (detailId == null) return;
    try {
      // Refresh the detail in place AND the underlying list, so navigating Back
      // reflects the split — e.g. a still-over-long trailing surfaces as a new
      // badged cycle instead of the stale pre-split card.
      const [d, c] = await Promise.all([getCycleDetail(detailId), getCycles()]);
      setDetail(d);
      setCycles((c.cycles || []).slice().reverse());
    } catch {
      setError('Failed to reload cycle');
    }
  }

  if (loading) return <div class="no-data">Loading...</div>;

  if (detail) {
    return (
      <CycleDetailView
        detail={detail}
        onBack={() => { setDetail(null); setDetailId(null); }}
        onSplit={refetchDetail}
      />
    );
  }

  if (cycles.length === 0) {
    return <div class="no-data">No cycles recorded yet</div>;
  }

  const cyclesForList = cycles.slice(0, DISPLAY_LIMIT);
  const cyclesForCharts = isLandscape ? cycles : cycles.slice(0, DISPLAY_LIMIT);

  return (
    <div class="history-content">
      <div class="view-toggle">
        <button class={`view-btn ${view === 'cycles' ? 'active' : ''}`} onClick={() => setView('cycles')}>
          Cycles
        </button>
        <button class={`view-btn ${view === 'trends' ? 'active' : ''}`} onClick={() => setView('trends')}>
          {cycles.length > DISPLAY_LIMIT ? `Trends (${isLandscape ? '90d' : '30d'})` : 'Trends'}
        </button>
      </div>

      {view === 'cycles' && (
        <>
          {cyclesForList.map((c, idx) => (
            <CycleCard key={cardKey(c, idx)} cycle={c} onClick={() => {
              const id = c.day?.id ?? c.night?.id;
              if (id != null) showDetail(id);
            }} />
          ))}
          {cycles.length > DISPLAY_LIMIT && (
            <div class="nights-caption">Showing {DISPLAY_LIMIT} most recent cycles</div>
          )}
        </>
      )}

      {view === 'trends' && <TrendsView cycles={cyclesForCharts} />}

      <ErrorToast message={error} onDismiss={clearError} />
    </div>
  );
}

function cardKey(c: CycleSummary, idx: number): string {
  return `${c.day?.id ?? 'nil'}-${c.night?.id ?? 'nil'}-${idx}`;
}

function CycleCard({ cycle, onClick }: { cycle: CycleSummary; onClick: () => void }) {
  const features = useConfig().features;
  const anchor = cycle.day?.startedAt ?? cycle.night?.startedAt;
  if (!anchor) return null;
  const date = new Date(anchor);
  const dateStr = date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
  const day = cycle.stats.day;
  const night = cycle.stats.night;
  const isFerber = features.ferber && !!cycle.night?.ferberEnabled;
  const isChair = features.chair && !!cycle.night?.chairEnabled;

  return (
    <div class="night-card clickable" onClick={onClick}>
      <h3>
        <span>
          {dateStr}
          {isFerber && <span class="ferber-badge" title={`Night ${cycle.night?.ferberNightNumber ?? ''}`}>🌱</span>}
          {isChair && <span class="chair-badge" title="Chair night">🪑</span>}
        </span>
      </h3>
      {night && (
        <div class="cycle-section cycle-section-night">
          <div class="cycle-section-header">
            <span>🌙 Night</span>
            <span class="cycle-section-time">
              {sessionUnusuallyLong(cycle.night) && <span class="long-badge">⚠ unusually long</span>}
              {cycle.night && fmtClockTime(cycle.night.startedAt)}
            </span>
          </div>
          {isDegenerate(cycle.night) ? <SplitMarkerNote kind="night" /> : (
            <>
              <div class="night-stats">
                <Stat value={fmtDur(night.longestSleepBlock)} label="Longest Sleep" />
                <Stat value={String(night.wakeCount)} label="Wakes" />
                <Stat value={fmtDur(night.totalFeedTime)} label="Feed Time" />
                <Stat value={fmtDur(night.totalSleepTime)} label="Total Sleep" />
              </div>
              <SleepBlocksPills blocks={night.sleepBlocks} longest={night.longestSleepBlock} active={!cycle.night?.endedAt} />
              <FeedTimesPills times={night.feedTimes} />
            </>
          )}
        </div>
      )}
      {day && (
        <div class="cycle-section cycle-section-day">
          <div class="cycle-section-header">
            <span>☀️ Day</span>
            <span class="cycle-section-time">
              {sessionUnusuallyLong(cycle.day) && <span class="long-badge">⚠ unusually long</span>}
              {cycle.day && fmtClockTime(cycle.day.startedAt)}
            </span>
          </div>
          {isDegenerate(cycle.day) ? <SplitMarkerNote kind="day" /> : (
            <>
              <div class="night-stats">
                <Stat value={fmtDur(day.totalNapTime)} label="Total Nap" />
                <Stat value={String(day.daySolidsCount)} label="Solid Feeds" />
                <Stat value={fmtDur(day.dayTotalFeedTime)} label="Breastfeed Time" />
                <Stat value={String(day.dayFeedCount + day.daySolidsCount)} label="Total Day Feeds" />
              </div>
              <DayRhythmPills segments={day.daySegments} live={!cycle.day?.endedAt} />
            </>
          )}
        </div>
      )}
    </div>
  );
}

// --- Trends view ---

function TrendsView({ cycles }: { cycles: CycleSummary[] }) {
  const features = useConfig().features;
  // Spread at each chart so we don't copy these props on every chart.
  const modeProps = {
    ...(features.ferber
      ? { highlightFerber: true, isFerber: (c: CycleSummary) => !!c.night?.ferberEnabled }
      : {}),
    ...(features.chair
      ? { highlightChair: true, isChair: (c: CycleSummary) => !!c.night?.chairEnabled }
      : {}),
  };
  // Trends are indexed oldest → newest for line charts; the cycles list is
  // newest-first, so reverse.
  const chronological = [...cycles].reverse();

  // Client-side moving average for Real Bedtime. Server-side averaging of
  // timestamps is awkward (they're `*time.Time`, not durations); since bedtime
  // is only meaningful as a clock-hour value, compute the 3-cycle rolling
  // mean of hours-since-epoch here.
  const bedtimeHours = chronological.map(c =>
    c.stats.night?.realBedtime ? toNightHour(c.stats.night.realBedtime) : null,
  );
  const bedtimeAvgs = computeMovingAvg(bedtimeHours, 3);
  const bedtimeAvgByCycle = new Map<CycleSummary, number | null>();
  chronological.forEach((c, i) => bedtimeAvgByCycle.set(c, bedtimeAvgs[i]));

  return (
    <div class="trends-grid">
      <StackedCycleTimelines cycles={cycles} />

      <FeedDurationChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        getSpans={feedSpansFor}
        color="#c0b040"
        title="Intra-sleep feed times"
        {...modeProps}
      />

      <NightHourChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        getDots={c => c.stats.night?.realBedtime ? [{ hour: toNightHour(c.stats.night.realBedtime) }] : []}
        getAvgHour={c => bedtimeAvgByCycle.get(c) ?? null}
        color="#6a9aff"
        title="Real Bedtime"
        {...modeProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.day?.totalNapTime ?? null,
          getAvg: c => c.avg?.day?.totalNapTime ?? null,
          color: '#5affaa',
        }]}
        formatValue={fmtDur}
        title="Total Nap Duration"
        {...modeProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.longestSleepBlock ?? null,
          getAvg: () => null,
          color: '#4a8aff',
        }]}
        formatValue={fmtDur}
        title="Longest Sleep Block (night)"
        {...modeProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.totalSleepTime ?? null,
          getAvg: c => c.avg?.night?.totalSleepTime ?? null,
          color: '#6a5aff',
        }]}
        formatValue={fmtDur}
        title="Total Sleep (night)"
        {...modeProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.wakeCount ?? null,
          color: '#ff5a5a',
        }]}
        formatValue={v => String(Math.round(v))}
        title="Wake Count (night)"
        {...modeProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.feedCount ?? null,
          color: '#ffaa5a',
        }]}
        formatValue={v => String(Math.round(v))}
        title="Intra-sleep feed count"
        {...modeProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.intraSleepFeedTime ?? null,
          getAvg: c => c.avg?.night?.intraSleepFeedTime ?? null,
          color: '#ffaa5a',
        }]}
        formatValue={fmtDur}
        title="Total intra-sleep feed time"
        {...modeProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.intraSleepCareTime ?? null,
          getAvg: c => c.avg?.night?.intraSleepCareTime ?? null,
          color: '#5acfd0',
        }]}
        formatValue={fmtDur}
        title="Intra-sleep care time"
        {...modeProps}
      />

      <ScatterChart
        points={chronological.filter(c => c.stats.day != null && c.stats.night != null)}
        getX={c => (c.stats.day?.dayTotalFeedTime ?? 0) + (c.stats.night?.totalFeedTime ?? 0) - (c.stats.night?.intraSleepFeedTime ?? 0)}
        getY={c => c.stats.night?.intraSleepFeedTime ?? 0}
        formatX={fmtDur}
        formatY={fmtDur}
        title="Intra-sleep feeding (Y) vs. other feeds in cycle (X)"
        color="#c0b040"
      />

      {features.ferber && (
        <>
          {chronological.some(c => c.stats.night?.ferber?.cryTime != null) && (
            <TrendChart
              points={chronological.filter(c => c.stats.night?.ferber?.cryTime != null)}
              getDate={c => c.night?.startedAt ?? c.day!.startedAt}
              series={[{
                getValue: c => nsToMinutes(c.stats.night!.ferber!.cryTime),
                getAvg: () => null,
                color: '#ff5a8a',
              }]}
              formatValue={v => `${Math.round(v)}m`}
              title="🌱 Cry time per night"
            />
          )}
          {chronological.some(c => c.stats.night?.ferber?.checkIns != null) && (
            <TrendChart
              points={chronological.filter(c => c.stats.night?.ferber?.checkIns != null)}
              getDate={c => c.night?.startedAt ?? c.day!.startedAt}
              series={[{
                getValue: c => c.stats.night!.ferber!.checkIns,
                getAvg: () => null,
                color: '#a05aff',
              }]}
              formatValue={v => String(Math.round(v))}
              title="🌱 Check-ins per night"
            />
          )}
          {chronological.some(c => c.stats.night?.ferber?.avgTimeToSettle != null && c.stats.night.ferber.avgTimeToSettle > 0) && (
            <TrendChart
              points={chronological.filter(c => c.stats.night?.ferber?.avgTimeToSettle != null && c.stats.night.ferber.avgTimeToSettle > 0)}
              getDate={c => c.night?.startedAt ?? c.day!.startedAt}
              series={[{
                getValue: c => nsToMinutes(c.stats.night!.ferber!.avgTimeToSettle),
                getAvg: () => null,
                color: '#5affaa',
              }]}
              formatValue={v => `${Math.round(v)}m`}
              title="🌱 Avg time to settle"
            />
          )}
        </>
      )}

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[
          {
            getValue: c => sumFeedSide(c.stats, s => s.feedTimeLeft),
            getAvg: c => sumFeedSide(c.avg, s => s.feedTimeLeft),
            color: '#5a9aff',
            label: 'Left',
          },
          {
            getValue: c => sumFeedSide(c.stats, s => s.feedTimeRight),
            getAvg: c => sumFeedSide(c.avg, s => s.feedTimeRight),
            color: '#ff7a5a',
            label: 'Right',
          },
        ]}
        formatValue={fmtDur}
        title="Feed by side"
        {...modeProps}
      />
    </div>
  );
}

// Each cycle is handed the *previous* cycle's events (the older one, at
// index+1 since the list is newest-first), so the bar can render state
// inherited across the 7am boundary (e.g., sleep trailing from last night).
//
// Collapsed by default; tapping expands. Avoids a fixed-height scroll region
// because nested scrolling on the trends page conflicts with iOS page-scroll
// gestures.
const TIMELINE_COLLAPSED_COUNT = 10;

// The chart renders one row per CALENDAR DAY rather than per cycle. A cycle
// (day + night) normally straddles midnight, and with natural drift may run
// longer or shorter than 24h; an over-long session (forgotten Start day) can
// span several days. Bucketing events by the day they fall on — and seeding
// each day's left edge from the prior day's tail — paints all of these
// uniformly: every day owns its own midnight→midnight track, a multi-day
// session contributes to each day it touches, and there's exactly one row per
// date (so split-created degenerate days can't double up).
function StackedCycleTimelines({ cycles }: { cycles: CycleSummary[] }) {
  const [expanded, setExpanded] = useState(false);
  const { rows, sessions } = useMemo(() => buildDayRows(cycles), [cycles]);

  const canCollapse = rows.length > TIMELINE_COLLAPSED_COUNT;
  const visibleRows = canCollapse && !expanded
    ? rows.slice(0, TIMELINE_COLLAPSED_COUNT)
    : rows;
  const hiddenCount = rows.length - visibleRows.length;

  return (
    <div class="trend-chart">
      <div class="trend-title">24h Cycle Timelines</div>
      <div class="stacked-cycle-list">
        {visibleRows.map(row => (
          <CycleTimelineBar
            key={row.dayKey}
            sessions={sessions}
            events={row.events}
            prevEvents={row.prevEvents}
            anchorDateIso={row.anchorIso}
            label={row.label}
          />
        ))}
      </div>
      {canCollapse && (
        <button
          class="stacked-cycle-toggle"
          onClick={() => setExpanded(e => !e)}
        >
          {expanded ? 'Show less' : `Show ${hiddenCount} more`}
        </button>
      )}
    </div>
  );
}

function localDayKey(dateLike: string | Date): number {
  const d = typeof dateLike === 'string' ? new Date(dateLike) : dateLike;
  return d.getFullYear() * 10000 + (d.getMonth() + 1) * 100 + d.getDate();
}

// midnightOfDayKey decodes a localDayKey back into that day's local midnight.
function midnightOfDayKey(key: number): Date {
  const year = Math.floor(key / 10000);
  const month = Math.floor((key % 10000) / 100);
  const day = key % 100;
  return new Date(year, month - 1, day);
}

interface DayRow {
  dayKey: number;
  anchorIso: string;
  label: string;
  events: EventEntry[];
  prevEvents: EventEntry[];
}

// buildDayRows turns the cycle list into one row per calendar day, newest-first.
// A day gets a row if any event falls on it OR any session spans it (so a day
// fully swallowed by an over-long session — no events of its own — still draws
// its inherited sleep). Each row carries the events of that day plus, as
// prevEvents, every earlier event (the bar keeps only the most recent as a
// left-edge seed). `sessions` is shared by all rows to cap final segments.
function buildDayRows(cycles: CycleSummary[]): { rows: DayRow[]; sessions: SessionMeta[] } {
  const sessions: SessionMeta[] = [];
  for (const c of cycles) {
    if (c.day) sessions.push(c.day);
    if (c.night) sessions.push(c.night);
  }

  const allEvents = cycles
    .flatMap(c => c.events)
    .sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());

  // Bucket events by their calendar day in one pass (allEvents is sorted, so
  // each bucket stays chronological). The bucket keys seed the row set.
  const eventsByDay = new Map<number, EventEntry[]>();
  for (const e of allEvents) {
    const key = localDayKey(e.timestamp);
    const bucket = eventsByDay.get(key);
    if (bucket) bucket.push(e);
    else eventsByDay.set(key, [e]);
  }

  const dayKeys = new Set<number>(eventsByDay.keys());
  // Cover days a session spans even if no event landed on them. Open sessions
  // run to now; this never reaches into the future.
  const nowMs = Date.now();
  for (const s of sessions) {
    const endMs = s.endedAt ? new Date(s.endedAt).getTime() : nowMs;
    const cur = new Date(s.startedAt);
    cur.setHours(0, 0, 0, 0);
    while (cur.getTime() <= endMs) {
      dayKeys.add(localDayKey(cur));
      cur.setDate(cur.getDate() + 1);
    }
  }

  const rows = [...dayKeys]
    .sort((a, b) => b - a) // newest-first
    .map(dayKey => {
      const midnight = midnightOfDayKey(dayKey);
      const midnightMs = midnight.getTime();
      return {
        dayKey,
        anchorIso: midnight.toISOString(),
        label: midnight.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
        events: eventsByDay.get(dayKey) ?? [],
        prevEvents: allEvents.filter(e => new Date(e.timestamp).getTime() < midnightMs),
      };
    });

  return { rows, sessions };
}

// --- Cycle detail view ---

function CycleDetailView({ detail, onBack, onSplit }: { detail: CycleDetail; onBack: () => void; onSplit: () => void }) {
  const ferberVisible = useConfig().features.ferber;
  const { day, night } = detail.cycle;
  const [splitTarget, setSplitTarget] = useState<SessionMeta | null>(null);
  const anchor = day?.startedAt ?? night?.startedAt;
  const date = anchor ? new Date(anchor) : new Date();
  const dateStr = date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });

  const s = detail.stats;
  const dayStats = s.day;
  const nightStats = s.night;

  return (
    <div class="history-content">
      <button class="back-btn" onClick={onBack}>← Back</button>

      <div class="night-card">
        <h3>
          <span>{dateStr}</span>
        </h3>

        {night && nightStats && sessionUnusuallyLong(night) && (
          <LongSessionBanner
            kind="night"
            durationNs={nightStats.nightDuration}
            onSplit={() => setSplitTarget(night)}
          />
        )}
        {day && dayStats && sessionUnusuallyLong(day) && (
          <LongSessionBanner
            kind="day"
            durationNs={dayStats.dayDuration}
            onSplit={() => setSplitTarget(day)}
          />
        )}

        {nightStats && (
          <div class="cycle-section cycle-section-night">
            <div class="cycle-section-header">
              <span>🌙 Night</span>
              <span class="cycle-section-time">{fmtDur(nightStats.nightDuration)}</span>
            </div>
            {isDegenerate(night) ? <SplitMarkerNote kind="night" /> : (
              <>
                <div class="night-stats">
                  <Stat value={fmtDur(nightStats.longestSleepBlock)} label="Longest Sleep" />
                  <Stat value={String(nightStats.wakeCount)} label="Wakes" />
                  <Stat value={fmtDur(nightStats.totalFeedTime)} label="Feed Time" />
                  <Stat value={fmtDur(nightStats.totalSleepTime)} label="Total Sleep" />
                </div>
                {ferberVisible && nightStats.ferber && night?.ferberEnabled && (
                  <div class="ferber-stats">
                    <div class="ferber-stats-header">🌱 Night {night.ferberNightNumber}</div>
                    <div class="night-stats">
                      <Stat value={String(nightStats.ferber.sessions)} label="Sessions" />
                      <Stat value={fmtDur(nightStats.ferber.avgTimeToSettle)} label="Session average" />
                      <Stat value={fmtDur(nightStats.ferber.cryTime)} label="Cry time" />
                      <Stat value={fmtDur(nightStats.ferber.fussTime)} label="Fuss time" />
                    </div>
                    <details class="ferber-details">
                      <summary>More</summary>
                      <div class="night-stats">
                        <Stat value={String(nightStats.ferber.checkIns)} label="Check-ins" />
                        <Stat value={String(nightStats.ferber.sessionsAbandoned)} label="Abandoned" />
                        <Stat value={fmtDur(nightStats.ferber.quietTime)} label="Quiet time" />
                      </div>
                    </details>
                  </div>
                )}
                <SleepBlocksPills blocks={nightStats.sleepBlocks} longest={nightStats.longestSleepBlock} active={!night?.endedAt} />
                <FeedTimesPills times={nightStats.feedTimes} />
                {detail.timeline.length > 0 && (
                  <TimelineBar timeline={detail.timeline} totalDurationNs={nightStats.nightDuration} />
                )}
              </>
            )}
          </div>
        )}

        {dayStats && (
          <div class="cycle-section cycle-section-day">
            <div class="cycle-section-header">
              <span>☀️ Day</span>
              <span class="cycle-section-time">{fmtDur(dayStats.dayDuration)}</span>
            </div>
            {isDegenerate(day) ? <SplitMarkerNote kind="day" /> : (
              <>
                <div class="night-stats">
                  <Stat value={fmtDur(dayStats.totalNapTime)} label="Total Nap" />
                  <Stat value={String(dayStats.napCount)} label="Naps" />
                  <Stat value={fmtDur(dayStats.dayTotalFeedTime)} label="Feed Time" />
                  <Stat value={String(dayStats.dayFeedCount)} label="Day Feeds" />
                </div>
                <DayRhythmPills segments={dayStats.daySegments} live={!day?.endedAt} />
                {detail.dayTimeline.length > 0 && (
                  <TimelineBar
                    timeline={detail.dayTimeline}
                    totalDurationNs={dayStats.dayDuration}
                  />
                )}
              </>
            )}
          </div>
        )}
      </div>

      <div class="night-card">
        <h3><span>Event Log</span></h3>
        <div class="event-log">
          {detail.events.map((evt, i) => {
            const t = fmtClockTime(evt.timestamp);
            const ai = ACTION_INFO[evt.action];
            const label = ai ? `${ai.icon} ${actionLabel(evt.action)}` : evt.action;
            const meta = fmtEventMeta(evt.metadata);
            return <div key={i} class="event-row">{t} — {label}{meta}</div>;
          })}
        </div>
      </div>

      {splitTarget && (
        <SplitSessionSheet
          session={splitTarget}
          onClose={() => setSplitTarget(null)}
          onSplit={() => { setSplitTarget(null); onSplit(); }}
        />
      )}
    </div>
  );
}

// LongSessionBanner is the contextual recovery affordance shown in the detail
// view above a session-half that exceeds 18h (a forgotten Start day/night). It
// signals the problem and offers the split in one place.
function LongSessionBanner({ kind, durationNs, onSplit }: { kind: 'night' | 'day'; durationNs: number; onSplit: () => void }) {
  const hours = Math.round(durationNs / NS_PER_HOUR);
  const word = kind === 'night' ? 'night' : 'day';
  return (
    <div class="long-banner">
      <div class="long-banner-text">⚠ This {word} is unusually long ({hours}h).</div>
      <button class="long-banner-btn" onClick={onSplit}>Split this session</button>
    </div>
  );
}

// DayRhythmPills renders the alternating awake/nap segment durations. The
// last pill blinks when `live` is true — indicating the current in-progress
// segment (parallel to how the night's last sleep block blinks on the
// active night session).
function DayRhythmPills({ segments, live }: { segments: DaySegment[]; live?: boolean }) {
  if (!segments || segments.length === 0) return null;
  return (
    <div class="pill-group">
      <div class="pill-group-label">Day rhythm</div>
      <div class="pill-group-pills">
        {segments.map((s, i) => {
          const isLast = i === segments.length - 1;
          const cls = [
            'pill',
            s.kind === 'nap' ? 'pill-nap' : 'pill-awake',
            live && isLast ? 'pill-day-live' : '',
          ].filter(Boolean).join(' ');
          return <span key={i} class={cls}>{fmtDur(s.duration)}</span>;
        })}
      </div>
    </div>
  );
}

function SleepBlocksPills({ blocks, longest, active }: { blocks: number[]; longest: number; active?: boolean }) {
  if (!blocks || blocks.length === 0) return null;

  const longestIdx = blocks.indexOf(longest);
  return (
    <div class="pill-group">
      <div class="pill-group-label">Sleep blocks</div>
      <div class="pill-group-pills">
        {blocks.map((b, i) => {
          const isLast = i === blocks.length - 1;
          const cls = [
            'pill',
            i === longestIdx ? 'pill-sleep-longest' : '',
            active && isLast ? 'pill-sleep-live' : '',
          ].filter(Boolean).join(' ');
          return <span key={i} class={cls}>{fmtDur(b)}</span>;
        })}
      </div>
    </div>
  );
}

function FeedTimesPills({ times }: { times: string[] | null }) {
  if (!times || times.length === 0) return null;
  return (
    <div class="pill-group">
      <div class="pill-group-label">Feeds at</div>
      <div class="pill-group-pills">
        {times.map((t, i) => (
          <span key={i} class="pill pill-feed">
            {fmtClockTime(t)}
          </span>
        ))}
      </div>
    </div>
  );
}

function fmtEventMeta(m?: Record<string, string>): string {
  if (!m) return '';
  const vals = Object.values(m);
  return vals.length ? ` (${vals.join(', ')})` : '';
}

function Stat({ value, label }: { value: string; label: string }) {
  return (
    <div class="stat">
      <div class="stat-value">{value}</div>
      <div class="stat-label">{label}</div>
    </div>
  );
}
