/**
 * Keeping MLS state between app launches, and joining a group from its
 * GroupInfo. A device stores each group's state and its own key packages
 * in its encrypted database (ADR 0013); these turn them into bytes and
 * back. The bytes hold private keys, so they are only ever stored
 * encrypted.
 */
import {
  decodeGroupState,
  decodeMlsMessage,
  encodeGroupState,
  encodeMlsMessage,
  joinGroupExternal,
} from "ts-mls";

import { lengthPrefixed, readFields } from "../encoding.js";
import { ciphersuite } from "./ciphersuite.js";
import { clientConfig, MlsMessageError, type CredentialCheck, type Group, type Member } from "./engine.js";

/** A group's state as bytes, to store encrypted. */
export function saveGroup(group: Group): Uint8Array {
  return encodeGroupState(group.state);
}

/** A group saved with `saveGroup`; `check` vets members from now on, as when it was made. */
export function loadGroup(bytes: Uint8Array, check?: CredentialCheck): Group {
  const decoded = decodeGroupState(bytes, 0);
  if (decoded === undefined) {
    throw new MlsMessageError("not a saved group");
  }
  return { state: { ...decoded[0], clientConfig: clientConfig(check) } };
}

/** A member (key package and its private keys) as bytes, to store encrypted. */
export function saveMember(member: Member): Uint8Array {
  return lengthPrefixed(
    member.keyPackage,
    member.secrets.initPrivateKey,
    member.secrets.hpkePrivateKey,
    member.secrets.signaturePrivateKey,
  );
}

/** A member saved with `saveMember`. */
export function loadMember(bytes: Uint8Array): Member {
  const fields = readFields(bytes, 4);
  if (fields === undefined) {
    throw new MlsMessageError("not a saved member");
  }
  const [keyPackage, initPrivateKey, hpkePrivateKey, signaturePrivateKey] = fields as [
    Uint8Array,
    Uint8Array,
    Uint8Array,
    Uint8Array,
  ];
  const message = decodeMlsMessage(keyPackage, 0)?.[0];
  if (message?.wireformat !== "mls_key_package") {
    throw new MlsMessageError("not a saved member");
  }
  return {
    keyPackage,
    publicPackage: message.keyPackage,
    secrets: { initPrivateKey, hpkePrivateKey, signaturePrivateKey },
  };
}

/**
 * Joins a group by external commit (RFC 9420 §12.4.3.2) from the GroupInfo
 * the delivery service holds: how a user's new device enters a
 * conversation without waiting for another member. Send `commit` to the
 * delivery service; the group is usable once it is accepted.
 */
export async function joinExternal(
  groupInfo: Uint8Array,
  member: Member,
  check?: CredentialCheck,
): Promise<{ group: Group; commit: Uint8Array }> {
  const message = decodeMlsMessage(groupInfo, 0)?.[0];
  if (message?.wireformat !== "mls_group_info") {
    throw new MlsMessageError("expected a group info");
  }
  const result = await joinGroupExternal(
    message.groupInfo,
    member.publicPackage,
    member.secrets,
    false,
    await ciphersuite(),
    undefined,
    clientConfig(check),
  );
  const commit = encodeMlsMessage({
    version: "mls10",
    wireformat: "mls_public_message",
    publicMessage: result.publicMessage,
  });
  return { group: { state: result.newState }, commit };
}
