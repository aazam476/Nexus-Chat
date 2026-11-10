import {
  BackupError,
  createBackupKey,
  createRecoveryPhrase,
  minimumPasswordMemory,
  newPasswordParameters,
  normalizeRecoveryPhrase,
  openBlob,
  passkeyKey,
  passwordKey,
  recoveryPhraseKey,
  sealBlob,
  unwrapBackupKey,
  wrapBackupKey,
} from "./backup.js";

const text = (value: string): Uint8Array => new TextEncoder().encode(value);
const salt = new Uint8Array(16).fill(7);

describe("backup blobs", () => {
  it("open only with the same key and name", async () => {
    const key = await createBackupKey();
    const blob = await sealBlob(key, "identity", text("master and self-signing keys"));

    expect(new TextDecoder().decode(await openBlob(key, "identity", blob))).toBe(
      "master and self-signing keys",
    );
    await expect(openBlob(key, "history/2026-11", blob)).rejects.toThrow(BackupError);
    await expect(openBlob(await createBackupKey(), "identity", blob)).rejects.toThrow(
      "the backup could not be opened",
    );
    const changed = blob.slice();
    changed[30] = (changed[30] ?? 0) ^ 1;
    await expect(openBlob(key, "identity", changed)).rejects.toThrow(BackupError);
  });

  it("use a fresh nonce each time", async () => {
    const key = await createBackupKey();

    expect(await sealBlob(key, "a", text("x"))).not.toEqual(await sealBlob(key, "a", text("x")));
  });
});

describe("the recovery phrase", () => {
  it("is 24 words that unlock the backup key", async () => {
    const backupKey = await createBackupKey();
    const phrase = createRecoveryPhrase();
    const wrapped = await wrapBackupKey(backupKey, "recovery-phrase", recoveryPhraseKey(phrase, salt));

    expect(phrase.split(" ")).toHaveLength(24);
    // Typed with different case and spacing.
    const typed = `  ${phrase.toUpperCase().split(" ").join("   ")}\n`;
    expect(await unwrapBackupKey(wrapped, "recovery-phrase", recoveryPhraseKey(typed, salt))).toEqual(
      backupKey,
    );
    await expect(
      unwrapBackupKey(wrapped, "recovery-phrase", recoveryPhraseKey(createRecoveryPhrase(), salt)),
    ).rejects.toThrow(BackupError);
    // The same phrase with another user's salt is another key.
    await expect(
      unwrapBackupKey(wrapped, "recovery-phrase", recoveryPhraseKey(phrase, new Uint8Array(16))),
    ).rejects.toThrow(BackupError);
  });

  it("catches mistyped and short phrases", () => {
    // BIP-39 test vector: 256 zero bits; "art" is the checksum word.
    const zero = `${"abandon ".repeat(23)}art`;
    const twelve = `${"abandon ".repeat(11)}about`;

    expect(normalizeRecoveryPhrase(zero)).toBe(zero);
    expect(normalizeRecoveryPhrase(`${"abandon ".repeat(23)}abandon`)).toBeUndefined();
    expect(normalizeRecoveryPhrase(zero.replace("art", "arts"))).toBeUndefined();
    // A valid 12-word phrase has only 128 bits and is not one of ours.
    expect(normalizeRecoveryPhrase(twelve)).toBeUndefined();
    expect(() => recoveryPhraseKey("not a phrase", salt)).toThrow("that is not a recovery phrase");
  });
});

describe("the password", () => {
  it("unlocks the backup key through Argon2id with the stored settings", async () => {
    const backupKey = await createBackupKey();
    const parameters = await newPasswordParameters(minimumPasswordMemory);
    const wrapped = await wrapBackupKey(
      backupKey,
      "password",
      await passwordKey("correct horse", parameters),
    );

    expect(
      await unwrapBackupKey(wrapped, "password", await passwordKey("correct horse", parameters)),
    ).toEqual(backupKey);
    await expect(
      unwrapBackupKey(wrapped, "password", await passwordKey("wrong horse", parameters)),
    ).rejects.toThrow(BackupError);
    // A key for one method does not open another method's copy.
    await expect(
      unwrapBackupKey(wrapped, "passkey", await passwordKey("correct horse", parameters)),
    ).rejects.toThrow(BackupError);
  });

  it("uses 256 MiB by default and refuses cheaper settings than 64 MiB", async () => {
    const parameters = await newPasswordParameters();

    expect(parameters).toMatchObject({ opsLimit: 3, memLimitBytes: 256 * 1024 * 1024 });
    expect(parameters.salt).toHaveLength(16);
    expect(await passwordKey("pw", parameters)).toHaveLength(32);
    await expect(
      passwordKey("pw", { ...parameters, memLimitBytes: minimumPasswordMemory - 1 }),
    ).rejects.toThrow("these password settings are too weak");
    await expect(passwordKey("pw", { ...parameters, opsLimit: 2 })).rejects.toThrow(BackupError);
  });
});

describe("the passkey", () => {
  it("unlocks the backup key from its PRF output", async () => {
    const backupKey = await createBackupKey();
    const prf = new Uint8Array(32).fill(1);
    const wrapped = await wrapBackupKey(backupKey, "passkey", passkeyKey(prf, salt));

    expect(await unwrapBackupKey(wrapped, "passkey", passkeyKey(prf, salt))).toEqual(backupKey);
    await expect(unwrapBackupKey(wrapped, "passkey", passkeyKey(new Uint8Array(32), salt))).rejects.toThrow(
      BackupError,
    );
    expect(() => passkeyKey(new Uint8Array(16), salt)).toThrow("the passkey gave too short a secret");
  });
});
