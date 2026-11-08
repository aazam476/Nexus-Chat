/**
 * When to warn about the HTTPS certificate expiring (spec §3.3): from 30
 * days before, more urgently from 7, matching the server's emails.
 */

const day = 24 * 60 * 60 * 1000;

/** How soon the certificate expires, if soon enough to warn about. */
export interface ExpiryWarning {
  /** Whole days left; 0 on the last day, negative once it has expired. */
  daysLeft: number;
  urgent: boolean;
}

/** The warning for a certificate expiring at `expiresAt`, or undefined when there is time. */
export function expiryWarning(expiresAt: string | undefined, now: Date): ExpiryWarning | undefined {
  if (expiresAt === undefined) {
    return undefined;
  }
  const left = new Date(expiresAt).getTime() - now.getTime();
  if (left > 30 * day) {
    return undefined;
  }
  return { daysLeft: Math.floor(left / day), urgent: left <= 7 * day };
}
