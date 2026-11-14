/**
 * Devices for messaging-engine tests, set up the way the app sets one up
 * after login, on a shared in-memory instance.
 */
import sodium from "libsodium-wrappers-sumo";

import type { EngineContext } from "../messaging/engine/context";
import { refillKeyPackages, setUpDevice } from "../messaging/engine/device";
import { restoreWithPhrase, setUpIdentity, type StoredIdentity } from "../messaging/engine/identity";
import { credentialCheck } from "../messaging/engine/trust";
import { createMemoryDatabase } from "../storage/localDatabase";
import type { FakeInstance } from "./fakeDelivery";

/** The test users, by the user part of their devices' addresses. */
export const users: Record<string, string> = {
  "@ada@test": "user-ada",
  "@grace@test": "user-grace",
  "@alan@test": "user-alan",
};

/** A set-up device. */
export interface TestDevice {
  context: EngineContext;
  identity: StoredIdentity;
  phrase?: string;
}

/** An instance key pair, for signing directives. */
export async function instanceKeys(): Promise<{ publicKey: Uint8Array; privateKey: Uint8Array }> {
  await sodium.ready;
  return sodium.crypto_sign_keypair();
}

/** Sets up a user's device the way the app does after login. */
export async function device(
  instance: FakeInstance,
  who: { name: string; deviceId: string; phrase?: string },
  key: Uint8Array,
): Promise<TestDevice> {
  const { name, deviceId, phrase } = who;
  const userId = users[`@${name}@test`] ?? "";
  const api = instance.api(userId, deviceId);
  const database = createMemoryDatabase();
  const setUp = await setUpIdentity(api, database, userId);
  const identity =
    setUp.status === "ready" ? setUp.identity : await restoreWithPhrase(api, database, userId, phrase ?? "");
  const own = await setUpDevice(database, identity, { user: `@${name}@test`, deviceId });
  await refillKeyPackages(api, database, own);
  const context: EngineContext = {
    api,
    database,
    device: own,
    check: credentialCheck(
      api,
      database,
      (user) => users[user],
      () => undefined,
    ),
    instanceKey: key,
    backupKey: identity.backupKey,
    userOfAddress: (user) => users[user],
  };
  return {
    context,
    identity,
    ...(setUp.status === "ready" && setUp.recoveryPhrase !== undefined
      ? { phrase: setUp.recoveryPhrase }
      : {}),
  };
}
