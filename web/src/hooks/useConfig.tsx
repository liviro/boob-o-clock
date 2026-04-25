import { createContext, ComponentChildren } from 'preact';
import { useContext, useEffect, useState } from 'preact/hooks';
import { getConfig, ServerConfig } from '../api';

const ConfigContext = createContext<ServerConfig | null>(null);

export function ConfigProvider({ children }: { children: ComponentChildren }) {
  const [config, setConfig] = useState<ServerConfig | null>(null);

  useEffect(() => {
    getConfig().then(setConfig).catch(() => {
      // Fail-closed: if the config fetch errors, render with all flags off.
      // Better to hide an opt-in feature than to leak it on a degraded server.
      setConfig({ features: { ferber: false, chair: false } });
    });
  }, []);

  if (!config) return <div class="no-data">Loading...</div>;
  return <ConfigContext.Provider value={config}>{children}</ConfigContext.Provider>;
}

export function useConfig(): ServerConfig {
  const cfg = useContext(ConfigContext);
  if (!cfg) throw new Error('useConfig called outside <ConfigProvider>');
  return cfg;
}
