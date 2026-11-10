import {
  addMembers,
  createGroup,
  createMember,
  encrypt,
  epoch,
  joinFromWelcome,
  members,
  MlsMessageError,
  receive,
  type Group,
} from "./engine.js";
import { joinExternal, loadGroup, loadMember, saveGroup, saveMember } from "./persistence.js";

const text = (value: string): Uint8Array => new TextEncoder().encode(value);
const read = (value: Uint8Array): string => new TextDecoder().decode(value);

async function plaintextOf(group: Group, message: Uint8Array): Promise<string> {
  const received = await receive(group, message);
  if (received.kind !== "message") {
    throw new Error("expected an application message");
  }
  return read(received.plaintext);
}

describe("saved MLS state", () => {
  it("keeps a group working after a restart", async () => {
    const ada = await createMember(text("ada"));
    const grace = await createMember(text("grace"));
    const added = await addMembers(await createGroup(text("g"), ada), [grace.keyPackage]);
    const graceGroup = await joinFromWelcome(added.welcome ?? new Uint8Array(), grace);

    const restored = loadGroup(saveGroup(added.group));
    expect(epoch(restored)).toBe(1n);
    const hello = await encrypt(restored, text("after a restart"));
    expect(await plaintextOf(graceGroup, hello.message)).toBe("after a restart");
  });

  it("keeps a member's key package and private keys", async () => {
    const grace = await createMember(text("grace"));
    const restored = loadMember(saveMember(grace));

    expect(restored.keyPackage).toEqual(grace.keyPackage);
    expect(restored.secrets).toEqual(grace.secrets);
    // A restored member can still join with its key package.
    const added = await addMembers(await createGroup(text("g"), await createMember(text("ada"))), [
      grace.keyPackage,
    ]);
    await expect(joinFromWelcome(added.welcome ?? new Uint8Array(), restored)).resolves.toBeDefined();
  });

  it("refuses bytes that are not saved state", async () => {
    const ada = await createMember(text("ada"));

    expect(() => loadGroup(new Uint8Array([1, 2, 3]))).toThrow(MlsMessageError);
    expect(() => loadMember(new Uint8Array([1, 2, 3]))).toThrow("not a saved member");
    // Four fields, but the first is not a key package.
    const saved = saveMember(ada);
    const wrong = saveMember({ ...ada, keyPackage: (await createGroupInfo()).slice() });
    expect(() => loadMember(wrong)).toThrow("not a saved member");
    expect(loadMember(saved).keyPackage).toEqual(ada.keyPackage);
  });
});

async function createGroupInfo(): Promise<Uint8Array> {
  const added = await addMembers(await createGroup(text("g"), await createMember(text("ada"))), [
    (await createMember(text("bob"))).keyPackage,
  ]);
  return added.groupInfo;
}

describe("joining by external commit", () => {
  it("lets another device enter from the GroupInfo", async () => {
    const ada = await createMember(text("ada"));
    const grace = await createMember(text("grace"));
    const added = await addMembers(await createGroup(text("g"), ada), [grace.keyPackage]);
    const graceGroup = await joinFromWelcome(added.welcome ?? new Uint8Array(), grace);
    const adaPhone = await createMember(text("ada-phone"));

    const joined = await joinExternal(added.groupInfo, adaPhone);
    const adaGroup = await receive(added.group, joined.commit);
    const graceAfter = await receive(graceGroup, joined.commit);
    expect(adaGroup.kind).toBe("commit");
    expect(graceAfter.kind).toBe("commit");
    expect([...members(joined.group).values()].map(read)).toEqual(["ada", "grace", "ada-phone"]);

    const hello = await encrypt(joined.group, text("from my phone"));
    expect(await plaintextOf(graceAfter.group, hello.message)).toBe("from my phone");
  });

  it("needs a GroupInfo", async () => {
    const ada = await createMember(text("ada"));
    await expect(joinExternal(ada.keyPackage, ada)).rejects.toThrow("expected a group info");
  });
});
