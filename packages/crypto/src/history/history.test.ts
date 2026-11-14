import {
  addMembers,
  createGroup,
  createMember,
  joinFromWelcome,
  removeMembers,
  updateKeys,
  type Group,
} from "../mls/engine.js";
import { exportSecret, ownLeaf, signAsMember, verifyMemberSignature } from "../mls/memberKeys.js";
import { ArchiveError, historyKey, openArchive, sealArchive, type ArchiveAddress } from "./archive.js";
import { decodeHistoryBundle, encodeHistoryBundle } from "./bundle.js";

const text = (value: string): Uint8Array => new TextEncoder().encode(value);
const read = (value: Uint8Array): string => new TextDecoder().decode(value);
const anyone = (): Promise<boolean> => Promise.resolve(true);
const address: ArchiveAddress = { channelId: text("channel-1"), messageId: text("message-1") };

async function twoMembers(): Promise<[Group, Group]> {
  const ada = await createMember(text("@ada@example.com/d1"));
  const grace = await createMember(text("@grace@example.com/d1"));
  const added = await addMembers(await createGroup(text("g"), ada), [grace.keyPackage]);
  return [added.group, await joinFromWelcome(added.welcome ?? new Uint8Array(), grace)];
}

describe("history keys", () => {
  it("are the same for every member and change with each epoch and channel", async () => {
    const [ada, grace] = await twoMembers();
    const key = await historyKey(ada, address.channelId);

    expect(key).toHaveLength(32);
    expect(await historyKey(grace, address.channelId)).toEqual(key);
    expect(await historyKey(ada, text("channel-2"))).not.toEqual(key);
    expect(await historyKey((await updateKeys(ada)).group, address.channelId)).not.toEqual(key);
    expect(await exportSecret(ada, "another label", new Uint8Array(0), 16)).toHaveLength(16);
  });
});

describe("archive copies", () => {
  it("open with the epoch's history key and name their sender", async () => {
    const [ada, grace] = await twoMembers();
    const copy = await sealArchive(ada, address, text("hello"));
    const opened = await openArchive(copy, address, await historyKey(grace, address.channelId), anyone);

    expect(copy.epoch).toBe(1n);
    expect(read(opened.plaintext)).toBe("hello");
    expect(read(opened.sender)).toBe("@ada@example.com/d1");
  });

  it("stay readable after the sender leaves and the group moves on", async () => {
    const [ada, grace] = await twoMembers();
    const key = await historyKey(grace, address.channelId);
    const copy = await sealArchive(ada, address, text("before"));
    const later = (await removeMembers(grace, [0])).group;

    expect(await historyKey(later, address.channelId)).not.toEqual(key);
    expect(read((await openArchive(copy, address, key, anyone)).plaintext)).toBe("before");
  });

  it("refuse the wrong key, place or version, changes, and unverified senders", async () => {
    const [ada] = await twoMembers();
    const key = await historyKey(ada, address.channelId);
    const copy = await sealArchive(ada, address, text("hello"));
    const changed = copy.sealed.slice();
    changed[20] = (changed[20] ?? 0) ^ 1;

    await expect(openArchive(copy, address, new Uint8Array(32), anyone)).rejects.toThrow(
      "could not be decrypted",
    );
    await expect(
      openArchive(copy, { ...address, messageId: text("message-2") }, key, anyone),
    ).rejects.toThrow(ArchiveError);
    await expect(openArchive({ ...copy, epoch: 2n }, address, key, anyone)).rejects.toThrow(ArchiveError);
    await expect(openArchive({ ...copy, sealed: changed }, address, key, anyone)).rejects.toThrow(
      ArchiveError,
    );
    await expect(
      openArchive({ ...copy, sealed: new Uint8Array([2, ...copy.sealed.slice(1)]) }, address, key, anyone),
    ).rejects.toThrow("this is not an archive copy this version can read");
    await expect(
      openArchive({ ...copy, sealed: copy.sealed.slice(0, 20) }, address, key, anyone),
    ).rejects.toThrow(ArchiveError);
    await expect(openArchive(copy, address, key, () => Promise.resolve(false))).rejects.toThrow(
      "the sender's credential could not be verified",
    );
  });

  it("refuse a copy whose signature is not the sender's", async () => {
    // Sealed correctly but carrying a signature over something else.
    const [ada] = await twoMembers();
    const key = await historyKey(ada, address.channelId);
    const forged = await sealWithInner(ada, key, await innerWithBadSignature(ada));

    await expect(openArchive(forged, address, key, anyone)).rejects.toThrow(
      "the sender's signature does not match",
    );
    await expect(
      openArchive(await sealWithInner(ada, key, text("not fields")), address, key, anyone),
    ).rejects.toThrow("the archive copy is malformed");
  });
});

