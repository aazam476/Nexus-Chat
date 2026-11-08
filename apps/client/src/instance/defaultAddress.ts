/**
 * The instance to use before the user picks one. The native apps connect to
 * any instance, so they start with none (spec §3.6).
 */
export function defaultAddress(): string | undefined {
  return undefined;
}
