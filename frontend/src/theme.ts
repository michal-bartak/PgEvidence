import { IsSystemDark } from '../wailsjs/go/main/App';
import { Environment } from '../wailsjs/runtime/runtime';

// Theme is one of "system" | "light" | "dark". Applied by setting data-theme on
// <html>; style.css defines the palettes (and color-scheme for native controls).

let mq: MediaQueryList | null = null;
let mqHandler: (() => void) | null = null;
let pollTimer: ReturnType<typeof setInterval> | null = null;
let isLinux: boolean | null = null;

async function linux(): Promise<boolean> {
  if (isLinux !== null) return isLinux;
  try {
    isLinux = (await Environment()).platform === 'linux';
  } catch {
    isLinux = false;
  }
  return isLinux;
}

async function resolveSystemDark(): Promise<boolean> {
  if (await linux()) {
    try {
      return await IsSystemDark();
    } catch {
      // fall through to matchMedia
    }
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function setDataTheme(dark: boolean) {
  document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light');
}

function teardown() {
  if (mq && mqHandler) mq.removeEventListener('change', mqHandler);
  mq = null;
  mqHandler = null;
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = null;
}

export async function applyTheme(theme: string) {
  teardown(); // avoid leaking listeners/timers across repeated switches

  if (theme === 'light') {
    setDataTheme(false);
    return;
  }
  if (theme === 'dark') {
    setDataTheme(true);
    return;
  }

  // system
  const refresh = async () => setDataTheme(await resolveSystemDark());
  await refresh();

  mq = window.matchMedia('(prefers-color-scheme: dark)');
  mqHandler = () => { refresh(); };
  mq.addEventListener('change', mqHandler);

  // WebKitGTK on Linux ignores prefers-color-scheme (no change events) — poll.
  if (await linux()) {
    pollTimer = setInterval(refresh, 5000);
  }
}
