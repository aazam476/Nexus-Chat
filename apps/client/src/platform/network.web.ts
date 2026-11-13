/** Browsers and the desktop app send originals by default (ADR 0014). */
export function onMobileData(): Promise<boolean> {
  return Promise.resolve(false);
}
