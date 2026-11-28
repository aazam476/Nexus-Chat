import { lengthPrefixed, uint64 } from "../encoding.js";
import { addMembers, createGroup, createMember, joinFromWelcome, type Group } from "../mls/engine.js";
import {
  ArchiveError,
  archiveEvidence,
  decodeEvidence,
  encodeEvidence,
  historyKey,
  sealArchive,
  verifyEvidence,
  type ArchiveAddress,
  type ArchiveEvidence,
} from "./archive.js";

const text = (value: string): Uint8Array => new TextEncoder().encode(value);
const read = (value: Uint8Array): string => new TextDecoder().decode(value);
const anyone = (): Promise<boolean> => Promise.resolve(true);
const address: ArchiveAddress = { channelId: text("channel-1"), messageId: text("message-1") };

async function threeMembers(): Promise<[Group, Group, Group]> {
  const ada = await createMember(text("@ada@example.com/d1"));
  const grace = await createMember(text("@grace@example.com/d1"));
  const sam = await createMember(text("@sam@example.com/d1"));
  const added = await addMembers(await createGroup(text("g"), ada), [grace.keyPackage, sam.keyPackage]);
  const welcome = added.welcome ?? new Uint8Array();
  return [added.group, await joinFromWelcome(welcome, grace), await joinFromWelcome(welcome, sam)];
}

/** Evidence of Ada's message "you are all fools", as Grace would report it. */
async function reported(): Promise<{ evidence: ArchiveEvidence; sam: Group; grace: Group }> {
  const [ada, grace, sam] = await threeMembers();
  const copy = await sealArchive(ada, address, text("you are all fools"));
  const evidence = await archiveEvidence(copy, address, await historyKey(grace, address.channelId));
  return { evidence, sam, grace };
}

describe("evidence for a report", () => {
  it("proves who sent what, where, to someone without the history key", async () => {
    const { evidence } = await reported();
    expect(read(evidence.plaintext)).toBe("you are all fools");
    expect(read(evidence.sender)).toBe("@ada@example.com/d1");
    expect(evidence.epoch).toBe(1n);

    const handed = decodeEvidence(encodeEvidence(evidence));
    expect(handed).toEqual(evidence);
    expect(handed !== undefined && (await verifyEvidence(handed, anyone))).toBe(true);
  });

  it("is refused when the plaintext, place, sender or key were changed", async () => {
    const { evidence, grace } = await reported();
    const forgeries: ArchiveEvidence[] = [
      { ...evidence, plaintext: text("I agree with everyone") },
      { ...evidence, address: { ...address, messageId: text("message-2") } },
      { ...evidence, address: { ...address, channelId: text("channel-2") } },
      { ...evidence, epoch: 2n },
      { ...evidence, sender: text("@grace@example.com/d1") },
      { ...evidence, signature: new Uint8Array(64) },
    ];
    for (const forged of forgeries) {
      expect(await verifyEvidence(forged, anyone)).toBe(false);
    }
    // A reporter's own message, signed by them, names them as its sender.
    const own = await sealArchive(grace, address, text("you are all fools"));
    const theirs = await archiveEvidence(own, address, await historyKey(grace, address.channelId));
    expect(read(theirs.sender)).toBe("@grace@example.com/d1");
    // And the credential check still has its say.
    expect(await verifyEvidence(evidence, () => Promise.resolve(false))).toBe(false);
  });

  it("cannot be made from a copy the history key does not open, or one tampered with", async () => {
    const { sam } = await reported();
    const [ada] = await threeMembers();
    const copy = await sealArchive(ada, address, text("hello"));
    await expect(archiveEvidence(copy, address, await historyKey(sam, address.channelId))).rejects.toThrow(
      ArchiveError,
    );
  });

  it("decodes only what was encoded as evidence", async () => {
    const { evidence } = await reported();
    const encoded = encodeEvidence(evidence);
    expect(decodeEvidence(encoded.slice(0, 20))).toBeUndefined();
    expect(decodeEvidence(new Uint8Array([0, 0]))).toBeUndefined();
    const fields = [
      address.channelId,
      address.messageId,
      uint64(1n),
      text("p"),
      text("s"),
      text("k"),
      text("x"),
    ];
    expect(decodeEvidence(lengthPrefixed(text("another label"), ...fields))).toBeUndefined();
    expect(
      decodeEvidence(
        lengthPrefixed(
          text("nexus-chat archive v1"),
          address.channelId,
          address.messageId,
          text("short"),
          ...fields.slice(3),
        ),
      ),
    ).toBeUndefined();
    expect(decodeEvidence(lengthPrefixed(text("nexus-chat archive v1"), ...fields))?.epoch).toBe(1n);
  });
});
