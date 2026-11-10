/**
 * Who this device trusts (docs/e2ee-design.md §2): each user's master key,
 * pinned the first time it is seen. A later, different master key is not
 * trusted until the user accepts the change, and the conversation shows a
 * warning. `credentialCheck` is what every MLS group runs on its members.
 */
import { deviceCredentialCheck, type CredentialCheck, type PublishedIdentity } from "@nexus-chat/crypto";

import type { LocalDatabase } from "../../storage/localDatabase";
import type { MessagingApi } from "./api";
import { fromBase64, toBase64, utf8 } from "./bytes";

/** Maps `@name@instance` to the user's ID; members of the conversation are known. */
export type UserResolver = (user: string) => string | undefined;

/** Called when a user's master key differs from the pinned one. */
export type IdentityChangeListener = (userId: string) => void;

const pins = "pins";

/** The identity to trust for `userId`: the pinned one, or the published one, pinned now. */
export async function trustedIdentity(
  api: MessagingApi,
  database: LocalDatabase,
  userId: string,
  onChange: IdentityChangeListener,
): Promise<PublishedIdentity | undefined> {
  const published = await api.getIdentity(userId);
  if (published === undefined) {
    return undefined;
  }
  const pinned = await database.get(pins, userId);
  if (pinned === undefined) {
    await database.put(pins, userId, utf8(toBase64(published.masterPublicKey)));
    return published;
  }
  if (new TextDecoder().decode(pinned) !== toBase64(published.masterPublicKey)) {
    onChange(userId);
    return undefined;
  }
  return published;
}

/** Accepts a user's new master key, after they started a new identity. */
export async function acceptIdentityChange(
  api: MessagingApi,
  database: LocalDatabase,
  userId: string,
): Promise<void> {
  const published = await api.getIdentity(userId);
  if (published !== undefined) {
    await database.put(pins, userId, utf8(toBase64(published.masterPublicKey)));
  }
}

/** The credential check for this device's groups. */
export function credentialCheck(
  api: MessagingApi,
  database: LocalDatabase,
  resolve: UserResolver,
  onChange: IdentityChangeListener,
): CredentialCheck {
  return deviceCredentialCheck(async (user) => {
    const userId = resolve(user);
    return userId === undefined ? undefined : trustedIdentity(api, database, userId, onChange);
  });
}

/** The pinned master key of a user, for showing their safety number. */
export async function pinnedMasterKey(
  database: LocalDatabase,
  userId: string,
): Promise<Uint8Array | undefined> {
  const pinned = await database.get(pins, userId);
  return pinned === undefined ? undefined : fromBase64(new TextDecoder().decode(pinned));
}
