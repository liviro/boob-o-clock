import { ACTION_INFO } from '../constants';

interface Props {
  lastAction?: string;
  onUndo: () => void;
}

export function UndoButton({ lastAction, onUndo }: Props) {
  const disabled = !lastAction;
  let label = '↩ Undo';
  if (lastAction) {
    const ai = ACTION_INFO[lastAction];
    const name = ai ? ai.label.replace(/\n/g, ' ') : lastAction;
    label = `↩ Undo: ${name}`;
  }

  return (
    <button class="undo-btn" disabled={disabled} onClick={onUndo}>
      {label}
    </button>
  );
}
