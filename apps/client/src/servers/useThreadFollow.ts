/**
 * Following a thread (spec §4.5): followed threads notify the user of new
 * replies (M8). The server knows who follows; repliers follow
 * automatically, and the root's sender when the thread starts.
 */
import { followThread, listChannelThreads, unfollowThread } from "@nexus-chat/api-client";
import { useCallback, useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { messengerOf, useMessaging } from "../messaging/MessagingContext";

/** Whether the user follows the thread (undefined until known, or while it has no replies), and changing that. */
export interface ThreadFollow {
  following: boolean | undefined;
  toggle: () => void;
  busy: boolean;
}

/** Counts the reasons to load the follow again: the channel's messages arriving, or a change here. */
function useReloads(channelId: string): [number, () => void] {
  const messenger = messengerOf(useMessaging());
  const [version, setVersion] = useState(0);
  const reload = useCallback(() => {
    setVersion((count) => count + 1);
  }, []);
  useEffect(
    () =>
      messenger?.subscribe((changed) => {
        if (changed === channelId) {
          reload();
        }
      }),
    [messenger, channelId, reload],
  );
  return [version, reload];
}

/** The user's follow of the thread on rootId, reloaded as the channel's messages arrive. */
export function useThreadFollow(serverId: string, channelId: string, rootId: string): ThreadFollow {
  const { client } = useConnection();
  const [following, setFollowing] = useState<boolean>();
  const [busy, setBusy] = useState(false);
  const [version, reload] = useReloads(channelId);
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    let current = true;
    void listChannelThreads({ client, path: { serverId, channelId } }).then((result) => {
      if (current) {
        setFollowing(result.data?.find((thread) => thread.rootId === rootId)?.following);
      }
    });
    return () => {
      current = false;
    };
  }, [client, serverId, channelId, rootId, version]);
  const toggle = (): void => {
    if (client === undefined || following === undefined) {
      return;
    }
    setBusy(true);
    const path = { serverId, channelId, rootId };
    void (following ? unfollowThread({ client, path }) : followThread({ client, path })).then(() => {
      setBusy(false);
      reload();
    });
  };
  return { following, toggle, busy };
}
