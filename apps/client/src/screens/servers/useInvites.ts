/** The server's invites for the Invite people screen, kept as they are made and revoked. */
import type { ServerInvite } from "@nexus-chat/api-client";
import { useCallback, useEffect, useState } from "react";

import { useConnection } from "../../connection/ConnectionContext";
import { invitesOf, revokeInvite } from "../../servers/inviteActions";

/** The invites, or undefined for someone who may not list them; adding and revoking. */
export interface Invites {
  invites: ServerInvite[] | undefined;
  add: (invite: ServerInvite) => void;
  revoke: (code: string) => void;
}

/** Loads the invites when `mayList`, the user's MANAGE_SERVER, allows it. */
export function useInvites(serverId: string, mayList: boolean): Invites {
  const { client } = useConnection();
  const [invites, setInvites] = useState<ServerInvite[]>();
  useEffect(() => {
    if (client === undefined || !mayList) {
      return;
    }
    let current = true;
    void invitesOf(client, serverId).then((listed) => {
      if (current) {
        setInvites(listed);
      }
    });
    return () => {
      current = false;
    };
  }, [client, serverId, mayList]);
  const add = useCallback((invite: ServerInvite) => {
    setInvites((current) => (current === undefined ? current : [...current, invite]));
  }, []);
  const revoke = useCallback(
    (code: string) => {
      if (client !== undefined) {
        void revokeInvite(client, serverId, code).then((result) => {
          if (result.ok) {
            setInvites((current) => current?.filter((invite) => invite.code !== code));
          }
        });
      }
    },
    [client, serverId],
  );
  return { invites, add, revoke };
}
