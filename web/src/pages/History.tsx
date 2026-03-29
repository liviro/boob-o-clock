import { useState, useEffect } from 'preact/hooks';
import { getNights, getNightDetail, NightSummary, NightDetail } from '../api';
import { fmtDur, ACTION_INFO } from '../constants';
import { TimelineBar } from '../components/TimelineBar';

export function History() {
  const [nights, setNights] = useState<NightSummary[]>([]);
  const [detail, setDetail] = useState<NightDetail | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => { loadNights(); }, []);

  async function loadNights() {
    setLoading(true);
    setDetail(null);
    try {
      const data = await getNights();
      setNights((data.nights || []).reverse());
    } catch {
      setNights([]);
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
    const n = detail.night;
    const s = detail.stats;
    const date = new Date(n.startedAt);
    const dateStr = date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });

    return (
      <div class="history-content">
        <button class="back-btn" onClick={loadNights}>← Back</button>

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
              const label = ai ? `${ai.icon} ${ai.label.replace(/\n/g, ' ')}` : evt.action;
              const meta = evt.metadata ? ` (${Object.values(evt.metadata).join(', ')})` : '';
              return <div key={i} class="event-row">{t} — {label}{meta}</div>;
            })}
          </div>
        </div>
      </div>
    );
  }

  if (nights.length === 0) {
    return <div class="no-data">No nights recorded yet</div>;
  }

  return (
    <div class="history-content">
      {nights.map(n => {
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
