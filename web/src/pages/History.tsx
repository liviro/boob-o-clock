import { useState, useEffect, useCallback } from 'preact/hooks';
import { getNights, getNightDetail, getTrends, NightSummary, NightDetail, TrendPoint } from '../api';
import { fmtDur, toNightHour, ACTION_INFO, actionLabel } from '../constants';
import { TimelineBar } from '../components/TimelineBar';
import { TrendChart } from '../components/TrendChart';
import { NightHourChart } from '../components/NightHourChart';
import { ErrorToast } from '../components/ErrorToast';
import { useIsLandscape } from '../hooks/useIsLandscape';

type View = 'nights' | 'trends';

const DISPLAY_LIMIT = 30;

const nsToMinutes = (ns: number) => Math.round(ns / 1e9 / 60);

export function History() {
  const [nights, setNights] = useState<NightSummary[]>([]);
  const [trends, setTrends] = useState<TrendPoint[] | null>(null);
  const [detail, setDetail] = useState<NightDetail | null>(null);
  const [view, setView] = useState<View>('nights');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const clearError = useCallback(() => setError(null), []);
  const isLandscape = useIsLandscape();

  useEffect(() => { loadNights(); }, []);

  async function loadNights() {
    setLoading(true);
    setDetail(null);
    try {
      const data = await getNights();
      setNights((data.nights || []).reverse());
    } catch {
      setError('Failed to load nights');
    } finally {
      setLoading(false);
    }
  }

  async function loadTrends() {
    try {
      const data = await getTrends();
      setTrends(data.trends || []);
    } catch {
      setError('Failed to load trends');
    }
  }

  function switchView(v: View) {
    setView(v);
    if (v === 'trends') loadTrends();
  }

  async function showDetail(id: number) {
    try {
      const data = await getNightDetail(id);
      setDetail(data);
    } catch {
      setError('Failed to load night details');
    }
  }

  if (loading) return <div class="no-data">Loading...</div>;

  if (detail) {
    return <NightDetailView detail={detail} onBack={() => setDetail(null)} />;
  }

  if (nights.length === 0) {
    return <div class="no-data">No nights recorded yet</div>;
  }

  const nightsForList = nights.slice(0, DISPLAY_LIMIT);
  const nightsForCharts = isLandscape ? nights : nights.slice(0, DISPLAY_LIMIT);
  const trendsForCharts: TrendPoint[] | null = trends
    ? (isLandscape ? trends : trends.slice(-DISPLAY_LIMIT))
    : null;

  return (
    <div class="history-content">
      <div class="view-toggle">
        <button class={`view-btn ${view === 'nights' ? 'active' : ''}`} onClick={() => switchView('nights')}>
          Nights
        </button>
        <button class={`view-btn ${view === 'trends' ? 'active' : ''}`} onClick={() => switchView('trends')}>
          {nights.length > DISPLAY_LIMIT ? `Trends (${isLandscape ? '90d' : '30d'})` : 'Trends'}
        </button>
      </div>

      {view === 'nights' && (
        <>
          {nightsForList.map(n => {
            const date = new Date(n.startedAt);
            const dateStr = date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
            const timeStr = date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
            const s = n.stats;

            return (
              <div key={n.id} class="night-card clickable" onClick={() => showDetail(n.id)}>
                <h3>
                  <span>{dateStr}{n.ferberEnabled && <span class="ferber-badge" title={`Night ${n.ferberNightNumber ?? ''}`}>🌱</span>}</span>
                  <span>{timeStr}</span>
                </h3>
                <div class="night-stats">
                  <Stat value={fmtDur(s.longestSleepBlock)} label="Longest Sleep" />
                  <Stat value={String(s.wakeCount)} label="Wakes" />
                  <Stat value={String(s.feedCount)} label="Feeds" />
                  <Stat value={fmtDur(s.totalSleepTime)} label="Total Sleep" />
                </div>
                <SleepBlocksPills blocks={s.sleepBlocks} longest={s.longestSleepBlock} active={!n.endedAt} />
                <FeedTimesPills times={s.feedTimes} />
              </div>
            );
          })}
          {nights.length > DISPLAY_LIMIT && (
            <div class="nights-caption">Showing {DISPLAY_LIMIT} most recent nights</div>
          )}
        </>
      )}

      {view === 'trends' && trends === null && (
        <div class="no-data">Loading trends...</div>
      )}

      {view === 'trends' && trendsForCharts !== null && (
        <div class="trends-grid">
          <TrendChart
            trends={trendsForCharts}
            series={[{ getValue: p => p.longestSleep, getAvg: p => p.avgLongestSleep, color: '#4a8aff' }]}
            formatValue={fmtDur}
            title="Longest Sleep Block"
            highlightFerber
          />
          <TrendChart
            trends={trendsForCharts}
            series={[{ getValue: p => p.totalSleep, getAvg: p => p.avgTotalSleep, color: '#6a5aff' }]}
            formatValue={fmtDur}
            title="Total Sleep"
            highlightFerber
          />
          <TrendChart
            trends={trendsForCharts}
            series={[{ getValue: p => p.wakeCount, getAvg: p => p.avgWakeCount, color: '#ff5a5a' }]}
            formatValue={v => String(Math.round(v))}
            title="Wake Count"
            highlightFerber
          />
          <TrendChart
            trends={trendsForCharts}
            series={[{ getValue: p => p.feedCount, getAvg: p => p.avgFeedCount, color: '#ffaa5a' }]}
            formatValue={v => String(Math.round(v))}
            title="Feed Count"
            highlightFerber
          />
          <TrendChart
            trends={trendsForCharts}
            series={[{ getValue: p => p.totalFeed, getAvg: p => p.avgTotalFeed, color: '#ffaa5a' }]}
            formatValue={fmtDur}
            title="Total Feed Time"
            highlightFerber
          />
          <TrendChart
            trends={trendsForCharts}
            series={[
              { getValue: p => p.feedTimeLeft, getAvg: p => p.avgFeedTimeLeft, color: '#5a9aff', label: 'Left' },
              { getValue: p => p.feedTimeRight, getAvg: p => p.avgFeedTimeRight, color: '#ff7a5a', label: 'Right' },
            ]}
            formatValue={fmtDur}
            title="Feed Time by Side"
            highlightFerber
          />
          <NightHourChart
            nights={nightsForCharts}
            getHours={n => (n.stats.feedTimes ?? []).map(toNightHour)}
            color="#c0b040"
            title="Feed Times"
            highlightFerber
          />
          <NightHourChart
            nights={nightsForCharts}
            getHours={n => n.stats.realBedtime ? [toNightHour(n.stats.realBedtime)] : []}
            color="#6a9aff"
            title="Real Bedtime"
            highlightFerber
          />
          {trendsForCharts.some(t => t.ferberCryTime != null) && (
            <TrendChart
              trends={trendsForCharts.filter(t => t.ferberCryTime != null)}
              series={[{ getValue: p => nsToMinutes(p.ferberCryTime!), getAvg: () => null, color: '#ff5a8a' }]}
              formatValue={v => `${Math.round(v)}m`}
              title="🌱 Cry time per night"
            />
          )}
          {trendsForCharts.some(t => t.ferberCheckIns != null) && (
            <TrendChart
              trends={trendsForCharts.filter(t => t.ferberCheckIns != null)}
              series={[{ getValue: p => p.ferberCheckIns!, getAvg: () => null, color: '#a05aff' }]}
              formatValue={v => String(Math.round(v))}
              title="🌱 Check-ins per night"
            />
          )}
          {trendsForCharts.some(t => t.ferberTimeToSettle != null && t.ferberTimeToSettle > 0) && (
            <TrendChart
              trends={trendsForCharts.filter(t => t.ferberTimeToSettle != null && t.ferberTimeToSettle > 0)}
              series={[{ getValue: p => nsToMinutes(p.ferberTimeToSettle!), getAvg: () => null, color: '#5affaa' }]}
              formatValue={v => `${Math.round(v)}m`}
              title="🌱 Avg time to settle"
            />
          )}
        </div>
      )}

      <ErrorToast message={error} onDismiss={clearError} />
    </div>
  );
}

