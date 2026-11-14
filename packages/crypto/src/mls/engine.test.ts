import {
  addMembers,
  createGroup,
  createMember,
  encrypt,
  epoch,
  groupId,
  epochAuthenticator,
  joinFromWelcome,
  members,
  MlsMessageError,
  receive,
  removeMembers,
  updateKeys,
  type Group,
  type Member,
} from "./engine.js";
import { defaultCapabilities, defaultLifetime, encodeMlsMessage, generateKeyPackage } from "ts-mls";

import { ciphersuite } from "./ciphersuite.js";

const text = (value: string): Uint8Array => new TextEncoder().encode(value);
const read = (value: Uint8Array): string => new TextDecoder().decode(value);

async function commitApplied(group: Group, commit: Uint8Array): Promise<Group> {
  const received = await receive(group, commit);
  expect(received.kind).toBe("commit");
  return received.group;
}

async function plaintextOf(group: Group, message: Uint8Array): Promise<[Group, string]> {
  const received = await receive(group, message);
  if (received.kind !== "message") {
    throw new Error("expected an application message");
  }
  return [received.group, read(received.plaintext)];
}

describe("an MLS group", () => {
  it("adds members, delivers messages, and locks out removed members", async () => {
    const aliceDevice = await createMember(text("alice"));
    const bobDevice = await createMember(text("bob"));
    const carolDevice = await createMember(text("carol"));
    let alice = await createGroup(text("channel-1"), aliceDevice);
    expect(epoch(alice)).toBe(0n);
    expect(groupId(alice)).toEqual(text("channel-1"));

    const added = await addMembers(alice, [bobDevice.keyPackage, carolDevice.keyPackage]);
    alice = added.group;
    let bob = await joinFromWelcome(added.welcome ?? new Uint8Array(), bobDevice);
    let carol = await joinFromWelcome(added.welcome ?? new Uint8Array(), carolDevice);
    expect(epoch(alice)).toBe(1n);
    expect(epochAuthenticator(bob)).toEqual(epochAuthenticator(alice));
    expect([...members(alice).values()].map(read)).toEqual(["alice", "bob", "carol"]);

    const hello = await encrypt(alice, text("hello"));
    alice = hello.group;
    [bob] = await plaintextOf(bob, hello.message).then(([group, plain]) => {
      expect(plain).toBe("hello");
      return [group];
    });
    [carol] = await plaintextOf(carol, hello.message).then(([group, plain]) => {
      expect(plain).toBe("hello");
      return [group];
    });

    const removed = await removeMembers(alice, [2]);
    alice = removed.group;
    expect(removed.welcome).toBeUndefined();
    bob = await commitApplied(bob, removed.commit);
    expect([...members(bob).keys()]).toEqual([0, 1]);
    expect(epochAuthenticator(bob)).toEqual(epochAuthenticator(alice));

    const refreshed = await updateKeys(alice);
    alice = refreshed.group;
    bob = await commitApplied(bob, refreshed.commit);
    expect(epoch(bob)).toBe(3n);

    const secret = await encrypt(alice, text("after carol left"));
    const [, plain] = await plaintextOf(bob, secret.message);
    expect(plain).toBe("after carol left");
    // Carol, still at the old epoch, cannot read the new epoch's messages.
    await expect(receive(carol, secret.message)).rejects.toThrow();
  });

  it("refuses bytes that are not the message expected", async () => {
    const device = await createMember(text("dave"));
    const group = await createGroup(text("g"), device);

    await expect(receive(group, new Uint8Array([1, 2, 3]))).rejects.toThrow(MlsMessageError);
    await expect(receive(group, device.keyPackage)).rejects.toThrow("expected a group message");
    await expect(addMembers(group, [new Uint8Array([0])])).rejects.toThrow(MlsMessageError);
    const welcomeFromCommit = (await addMembers(group, [(await createMember(text("erin"))).keyPackage]))
      .commit;
    await expect(joinFromWelcome(welcomeFromCommit, device)).rejects.toThrow("expected a welcome");
    await expect(addMembers(group, [welcomeFromCommit])).rejects.toThrow("expected a key package");
  });
});

describe("message senders", () => {
  it("are named by the credential MLS authenticated, from the epoch the message was sent in", async () => {
    const ada = await createMember(text("ada"));
    const grace = await createMember(text("grace"));
    const added = await addMembers(await createGroup(text("g"), ada), [grace.keyPackage]);
    const graceGroup = await joinFromWelcome(added.welcome ?? new Uint8Array(), grace);
    const first = await encrypt(graceGroup, text("first"));
    const second = await encrypt(first.group, text("second"));

    const hello = await receive(added.group, first.message);
    expect(hello.kind === "message" && read(hello.sender ?? new Uint8Array())).toBe("grace");
    // Ada removes Grace before reading her second message, which was sent
    // in the earlier epoch: it is read with that epoch's keys and tree.
    const removed = await removeMembers(hello.group, [1]);
    const late = await receive(removed.group, second.message);
    expect(late.kind === "message" && read(late.sender ?? new Uint8Array())).toBe("grace");
  });

  it("are left unnamed when the credential is not a basic one", async () => {
    const suite = await ciphersuite();
    const generated = await generateKeyPackage(
      { credentialType: "x509", certificates: [new Uint8Array([1])] },
      defaultCapabilities(),
      defaultLifetime,
      [],
      suite,
    );
    const x509: Member = {
      keyPackage: encodeMlsMessage({
        version: "mls10",
        wireformat: "mls_key_package",
        keyPackage: generated.publicPackage,
      }),
      publicPackage: generated.publicPackage,
      secrets: generated.privatePackage,
    };
    const added = await addMembers(await createGroup(text("g"), await createMember(text("ada"))), [
      x509.keyPackage,
    ]);
    const theirs = await joinFromWelcome(added.welcome ?? new Uint8Array(), x509);
    const received = await receive(added.group, (await encrypt(theirs, text("hi"))).message);

    expect(received.kind === "message" && received.sender).toBeUndefined();
  });
});
