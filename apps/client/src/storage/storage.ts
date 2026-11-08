/**
 * Small secrets the app keeps between launches: the instance address and
 * the session token. Each platform has its own implementation file
 * (`secureStorage.native.ts`, `secureStorage.web.ts`), chosen by the
 * bundler; this file holds the shared contract.
 */

/** A store of short strings by key. */
export interface SecureStorage {
  get(key: string): Promise<string | undefined>;
  set(key: string, value: string): Promise<void>;
  remove(key: string): Promise<void>;
}

/** Keys the app stores. */
export const storageKeys = {
  instance: "instance",
  session: "session",
  /** A provider sign-in in progress on web (see auth/providerSignIn.web.ts). */
  providerFlow: "provider-flow",
} as const;

/**
 * Keeps values in memory only. Used where nothing safer exists, so a value
 * is lost when the app closes rather than stored readably.
 */
export function createMemoryStorage(): SecureStorage {
  const values = new Map<string, string>();
  return {
    get: (key) => Promise.resolve(values.get(key)),
    set: (key, value) => {
      values.set(key, value);
      return Promise.resolve();
    },
    remove: (key) => {
      values.delete(key);
      return Promise.resolve();
    },
  };
}
