/**
 * WebAuthn in the browser. The server sends the W3C options as JSON; the
 * browser's own JSON helpers turn them into the binary form and back, so
 * no base64 handling of our own is needed.
 */

interface JsonCredentialStatics {
  parseRequestOptionsFromJSON: (options: unknown) => PublicKeyCredentialRequestOptions;
}

interface JsonCredential {
  toJSON: () => Record<string, unknown>;
}

/** Whether passkeys can be used here: a secure page in a browser with the JSON helpers. */
export function passkeysSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    window.isSecureContext &&
    typeof PublicKeyCredential !== "undefined" &&
    "parseRequestOptionsFromJSON" in PublicKeyCredential
  );
}

/** Asks the authenticator for an assertion for `options` (`{ publicKey: … }`). */
export async function getAssertion(options: Record<string, unknown>): Promise<Record<string, unknown>> {
  const statics = PublicKeyCredential as unknown as JsonCredentialStatics;
  const publicKey = statics.parseRequestOptionsFromJSON(options.publicKey);
  const credential = (await navigator.credentials.get({ publicKey })) as unknown as JsonCredential | null;
  if (credential === null) {
    throw new Error("no credential was chosen");
  }
  return credential.toJSON();
}
