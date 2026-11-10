import { defaultCapabilities, defaultLifetime, encodeMlsMessage, generateKeyPackage } from "ts-mls";

import { ciphersuite } from "../mls/ciphersuite.js";
import {
  addMembers,
  createGroup,
  createMember,
  createSignatureKeys,
  joinFromWelcome,
  members,
  MlsMessageError,
  type Member,
} from "../mls/engine.js";
import {
  decodeDeviceCredential,
  deviceCredentialCheck,
  encodeDeviceCredential,
  userOf,
  type TrustedIdentityLookup,
} from "./credential.js";
import { certifyDevice, createIdentity, type NewIdentity, type PublishedIdentity } from "./identity.js";

const publishedOf = (user: NewIdentity): PublishedIdentity => ({
  masterPublicKey: user.masterKey.publicKey,
  selfSigningPublicKey: user.selfSigningKey.publicKey,
  selfSigningSignature: user.selfSigningSignature,
});

/** A device of `user` with a certified signature key, as the app makes them. */
async function device(user: NewIdentity, identity: string): Promise<Member> {
  const keys = await createSignatureKeys();
  const certificate = await certifyDevice(user.selfSigningKey, identity, keys.publicKey);
  return createMember(encodeDeviceCredential({ identity, certificate: certificate.signature }), keys);
}

describe("device credentials", () => {
  it("round-trip and refuse anything else", () => {
    const credential = { identity: "@ada@example.com/d1", certificate: new Uint8Array([1, 2, 3]) };
    const bytes = encodeDeviceCredential(credential);

    expect(decodeDeviceCredential(bytes)).toEqual(credential);
    expect(decodeDeviceCredential(new TextEncoder().encode("@ada@example.com"))).toBeUndefined();
    expect(decodeDeviceCredential(new Uint8Array([0, 0]))).toBeUndefined();
    // Truncated in the certificate, cut after the label, and with bytes left over.
    expect(decodeDeviceCredential(bytes.slice(0, -1))).toBeUndefined();
    expect(decodeDeviceCredential(bytes.slice(0, 4 + 31))).toBeUndefined();
    expect(decodeDeviceCredential(new Uint8Array([...bytes, 0]))).toBeUndefined();
    const relabelled = bytes.slice();
    relabelled[5] = 0x4e;
    expect(decodeDeviceCredential(relabelled)).toBeUndefined();
  });

  it("name their user", () => {
    expect(userOf("@ada@example.com/d1")).toBe("@ada@example.com");
    expect(userOf("@ada@example.com")).toBeUndefined();
    expect(userOf("ada/d1")).toBeUndefined();
    expect(userOf("@ada@example.com/d1/extra")).toBeUndefined();
  });
});

describe("a group that checks credentials", () => {
  it("admits devices certified by their users and refuses others", async () => {
    const ada = await createIdentity();
    const grace = await createIdentity();
    const mallory = await createIdentity();
    const trusted = new Map([
      ["@ada@example.com", publishedOf(ada)],
      ["@grace@example.com", publishedOf(grace)],
    ]);
    const lookup: TrustedIdentityLookup = (user) => Promise.resolve(trusted.get(user));
    const check = deviceCredentialCheck(lookup);

    const adaDevice = await device(ada, "@ada@example.com/d1");
    const graceDevice = await device(grace, "@grace@example.com/d1");
    let group = await createGroup(new TextEncoder().encode("g"), adaDevice, check);
    const added = await addMembers(group, [graceDevice.keyPackage]);
    group = added.group;
    const joined = await joinFromWelcome(added.welcome ?? new Uint8Array(), graceDevice, check);
    expect([...members(joined).values()].map((bytes) => decodeDeviceCredential(bytes)?.identity)).toEqual([
      "@ada@example.com/d1",
      "@grace@example.com/d1",
    ]);

    // Mallory signs a device claiming to be Grace's; a device of an unknown user; plain bytes.
    const forged = await device(mallory, "@grace@example.com/d2");
    const stranger = await device(mallory, "@mallory@example.com/d1");
    const plain = await createMember(new TextEncoder().encode("@grace@example.com/d3"));
    // And a credential that is not a BasicCredential at all.
    const x509 = await generateKeyPackage(
      { credentialType: "x509", certificates: [new Uint8Array([1])] },
      defaultCapabilities(),
      defaultLifetime,
      [],
      await ciphersuite(),
    );
    const x509Package = encodeMlsMessage({
      version: "mls10",
      wireformat: "mls_key_package",
      keyPackage: x509.publicPackage,
    });
    for (const keyPackage of [forged.keyPackage, stranger.keyPackage, plain.keyPackage, x509Package]) {
      await expect(addMembers(group, [keyPackage])).rejects.toThrow();
    }
  });

  it("refuses to join a group with a member it can't verify", async () => {
    const ada = await createIdentity();
    const grace = await createIdentity();
    const adaDevice = await device(ada, "@ada@example.com/d1");
    const graceDevice = await device(grace, "@grace@example.com/d1");
    // Ada's side doesn't check; Grace knows only herself, so Ada is unverified.
    const group = await createGroup(new TextEncoder().encode("g"), adaDevice);
    const added = await addMembers(group, [graceDevice.keyPackage]);
    const onlyGrace = deviceCredentialCheck((user) =>
      Promise.resolve(user === "@grace@example.com" ? publishedOf(grace) : undefined),
    );

    await expect(
      joinFromWelcome(added.welcome ?? new Uint8Array(), graceDevice, onlyGrace),
    ).rejects.toThrow();
    // A key package is not a welcome, whatever the check.
    await expect(joinFromWelcome(graceDevice.keyPackage, graceDevice, onlyGrace)).rejects.toThrow(
      MlsMessageError,
    );
  });
});
