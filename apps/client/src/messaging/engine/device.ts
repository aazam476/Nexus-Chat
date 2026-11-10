/**
 * This device as an MLS client (docs/e2ee-design.md §2–3): one MLS
 * signature key, certified by the user's self-signing key, and a stock of
 * key packages on the server so others can add the device while it is
 * offline. Key packages' private keys stay here, in the encrypted
 * database, until a Welcome uses one.
 */
import {
  certifyDevice,
  createMember,
  createSignatureKeys,
  encodeDeviceCredential,
  loadMember,
  saveMember,
  type Member,
  type SignatureKeys,
} from "@nexus-chat/crypto";

import type { LocalDatabase } from "../../storage/localDatabase";
import type { MessagingApi } from "./api";
import { fromBase64, toBase64, utf8 } from "./bytes";
import type { StoredIdentity } from "./identity";

/** How many one-time key packages a device keeps on the server (§3.2). */
export const keyPackageTarget = 50;

/** This device's MLS credential and signature keys. */
export interface Device {
  /** The server's ID for this device (its session). */
  deviceId: string;
  /** `@user@instance/deviceId`. */
  address: string;
  credential: Uint8Array;
  signatureKeys: SignatureKeys;
}

const keysCollection = "keys";
const membersCollection = "members";

/** Loads this device's keys, making and certifying them the first time. */
export async function setUpDevice(
  database: LocalDatabase,
  identity: StoredIdentity,
  where: { user: string; deviceId: string },
): Promise<Device> {
  const { deviceId } = where;
  const address = `${where.user}/${deviceId}`;
  const stored = await database.get(keysCollection, "device");
  if (stored !== undefined) {
    const fields = JSON.parse(new TextDecoder().decode(stored)) as Record<string, string>;
    if (fields.address === address) {
      return {
        deviceId,
        address,
        credential: fromBase64(fields.credential ?? ""),
        signatureKeys: {
          signKey: fromBase64(fields.signKey ?? ""),
          publicKey: fromBase64(fields.publicKey ?? ""),
        },
      };
    }
  }
  const signatureKeys = await createSignatureKeys();
  const certificate = await certifyDevice(identity.selfSigningKey, address, signatureKeys.publicKey);
  const credential = encodeDeviceCredential({ identity: address, certificate: certificate.signature });
  const record = {
    address,
    credential: toBase64(credential),
    signKey: toBase64(signatureKeys.signKey),
    publicKey: toBase64(signatureKeys.publicKey),
  };
  await database.put(keysCollection, "device", utf8(JSON.stringify(record)));
  return { deviceId, address, credential, signatureKeys };
}

/** Makes a key package and keeps its private keys. */
async function newMember(database: LocalDatabase, device: Device, id: string): Promise<Member> {
  const member = await createMember(device.credential, device.signatureKeys);
  await database.put(membersCollection, id, saveMember(member));
  return member;
}

/** Tops the server's stock up to keyPackageTarget when it asks for more, and keeps a last-resort package there. */
export async function refillKeyPackages(
  api: MessagingApi,
  database: LocalDatabase,
  device: Device,
): Promise<number> {
  const stock = await api.keyPackageStock();
  const missing = stock.refill ? keyPackageTarget - stock.oneTime : 0;
  if (missing <= 0 && stock.hasLastResort) {
    return 0;
  }
  const fresh: Uint8Array[] = [];
  for (let index = 0; index < missing; index++) {
    fresh.push((await newMember(database, device, crypto.randomUUID())).keyPackage);
  }
  const lastResort = stock.hasLastResort
    ? undefined
    : (await newMember(database, device, "last-resort")).keyPackage;
  await api.uploadKeyPackages(fresh, lastResort);
  return fresh.length;
}

/** Every key package this device still holds the private keys of, for trying a Welcome. */
export async function heldMembers(database: LocalDatabase): Promise<{ id: string; member: Member }[]> {
  const ids = await database.list(membersCollection);
  const held: { id: string; member: Member }[] = [];
  for (const id of ids) {
    const saved = await database.get(membersCollection, id);
    if (saved !== undefined) {
      held.push({ id, member: loadMember(saved) });
    }
  }
  return held;
}

/** Forgets a used one-time key package; the last-resort one is kept. */
export async function forgetMember(database: LocalDatabase, id: string): Promise<void> {
  if (id !== "last-resort") {
    await database.delete(membersCollection, id);
  }
}
