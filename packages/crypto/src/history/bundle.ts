/**
 * A history bundle (docs/e2ee-design.md §6, ADR 0003): the history keys of
 * a channel, which a current member sends as an MLS application message
 * when someone joins a channel whose audience may read history from before
 * they joined. The MLS message already encrypts and authenticates it; this
 * is only its layout: the label, the channel, then one field of 56-byte
 * records, each the 16-byte MLS group ID, an 8-byte epoch and the 32-byte
 * key. A channel moves between groups as its audience changes, and each
 * group counts its own epochs, so a key is named by both.
 */
import { lengthPrefixed, readFields, uint64, utf8 } from "../encoding.js";

const label = "nexus-chat history bundle v1";
const groupIdBytes = 16;
const keyBytes = 32;
const recordBytes = groupIdBytes + 8 + keyBytes;

/** One epoch's history key, in one MLS group. */
export interface HistoryKeyEntry {
  groupId: Uint8Array;
  epoch: bigint;
  key: Uint8Array;
}

/** The bytes to send. */
export function encodeHistoryBundle(channelId: Uint8Array, entries: HistoryKeyEntry[]): Uint8Array {
  const records = new Uint8Array(entries.length * recordBytes);
  entries.forEach((entry, index) => {
    if (entry.key.length !== keyBytes || entry.groupId.length !== groupIdBytes) {
      throw new RangeError("history keys are 32 bytes and group IDs 16");
    }
    const at = index * recordBytes;
    records.set(entry.groupId, at);
    records.set(uint64(entry.epoch), at + groupIdBytes);
    records.set(entry.key, at + groupIdBytes + 8);
  });
  return lengthPrefixed(utf8(label), channelId, records);
}

/** Reads a bundle made by `encodeHistoryBundle`, or undefined if the bytes are not one. */
export function decodeHistoryBundle(
  bytes: Uint8Array,
): { channelId: Uint8Array; entries: HistoryKeyEntry[] } | undefined {
  const fields = readFields(bytes, 3);
  if (fields === undefined) {
    return undefined;
  }
  const [name, channelId, records] = fields as [Uint8Array, Uint8Array, Uint8Array];
  if (new TextDecoder().decode(name) !== label || records.length % recordBytes !== 0) {
    return undefined;
  }
  const view = new DataView(records.buffer, records.byteOffset, records.byteLength);
  const entries: HistoryKeyEntry[] = [];
  for (let offset = 0; offset < records.length; offset += recordBytes) {
    entries.push({
      groupId: records.slice(offset, offset + groupIdBytes),
      epoch: view.getBigUint64(offset + groupIdBytes),
      key: records.slice(offset + groupIdBytes + 8, offset + recordBytes),
    });
  }
  return { channelId, entries };
}
