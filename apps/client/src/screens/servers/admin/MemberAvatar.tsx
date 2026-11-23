import type { UserSummary } from "@nexus-chat/api-client";
import { Avatar } from "@nexus-chat/ui";
import type { ReactNode } from "react";

import { useConnection } from "../../../connection/ConnectionContext";

/** A member's profile picture, or their initials. */
export function MemberAvatar({ user, size }: { user: UserSummary; size?: number }): ReactNode {
  const { address } = useConnection();
  const image =
    user.avatarUrl === undefined || address === undefined ? {} : { imageUrl: `${address}${user.avatarUrl}` };
  return <Avatar name={user.displayName} {...(size === undefined ? {} : { size })} {...image} />;
}
