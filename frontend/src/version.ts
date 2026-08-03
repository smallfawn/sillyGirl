export const SILLYGIRL_VERSION = '1.0.2';

declare global {
  interface Window {
    __SILLYGIRL_VERSION__?: string;
    SILLYGIRL_VERSION?: string;
  }
}

export function mountSillyGirlVersion() {
  if (typeof window === 'undefined') return;
  window.__SILLYGIRL_VERSION__ = SILLYGIRL_VERSION;
  window.SILLYGIRL_VERSION = SILLYGIRL_VERSION;
}
