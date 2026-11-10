import sodium from "libsodium-wrappers-sumo";

import {
  certifyDevice,
  createIdentity,
  IdentityError,
  rotateSelfSigningKey,
  safetyNumber,
  verifyDevice,
  type NewIdentity,
  type PublishedIdentity,
} from "./identity.js";

const published = (
  identity: Pick<NewIdentity, "selfSigningKey" | "selfSigningSignature">,
  master: Uint8Array,
): PublishedIdentity => ({
  masterPublicKey: master,
  selfSigningPublicKey: identity.selfSigningKey.publicKey,
  selfSigningSignature: identity.selfSigningSignature,
});

async function deviceKey(): Promise<Uint8Array> {
  await sodium.ready;
  return sodium.crypto_sign_keypair().publicKey;
}

describe("a user's identity", () => {
  it("certifies devices that others can check back to the master key", async () => {
    const user = await createIdentity();
    const device = await certifyDevice(user.selfSigningKey, "@ada@example.com/d1", await deviceKey());

    expect(user.masterKey.publicKey).toHaveLength(32);
    await expect(verifyDevice(published(user, user.masterKey.publicKey), device)).resolves.toBeUndefined();
  });

  it("refuses devices certified by someone else or for another identity", async () => {
    const user = await createIdentity();
    const mallory = await createIdentity();
    const key = await deviceKey();
    const forged = await certifyDevice(mallory.selfSigningKey, "@ada@example.com/d1", key);
    const genuine = await certifyDevice(user.selfSigningKey, "@ada@example.com/d1", key);
    const ada = published(user, user.masterKey.publicKey);

    await expect(verifyDevice(ada, forged)).rejects.toThrow("the device is not signed by its user");
    await expect(verifyDevice(ada, { ...genuine, identity: "@ada@example.com/d2" })).rejects.toThrow(
      IdentityError,
    );
    await expect(verifyDevice(ada, { ...genuine, signaturePublicKey: await deviceKey() })).rejects.toThrow(
      IdentityError,
    );
    await expect(verifyDevice(ada, { ...genuine, signature: new Uint8Array(3) })).rejects.toThrow(
      IdentityError,
    );
  });

  it("refuses a self-signing key the master key did not sign", async () => {
    const user = await createIdentity();
    const mallory = await createIdentity();
    const device = await certifyDevice(mallory.selfSigningKey, "@ada@example.com/d1", await deviceKey());

    // Mallory's self-signing key presented under Ada's master key.
    await expect(verifyDevice(published(mallory, user.masterKey.publicKey), device)).rejects.toThrow(
      "the self-signing key is not signed by the master key",
    );
  });

  it("keeps a device signature from passing as a self-signing signature", async () => {
    const user = await createIdentity();
    // The master key signs a device statement whose key is a self-signing key.
    const device = await certifyDevice(user.masterKey, "", user.selfSigningKey.publicKey);

    await expect(
      verifyDevice(
        { ...published(user, user.masterKey.publicKey), selfSigningSignature: device.signature },
        device,
      ),
    ).rejects.toThrow("the self-signing key is not signed by the master key");
  });

  it("rotates the self-signing key under the same master key", async () => {
    const user = await createIdentity();
    const rotated = await rotateSelfSigningKey(user.masterKey);
    const old = await certifyDevice(user.selfSigningKey, "@ada@example.com/d1", await deviceKey());
    const renewed = await certifyDevice(
      rotated.selfSigningKey,
      "@ada@example.com/d1",
      old.signaturePublicKey,
    );
    const now = published(rotated, user.masterKey.publicKey);

    expect(rotated.selfSigningKey.publicKey).not.toEqual(user.selfSigningKey.publicKey);
    await expect(verifyDevice(now, renewed)).resolves.toBeUndefined();
    await expect(verifyDevice(now, old)).rejects.toThrow(IdentityError);
  });
});

describe("the safety number", () => {
  it("is 12 groups of 5 digits that differ between users", async () => {
    const first = await createIdentity();
    const second = await createIdentity();
    const number = safetyNumber(first.masterKey.publicKey);

    expect(number).toMatch(/^(\d{5} ){11}\d{5}$/);
    expect(safetyNumber(first.masterKey.publicKey)).toBe(number);
    expect(safetyNumber(second.masterKey.publicKey)).not.toBe(number);
  });

  it("stays the same across versions", () => {
    // Changing how it is derived would make every verified contact look changed.
    expect(safetyNumber(new Uint8Array(32))).toMatchSnapshot();
  });
});
