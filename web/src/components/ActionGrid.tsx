import { ACTION_INFO } from '../constants';

interface Props {
  actions: string[];
  onPointerDown: (action: string) => void;
  onPointerUp: (action: string) => void;
  onPointerCancel: () => void;
}

// States with <= this many actions render every button full-width.
// Denser states (Awake has 6) use per-action cls so Stroller/Poop can pair.
const FULL_WIDTH_THRESHOLD = 4;

export function ActionGrid({ actions, onPointerDown, onPointerUp, onPointerCancel }: Props) {
  const allFull = actions.length <= FULL_WIDTH_THRESHOLD;
  return (
    <div class="action-grid">
      {actions.map(action => {
        const ai = ACTION_INFO[action] || { icon: '?', label: action, cls: '' };
        const cls = allFull && !ai.cls.includes('full-width') ? `${ai.cls} full-width` : ai.cls;
        return (
          <button
            key={action}
            class={`action-btn ${cls}`}
            onTouchStart={(e) => { e.preventDefault(); onPointerDown(action); }}
            onTouchEnd={() => onPointerUp(action)}
            onTouchCancel={onPointerCancel}
            onMouseDown={() => onPointerDown(action)}
            onMouseUp={() => onPointerUp(action)}
            onMouseLeave={onPointerCancel}
          >
            <span class="action-icon">{ai.icon}</span>
            <span>{ai.label.split('\n').map((line, i) =>
              i > 0 ? [<br />, line] : line
            )}</span>
          </button>
        );
      })}
    </div>
  );
}
