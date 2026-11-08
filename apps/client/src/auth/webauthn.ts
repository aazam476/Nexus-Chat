/**
 * WebAuthn on iOS and Android needs the app to be linked to each
 * instance's domain, which arrives with the native passkey work in a later
 * milestone (docs/platform-support.md). Until then the native apps offer
 * password, provider and code logins.
 */

/** Whether passkeys can be used here. */
export function passkeysSupported(): boolean {
  return false;
}

/** Asks the authenticator for an assertion; never called where unsupported. */
export function getAssertion(_options: Record<string, unknown>): Promise<Record<string, unknown>> {
  return Promise.reject(new Error("passkeys are not supported on this platform yet"));
}
