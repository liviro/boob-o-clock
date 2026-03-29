import { ACTION_INFO } from '../constants';

interface Props {
  actions: string[];
  onAction: (action: string) => void;
}

export function ActionGrid({ actions, onAction }: Props) {
  return (
    <div class="action-grid">
      {actions.map(action => {
        const ai = ACTION_INFO[action] || { icon: '?', label: action, cls: '' };
        return (
          <button
            key={action}
            class={`action-btn ${ai.cls}`}
            onClick={() => onAction(action)}
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
