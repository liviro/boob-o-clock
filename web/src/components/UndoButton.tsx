import { actionLabel } from '../constants';

interface Props {
  lastAction?: string;
  onUndo: () => void;
}

export function UndoButton({ lastAction, onUndo }: Props) {
  return (
    <button class="undo-btn" disabled={!lastAction} onClick={onUndo}>
      {lastAction ? `↩ Undo: ${actionLabel(lastAction)}` : '↩ Undo'}
    </button>
  );
}