describe("member signatures", () => {
  it("verify with the signer's key only, and never throw on bad input", async () => {
    const [ada, grace] = await twoMembers();
    const signature = await signAsMember(ada, "label", text("content"));

    expect(
      await verifyMemberSignature(ownLeaf(ada).signaturePublicKey, "label", text("content"), signature),
    ).toBe(true);
    expect(
      await verifyMemberSignature(ownLeaf(grace).signaturePublicKey, "label", text("content"), signature),
    ).toBe(false);
    expect(
      await verifyMemberSignature(ownLeaf(ada).signaturePublicKey, "other", text("content"), signature),
    ).toBe(false);
    expect(await verifyMemberSignature(new Uint8Array(3), "label", text("content"), signature)).toBe(false);
  });

  it("need a basic-credential leaf for this device", async () => {
    const [ada] = await twoMembers();
    const broken: Group = { state: { ...ada.state, ratchetTree: [] } };

    expect(() => ownLeaf(broken)).toThrow("no basic-credential leaf");
  });
});

describe("history bundles", () => {
  it("round-trip and refuse anything else", () => {
    const entries = [
      { groupId: new Uint8Array(16).fill(9), epoch: 1n, key: new Uint8Array(32).fill(1) },
      { groupId: new Uint8Array(16).fill(8), epoch: 7n, key: new Uint8Array(32).fill(7) },
    ];
    const bytes = encodeHistoryBundle(text("channel-1"), entries);

    expect(decodeHistoryBundle(bytes)).toEqual({ channelId: text("channel-1"), entries });
    expect(decodeHistoryBundle(encodeHistoryBundle(text("c"), []))).toEqual({
      channelId: text("c"),
      entries: [],
    });
    expect(decodeHistoryBundle(bytes.slice(0, -1))).toBeUndefined();
    expect(decodeHistoryBundle(text("hello"))).toBeUndefined();
    const groupId = new Uint8Array(16);
    expect(() => encodeHistoryBundle(text("c"), [{ groupId, epoch: 1n, key: new Uint8Array(16) }])).toThrow(
      RangeError,
    );
    expect(() =>
      encodeHistoryBundle(text("c"), [{ groupId: new Uint8Array(8), epoch: 1n, key: new Uint8Array(32) }]),
    ).toThrow(RangeError);
  });

  it("refuse another label or a record of the wrong size", async () => {
    const { lengthPrefixed } = await import("../encoding.js");

    expect(
      decodeHistoryBundle(lengthPrefixed(text("other v1"), text("c"), new Uint8Array(56))),
    ).toBeUndefined();
    expect(
      decodeHistoryBundle(
        lengthPrefixed(text("nexus-chat history bundle v1"), text("c"), new Uint8Array(55)),
      ),
    ).toBeUndefined();
    // The label is fixed: a bundle built by hand with it is read.
    expect(
      decodeHistoryBundle(lengthPrefixed(text("nexus-chat history bundle v1"), text("c"), new Uint8Array(56)))
        ?.entries,
    ).toHaveLength(1);
  });
});

// Helpers that build archive copies by hand, to test what openArchive refuses.
async function innerWithBadSignature(group: Group): Promise<Uint8Array> {
  const { lengthPrefixed } = await import("../encoding.js");
  const sender = ownLeaf(group);
  const signature = await signAsMember(group, "nexus-chat archive v1", text("something else"));
  return lengthPrefixed(text("hello"), sender.identity, sender.signaturePublicKey, signature);
}

async function sealWithInner(
  group: Group,
  key: Uint8Array,
  inner: Uint8Array,
): Promise<{ epoch: bigint; sealed: Uint8Array }> {
  const { lengthPrefixed, uint64 } = await import("../encoding.js");
  const epochNow = group.state.groupContext.epoch;
  const additionalData = lengthPrefixed(
    text("nexus-chat archive v1"),
    address.channelId,
    uint64(epochNow),
    address.messageId,
  );
  const nonce = new Uint8Array(12);
  const cryptoKey = await crypto.subtle.importKey("raw", new Uint8Array(key), "AES-GCM", false, ["encrypt"]);
  const ciphertext = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: nonce, additionalData: new Uint8Array(additionalData) },
    cryptoKey,
    new Uint8Array(inner),
  );
  return { epoch: epochNow, sealed: new Uint8Array([1, ...nonce, ...new Uint8Array(ciphertext)]) };
}
