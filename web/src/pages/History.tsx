import { useState, useEffect, useCallback } from 'preact/hooks';
import { getCycles, getCycleDetail, CycleSummary, CycleDetail, SessionMeta, DaySegment } from '../api';
import { fmtDur, toNightHour, ACTION_INFO, actionLabel } from '../constants';
import { TimelineBar } from '../components/TimelineBar';
import { CycleTimelineBar } from '../components/CycleTimelineBar';
import { TrendChart } from '../components/TrendChart';
import { NightHourChart } from '../components/NightHourChart';
import { ErrorToast } from '../components/ErrorToast';
import { useIsLandscape } from '../hooks/useIsLandscape';
import { useConfig } from '../hooks/useConfig';

type View = 'cycles' | 'trends';

const DISPLAY_LIMIT = 30;

const nsToMinutes = (ns: number) => Math.round(ns / 1e9 / 60);

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

export function History() {
  const [cycles, setCycles] = useState<CycleSummary[]>([]);
  const [detail, setDetail] = useState<CycleDetail | null>(null);
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
    } catch {
      setError('Failed to load cycle details');
    }
  }

  if (loading) return <div class="no-data">Loading...</div>;

  if (detail) {
    return <CycleDetailView detail={detail} onBack={() => setDetail(null)} />;
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
  const ferberVisible = useConfig().features.ferber;
  const anchor = cycle.day?.startedAt ?? cycle.night?.startedAt;
  if (!anchor) return null;
  const date = new Date(anchor);
  const dateStr = date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
  const timeStr = cycle.night
    ? new Date(cycle.night.startedAt).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
    : '';
  const day = cycle.stats.day;
  const night = cycle.stats.night;
  const isFerber = ferberVisible && !!cycle.night?.ferberEnabled;

  return (
    <div class="night-card clickable" onClick={onClick}>
      <h3>
        <span>{dateStr}{isFerber && <span class="ferber-badge" title={`Night ${cycle.night?.ferberNightNumber ?? ''}`}>🌱</span>}</span>
        <span>{timeStr}</span>
      </h3>
      {day && (
        <div class="cycle-section cycle-section-day">
          <div class="cycle-section-header">☀️ Day</div>
          <div class="night-stats">
            <Stat value={fmtDur(day.totalNapTime)} label="Total Nap" />
            <Stat value={String(day.napCount)} label="Naps" />
            <Stat value={fmtDur(day.dayTotalFeedTime)} label="Feed Time" />
            <Stat value={String(day.dayFeedCount)} label="Day Feeds" />
          </div>
          <DayRhythmPills segments={day.daySegments} live={!cycle.day?.endedAt} />
        </div>
      )}
      {night && (
        <div class="cycle-section cycle-section-night">
          <div class="cycle-section-header">🌙 Night</div>
          <div class="night-stats">
            <Stat value={fmtDur(night.longestSleepBlock)} label="Longest Sleep" />
            <Stat value={String(night.wakeCount)} label="Wakes" />
            <Stat value={fmtDur(night.totalFeedTime)} label="Feed Time" />
            <Stat value={fmtDur(night.totalSleepTime)} label="Total Sleep" />
          </div>
          <SleepBlocksPills blocks={night.sleepBlocks} longest={night.longestSleepBlock} active={!cycle.night?.endedAt} />
          <FeedTimesPills times={night.feedTimes} />
        </div>
      )}
    </div>
  );
}

// --- Trends view ---

