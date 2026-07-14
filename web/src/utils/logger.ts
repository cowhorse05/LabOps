import { useLogStore, type LogLevel } from '@/stores/logStore';

function formatDetail(err: unknown): string | undefined {
  if (err instanceof Error) {
    return `${err.name}: ${err.message}\n${err.stack ?? ''}`;
  }
  if (typeof err === 'string') return err;
  if (err && typeof err === 'object') {
    try { return JSON.stringify(err, null, 2); } catch { return String(err); }
  }
  return undefined;
}

function log(level: LogLevel, source: string, message: string, err?: unknown) {
  const detail = err ? formatDetail(err) : undefined;

  // Console output
  const consoleMsg = `[${source}] ${message}`;
  switch (level) {
    case 'debug': console.debug(consoleMsg, detail ?? ''); break;
    case 'info':  console.info(consoleMsg, detail ?? ''); break;
    case 'warn':  console.warn(consoleMsg, detail ?? ''); break;
    case 'error': console.error(consoleMsg, detail ?? ''); break;
  }

  // Store (fail silently — don't break the app if the store isn't ready)
  try {
    useLogStore.getState().push({ level, source, message, detail });
  } catch {
    // store not available (e.g. during SSR or init failure)
  }
}

export const logger = {
  debug: (source: string, message: string, err?: unknown) => log('debug', source, message, err),
  info:  (source: string, message: string, err?: unknown) => log('info',  source, message, err),
  warn:  (source: string, message: string, err?: unknown) => log('warn',  source, message, err),
  error: (source: string, message: string, err?: unknown) => log('error', source, message, err),
};
