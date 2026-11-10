import { protectKey, unprotectKey, type SafeStorage } from "./keyProtection";

/** Reverses the bytes: enough to show they went through the OS. */
function fakeStorage(backend = "keychain", available = true): SafeStorage {
  return {
    isEncryptionAvailable: () => available,
    getSelectedStorageBackend: () => backend,
    encryptString: (text) => Buffer.from(text).reverse(),
    decryptString: (encrypted) => Buffer.from(encrypted).reverse().toString(),
  };
}

const key = new Uint8Array(32).map((_, index) => index);

describe("protecting the database key", () => {
  it("round-trips through the OS secret store", () => {
    const sealed = protectKey(fakeStorage(), key);

    expect(sealed).toBeDefined();
    expect(sealed).not.toEqual(key);
    expect(unprotectKey(fakeStorage(), sealed)).toEqual(key);
  });

  it("declines where the OS offers no real protection", () => {
    expect(protectKey(fakeStorage("basic_text"), key)).toBeUndefined();
    expect(protectKey(fakeStorage("keychain", false), key)).toBeUndefined();
    const withoutBackendInfo: SafeStorage = { ...fakeStorage(), getSelectedStorageBackend: undefined };
    expect(protectKey(withoutBackendInfo, key)).toBeDefined();
  });

  it("refuses anything but a 32-byte key", () => {
    expect(() => protectKey(fakeStorage(), new Uint8Array(16))).toThrow("expected a 32-byte key");
    expect(() => protectKey(fakeStorage(), "key")).toThrow(TypeError);
    expect(() => unprotectKey(fakeStorage(), "sealed")).toThrow(TypeError);
    const short = protectKey(fakeStorage(), key)?.slice(0, 10) ?? new Uint8Array(0);
    expect(() => unprotectKey(fakeStorage(), short)).toThrow("not 32 bytes");
  });
});
