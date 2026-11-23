import type { Server } from "@nexus-chat/api-client";
import { Avatar } from "@nexus-chat/ui";
import type { ReactNode } from "react";

import { useConnection } from "../../connection/ConnectionContext";

/** A server's icon, or its initials. */
export function ServerIcon({ server, size }: { server: Server; size?: number }): ReactNode {
  const { address } = useConnection();
  const image =
    server.iconUrl === undefined || address === undefined ? {} : { imageUrl: `${address}${server.iconUrl}` };
  return <Avatar name={server.name} {...(size === undefined ? {} : { size })} {...image} />;
}
