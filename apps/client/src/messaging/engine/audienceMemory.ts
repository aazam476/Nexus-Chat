/**
 * Which groups on this device are server audiences, kept across restarts,
 * so a sync before the server screens load still files their messages as
 * channel messages.
 */
import type { LocalDatabase } from "../../storage/localDatabase";
import { utf8 } from "./bytes";
import type { Audience } from "./channels";

const collection = "audiences";

/** Remembers the audience's server and channels. */
export async function rememberAudience(database: LocalDatabase, audience: Audience): Promise<void> {
  const remembered = utf8(JSON.stringify({ serverId: audience.serverId, channelIds: audience.channelIds }));
  await database.put(collection, audience.audienceId, remembered);
}

/**
 * An audience this device joined before the app last started. Until the
 * server screens load it again, nobody new is sent its history: who may
 * read it is not known yet, and another member's device will.
 */
export async function storedAudience(
  database: LocalDatabase,
  audienceId: string,
): Promise<Audience | undefined> {
  const remembered = await database.get(collection, audienceId);
  if (remembered === undefined) {
    return undefined;
  }
  const { serverId, channelIds } = JSON.parse(new TextDecoder().decode(remembered)) as {
    serverId: string;
    channelIds: string[];
  };
  return { serverId, audienceId, channelIds, mayReadHistory: () => false };
}
