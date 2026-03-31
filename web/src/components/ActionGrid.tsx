import { ACTION_INFO } from '../constants';

interface Props {
  actions: string[];
  onPointerDown: (action: string) => void;
  onPointerUp: (action: string) => void;
  onPointerCancel: () => void;
}

export function ActionGrid({ actions, onPointerDown, onPointerUp, onPointerCancel }: Props) {
  return (
    <div class="action-grid">
      {actions.map(action => {
        const ai = ACTION_INFO[action] || { icon: '?', label: action, cls: '' };
        return (
          <button
            key={action}
            class={`action-btn ${ai.cls}`}
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