function TrendsView({ cycles }: { cycles: CycleSummary[] }) {
  const ferberVisible = useConfig().features.ferber;
  // Spread at each chart so we don't copy these two props 10 times.
  const ferberProps = ferberVisible
    ? { highlightFerber: true, isFerber: (c: CycleSummary) => !!c.night?.ferberEnabled }
    : {};
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

      <NightHourChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        getDots={c => c.stats.night?.feedTimes?.map(t => ({ hour: toNightHour(t) })) ?? []}
        color="#c0b040"
        title="Feed Times"
        {...ferberProps}
      />

      <NightHourChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        getDots={c => c.stats.night?.realBedtime ? [{ hour: toNightHour(c.stats.night.realBedtime) }] : []}
        getAvgHour={c => bedtimeAvgByCycle.get(c) ?? null}
        color="#6a9aff"
        title="Real Bedtime"
        {...ferberProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.day?.totalNapTime ?? 0,
          getAvg: c => c.avg?.day?.totalNapTime ?? null,
          color: '#5affaa',
        }]}
        formatValue={fmtDur}
        title="Total Nap Duration"
        {...ferberProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.longestSleepBlock ?? 0,
          getAvg: () => null,
          color: '#4a8aff',
        }]}
        formatValue={fmtDur}
        title="Longest Sleep Block (night)"
        {...ferberProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.totalSleepTime ?? 0,
          getAvg: c => c.avg?.night?.totalSleepTime ?? null,
          color: '#6a5aff',
        }]}
        formatValue={fmtDur}
        title="Total Sleep (night)"
        {...ferberProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.wakeCount ?? 0,
          color: '#ff5a5a',
        }]}
        formatValue={v => String(Math.round(v))}
        title="Wake Count (night)"
        {...ferberProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.feedCount ?? 0,
          color: '#ffaa5a',
        }]}
        formatValue={v => String(Math.round(v))}
        title="Feed Count (night)"
        {...ferberProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[{
          getValue: c => c.stats.night?.totalFeedTime ?? 0,
          getAvg: c => c.avg?.night?.totalFeedTime ?? null,
          color: '#ffaa5a',
        }]}
        formatValue={fmtDur}
        title="Total Feed Time (night)"
        {...ferberProps}
      />

      <TrendChart
        points={chronological}
        getDate={c => c.night?.startedAt ?? c.day!.startedAt}
        series={[
          { getValue: c => c.stats.night?.feedTimeLeft ?? 0, getAvg: c => c.avg?.night?.feedTimeLeft ?? null, color: '#5a9aff', label: 'Left' },
          { getValue: c => c.stats.night?.feedTimeRight ?? 0, getAvg: c => c.avg?.night?.feedTimeRight ?? null, color: '#ff7a5a', label: 'Right' },
        ]}
        formatValue={fmtDur}
        title="Feed Time by Side (night)"
        {...ferberProps}
      />

      {ferberVisible && (
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
    </div>
  );
}

// StackedCycleTimelines renders one CycleTimelineBar per cycle, newest at top.
// Uses the events inline on each CycleSummary — no additional fetches. Each
// cycle is handed the *previous* cycle's events too (the older cycle, which
// is at index+1 since the list is newest-first), so the bar can render state
// inherited across the 7am boundary (e.g., sleep trailing from last night).
function StackedCycleTimelines({ cycles }: { cycles: CycleSummary[] }) {
  return (
    <div class="trend-chart">
      <div class="trend-title">24h Cycle Timelines</div>
      <div class="stacked-cycle-list">
        {cycles.map((c, i) => {
          const anchor = c.day?.startedAt ?? c.night?.startedAt;
          if (!anchor) return null;
          const label = new Date(anchor).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
          // Older cycle = next one in the newest-first array.
          const prevCycle = cycles[i + 1];
          return (
            <CycleTimelineBar
              key={cardKey(c, i)}
              day={c.day}
              night={c.night}
              events={c.events}
              prevEvents={prevCycle?.events}
              label={label}
            />
          );
        })}
      </div>
    </div>
  );
}

// --- Cycle detail view ---

function CycleDetailView({ detail, onBack }: { detail: CycleDetail; onBack: () => void }) {
  const ferberVisible = useConfig().features.ferber;
  const { day, night } = detail.cycle;
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
          <span>{nightStats ? fmtDur(nightStats.nightDuration) : ''}</span>
        </h3>

        {dayStats && (
          <div class="cycle-section cycle-section-day">
            <div class="cycle-section-header">☀️ Day</div>
            <div class="night-stats">
              <Stat value={fmtDur(dayStats.totalNapTime)} label="Total Nap" />
              <Stat value={String(dayStats.napCount)} label="Naps" />
              <Stat value={fmtDur(dayStats.dayTotalFeedTime)} label="Feed Time" />
              <Stat value={String(dayStats.dayFeedCount)} label="Day Feeds" />
            </div>
            <DayRhythmPills segments={dayStats.daySegments} live={!day?.endedAt} />
            {detail.dayTimeline.length > 0 && day && (
              <TimelineBar
                timeline={detail.dayTimeline}
                totalDurationNs={dayDurationNs(day)}
              />
            )}
          </div>
        )}

        {nightStats && (
          <div class="cycle-section cycle-section-night">
            <div class="cycle-section-header">🌙 Night</div>
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
          </div>
        )}
      </div>

      <div class="night-card">
        <h3><span>Event Log</span></h3>
        <div class="event-log">
          {detail.events.map((evt, i) => {
            const t = new Date(evt.timestamp).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
            const ai = ACTION_INFO[evt.action];
            const label = ai ? `${ai.icon} ${actionLabel(evt.action)}` : evt.action;
            const meta = fmtEventMeta(evt.metadata);
            return <div key={i} class="event-row">{t} — {label}{meta}</div>;
          })}
        </div>
      </div>
    </div>
  );
}

// dayDurationNs returns the day session's total duration in nanoseconds,
// falling back to "now" for in-progress (unended) sessions — same convention
// as the server's ComputeStats for open sessions.
function dayDurationNs(day: SessionMeta): number {
  const startMs = new Date(day.startedAt).getTime();
  const endMs = day.endedAt ? new Date(day.endedAt).getTime() : Date.now();
  return (endMs - startMs) * 1e6;
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
            {new Date(t).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
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
