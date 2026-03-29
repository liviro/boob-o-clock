import { useState, useEffect } from 'preact/hooks';
import { getNights, getNightDetail, getTrends, NightSummary, NightDetail, TrendPoint } from '../api';
import { fmtDur, ACTION_INFO, actionLabel } from '../constants';
import { TimelineBar } from '../components/TimelineBar';
import { TrendChart } from '../components/TrendChart';

type View = 'nights' | 'trends';

function fmtDurChart(ns: number): string {
  const h = Math.floor(ns / 1e9 / 3600);
  const m = Math.floor((ns / 1e9 % 3600) / 60);
  if (h > 0) return `${h}h${m > 0 ? m + 'm' : ''}`;
  return `${m}m`;
}

export function History() {
  const [nights, setNights] = useState<NightSummary[]>([]);
  const [trends, setTrends] = useState<TrendPoint[]>([]);
  const [detail, setDetail] = useState<NightDetail | null>(null);
  const [view, setView] = useState<View>('nights');
  const [loading, setLoading] = useState(true);

  useEffect(() => { loadNights(); }, []);

  async function loadNights() {
    setLoading(true);
    setDetail(null);
    try {
      const [nightsData, trendsData] = await Promise.all([getNights(), getTrends()]);
      setNights((nightsData.nights || []).reverse());
      setTrends(trendsData.trends || []);
    } catch {
      setNights([]);
      setTrends([]);
    } finally {
      setLoading(false);
    }
  }

  async function showDetail(id: number) {
    try {
      const data = await getNightDetail(id);
      setDetail(data);
    } catch {
      // stay on list
    }
  }

  if (loading) return <div class="no-data">Loading...</div>;

  if (detail) {
    return <NightDetailView detail={detail} onBack={() => setDetail(null)} />;
  }

  if (nights.length === 0) {
    return <div class="no-data">No nights recorded yet</div>;
  }

  return (
    <div class="history-content">
      <div class="view-toggle">
        <button class={`view-btn ${view === 'nights' ? 'active' : ''}`} onClick={() => setView('nights')}>
          Nights
        </button>
        <button class={`view-btn ${view === 'trends' ? 'active' : ''}`} onClick={() => setView('trends')}>
          Trends
        </button>
      </div>

      {view === 'nights' && nights.map(n => {
        const date = new Date(n.startedAt);
        const dateStr = date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
        const timeStr = date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
        const s = n.stats;

        return (
          <div key={n.id} class="night-card" onClick={() => showDetail(n.id)}>
            <h3>
              <span>{dateStr}</span>
              <span>{timeStr}</span>
            </h3>
            <div class="night-stats">
              <Stat value={fmtDur(s.longestSleepBlock)} label="Longest Sleep" />
              <Stat value={String(s.wakeCount)} label="Wakes" />
              <Stat value={String(s.feedCount)} label="Feeds" />
              <Stat value={fmtDur(s.totalSleepTime)} label="Total Sleep" />
            </div>
          </div>
        );
      })}

      {view === 'trends' && (
        <div class="trends-grid">
          <TrendChart
            trends={trends}
            getValue={p => p.longestSleep}
            getAvg={p => p.avgLongestSleep}
            formatValue={fmtDurChart}
            title="Longest Sleep Block"
            color="#4a8aff"
          />
          <TrendChart
            trends={trends}
            getValue={p => p.totalSleep}
            getAvg={p => p.avgTotalSleep}
            formatValue={fmtDurChart}
            title="Total Sleep"
            color="#6a5aff"
          />
          <TrendChart
            trends={trends}
            getValue={p => p.wakeCount}
            getAvg={p => p.avgWakeCount}
            formatValue={v => String(Math.round(v))}
            title="Wake Count"
            color="#ff5a5a"
          />
          <TrendChart
            trends={trends}
            getValue={p => p.feedCount}
            getAvg={p => p.avgFeedCount}
            formatValue={v => String(Math.round(v))}
            title="Feed Count"
            color="#ffaa5a"
          />
        </div>
      )}
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
        <TimelineBar timeline={detail.timeline} totalDurationNs={s.nightDuration} />
      </div>

      <div class="night-card">
        <h3><span>Event Log</span></h3>
        <div class="event-log">
          {detail.events.map((evt, i) => {
            const t = new Date(evt.timestamp).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
            const ai = ACTION_INFO[evt.action];
            const label = ai ? `${ai.icon} ${actionLabel(evt.action)}` : evt.action;
            const meta = evt.metadata ? ` (${Object.values(evt.metadata).join(', ')})` : '';
            return <div key={i} class="event-row">{t} — {label}{meta}</div>;
          })}
        </div>
      </div>
    </div>
  );
}

function Stat({ value, label }: { value: string; label: string }) {
  return (
    <div class="stat">
      <div class="stat-value">{value}</div>
      <div class="stat-label">{label}</div>
    </div>
  );
}
