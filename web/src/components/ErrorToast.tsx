import { useEffect } from 'preact/hooks';

interface Props {
  message: string | null;
  onDismiss: () => void;
}

export function ErrorToast({ message, onDismiss }: Props) {
  useEffect(() => {
    if (!message) return;
    const id = setTimeout(onDismiss, 3000);
    return () => clearTimeout(id);
  }, [message, onDismiss]);

  if (!message) return null;

  return <div class="error-toast">{message}</div>;
}
