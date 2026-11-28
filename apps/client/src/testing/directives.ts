/**
 * Directives signed as the instance signs them, for tests of what devices
 * do with them (docs/e2ee-design.md §3.2, spec §4.7).
 */
import sodium from "libsodium-wrappers-sumo";

import { toBase64, utf8 } from "../messaging/engine/bytes";
import type { FakeInstance } from "./fakeDelivery";

const label = "nexus-chat membership directive v1\n";

/** Gives the instance a signing key, and returns the private half. */
export async function signingInstance(instance: FakeInstance): Promise<Uint8Array> {
  await sodium.ready;
  const keys = sodium.crypto_sign_keypair();
  instance.instanceKey = new Uint8Array(keys.publicKey);
  return keys.privateKey;
}

/** Delivers a directive to the group, signed with `key`. */
export function deliverDirective(
  instance: FakeInstance,
  groupId: string,
  directive: object,
  key: Uint8Array,
): void {
  const bytes = utf8(JSON.stringify(directive));
  const signature = sodium.crypto_sign_detached(new Uint8Array([...utf8(label), ...bytes]), key);
  instance.deliver(groupId, {
    kind: "directive",
    epoch: instance.groups.get(groupId)?.epoch ?? 0,
    data: utf8(JSON.stringify({ directive: toBase64(bytes), signature: toBase64(signature) })),
  });
}

/** Deletes a channel message as a moderator would: its archive copy goes, and a directive tells devices. */
export function moderatorDeletes(
  instance: FakeInstance,
  where: { audienceId: string; channelId: string },
  message: { id: string; senderId: string },
  key: Uint8Array,
): void {
  const copies = instance.channelArchives.get(where.channelId) ?? [];
  instance.channelArchives.set(
    where.channelId,
    copies.filter((copy) => copy.messageId !== message.id),
  );
  const directive = {
    type: "delete",
    messageId: message.id,
    channelId: where.channelId,
    userId: message.senderId,
  };
  deliverDirective(instance, where.audienceId, directive, key);
}

/** Deletes what a user sent to a channel since a time, as a ban does. */
export function banPurges(
  instance: FakeInstance,
  where: { audienceId: string; channelId: string },
  purge: { userId: string; since: string },
  key: Uint8Array,
): void {
  const copies = instance.channelArchives.get(where.channelId) ?? [];
  instance.channelArchives.set(
    where.channelId,
    copies.filter((copy) => copy.senderId !== purge.userId || copy.createdAt < purge.since),
  );
  deliverDirective(instance, where.audienceId, { type: "purge", ...purge }, key);
}