function NightDetailView({ detail, onBack }: { detail: NightDetail; onBack: () => void }) {
  const n = detail.night;
  const s = detail.stats;
  const date = new Date(n.startedAt);
  const dateStr = date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });

  return (
    <div class="history-content">
      <button class="back-btn" onClick={onBack}>← Back</button>

      <div class="night-card">
        <h3>
          <span>{dateStr}</span>
          <span>{fmtDur(s.nightDuration)}</span>
        </h3>
        <div class="night-stats">
          <Stat value={fmtDur(s.longestSleepBlock)} label="Longest Sleep" />
          <Stat value={String(s.wakeCount)} label="Wakes" />
          <Stat value={fmtDur(s.totalFeedTime)} label="Feed Time" />
          <Stat value={fmtDur(s.totalSleepTime)} label="Total Sleep" />
        </div>
        {s.ferber && n.ferberEnabled && (
          <div class="ferber-stats">
            <div class="ferber-stats-header">🌱 Night {n.ferberNightNumber}</div>
            <div class="night-stats">
              <Stat value={String(s.ferber.sessions)} label="Sessions" />
              <Stat value={fmtDur(s.ferber.avgTimeToSettle)} label="Session average" />
              <Stat value={fmtDur(s.ferber.cryTime)} label="Cry time" />
              <Stat value={fmtDur(s.ferber.fussTime)} label="Fuss time" />
            </div>
            <details class="ferber-details">
              <summary>More</summary>
              <div class="night-stats">
                <Stat value={String(s.ferber.checkIns)} label="Check-ins" />
                <Stat value={String(s.ferber.sessionsAbandoned)} label="Abandoned" />
                <Stat value={fmtDur(s.ferber.quietTime)} label="Quiet time" />
              </div>
            </details>
          </div>
        )}
        <SleepBlocksPills blocks={s.sleepBlocks} longest={s.longestSleepBlock} active={!n.endedAt} />
        <FeedTimesPills times={s.feedTimes} />
        <TimelineBar timeline={detail.timeline} totalDurationNs={s.nightDuration} />
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
