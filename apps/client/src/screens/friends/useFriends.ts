/**
 * The friends list and what can be done with it. It reloads after every
 * change and when the gateway says a relationship changed.
 */
import {
  acceptFriendRequest,
  deleteFriendRequest,
  listFriends,
  openDirectMessage,
  removeFriend,
  sendFriendRequest,
  type Friends,
} from "@nexus-chat/api-client";
import { useCallback, useEffect, useState } from "react";

import { useConnection } from "../../connection/ConnectionContext";
import { useGatewayEvents } from "../../realtime/GatewayContext";

/** The lists and the actions on them; each action resolves whether the server agreed. */
export interface FriendsState {
  friends: Friends | undefined;
  request: (username: string) => Promise<boolean>;
  accept: (userId: string) => Promise<boolean>;
  /** Declines a request for you, or cancels one you sent. */
  withdraw: (userId: string) => Promise<boolean>;
  remove: (userId: string) => Promise<boolean>;
  /** The DM with the user, opened or created; undefined when not allowed. */
  message: (userId: string) => Promise<string | undefined>;
}

/** Loads the friends list and keeps it current. */
export function useFriends(): FriendsState {
  const { client } = useConnection();
  const [friends, setFriends] = useState<Friends>();
  const [version, setVersion] = useState(0);
  const reload = useCallback(() => {
    setVersion((count) => count + 1);
  }, []);
  useGatewayEvents((event) => {
    if (event.type === "relationship_update") {
      reload();
    }
  });
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    let current = true;
    void listFriends({ client }).then((result) => {
      if (current && result.data !== undefined) {
        setFriends(result.data);
      }
    });
    return () => {
      current = false;
    };
  }, [client, version]);
  return { friends, ...useFriendActions(reload) };
}

function useFriendActions(reload: () => void): Omit<FriendsState, "friends"> {
  const { client } = useConnection();
  const done = async (call: Promise<{ response?: Response }> | undefined): Promise<boolean> => {
    const result = await call;
    reload();
    return result?.response?.ok === true;
  };
  return {
    request: (username) => done(client && sendFriendRequest({ client, body: { username } })),
    accept: (userId) => done(client && acceptFriendRequest({ client, path: { userId } })),
    withdraw: (userId) => done(client && deleteFriendRequest({ client, path: { userId } })),
    remove: (userId) => done(client && removeFriend({ client, path: { userId } })),
    message: async (userId) => {
      const result = client && (await openDirectMessage({ client, body: { userId } }));
      return result?.data?.id;
    },
  };
}
