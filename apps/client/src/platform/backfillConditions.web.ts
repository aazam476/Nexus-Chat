/** Browsers and the desktop app backfill whenever asked: there is no metered data or battery to spare. */
export function goodTimeToBackfill(): Promise<boolean> {
  return Promise.resolve(true);
}
