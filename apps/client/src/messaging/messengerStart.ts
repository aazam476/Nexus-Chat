/**
 * Starting messaging on a signed-in device: setting up or restoring the
 * user's identity, this device's keys and key packages, then the messenger.
 */
import type { LocalDatabase } from "../storage/localDatabase";
import { utf8 } from "./engine/bytes";
import { refillKeyPackages, setUpDevice } from "./engine/device";
import { restoreWithPhrase, setUpIdentity, type StoredIdentity } from "./engine/identity";
import { Messenger, type MessengerSetup, type MessengerStart } from "./messenger";

async function ready(setup: MessengerSetup, identity: StoredIdentity): Promise<Messenger> {
  const { api, database, user, where } = setup;
  const device = await setUpDevice(database, identity, {
    user: `@${user.username}@${where.domain}`,
    deviceId: where.deviceId,
  });
  await refillKeyPackages(api, database, device);
  const messenger = new Messenger(
    { api, database, device, instanceKey: setup.instanceKey, backupKey: identity.backupKey },
    where.domain,
  );
  messenger.learn([user]);
  return messenger;
}

const unconfirmed = { collection: "keys", id: "unconfirmed-recovery-phrase" } as const;

/**
 * Sets up messaging on this device, or says the identity must be restored
 * first. A new recovery phrase is kept on the device until the user
 * confirms they wrote it down, so closing the app does not lose it.
 */
export async function startMessenger(setup: MessengerSetup): Promise<MessengerStart> {
  const { database } = setup;
  const result = await setUpIdentity(setup.api, database, setup.user.id);
  if (result.status === "needs-restore") {
    return result;
  }
  if (result.recoveryPhrase !== undefined) {
    await database.put(unconfirmed.collection, unconfirmed.id, utf8(result.recoveryPhrase));
  }
  const messenger = await ready(setup, result.identity);
  const pending = await database.get(unconfirmed.collection, unconfirmed.id);
  return pending === undefined
    ? { status: "ready", messenger }
    : { status: "ready", messenger, recoveryPhrase: new TextDecoder().decode(pending) };
}

/** The user wrote the recovery phrase down: the device forgets it. */
export async function confirmRecoveryPhrase(database: LocalDatabase): Promise<void> {
  await database.delete(unconfirmed.collection, unconfirmed.id);
}

/** Restores the identity from the recovery phrase, then sets up messaging. */
export async function restoreMessenger(setup: MessengerSetup, phrase: string): Promise<Messenger> {
  const identity = await restoreWithPhrase(setup.api, setup.database, setup.user.id, phrase);
  return ready(setup, identity);
}
