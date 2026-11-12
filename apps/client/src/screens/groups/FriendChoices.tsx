import type { Contact } from "@nexus-chat/api-client";
import { Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { isolate } from "../../i18n/bidi";
import { ChoiceRow } from "./ChoiceRow";

/** Props of {@link FriendChoices}. */
export interface FriendChoicesProps {
  friends: Contact[];
  chosen: ReadonlySet<string>;
  room: number;
  toggle: (userId: string) => void;
}

/** Friends to pick for a group, with how many are picked out of how many fit. */
export function FriendChoices({ friends, chosen, room, toggle }: FriendChoicesProps): ReactNode {
  const { t } = useTranslation();
  const { address } = useConnection();
  if (friends.length === 0) {
    return <Text tone="muted">{t("groups.noFriends")}</Text>;
  }
  const full = chosen.size >= room;
  return (
    <Stack gap="xs">
      <Text tone="muted" live>
        {t("groups.selected", { count: chosen.size, maximum: room })}
      </Text>
      {friends.map(({ user }) => (
        <ChoiceRow
          key={user.id}
          name={isolate(user.displayName)}
          detail={`@${user.username}`}
          imageUrl={
            user.avatarUrl === undefined || address === undefined ? undefined : `${address}${user.avatarUrl}`
          }
          chosen={chosen.has(user.id)}
          enabled={chosen.has(user.id) || !full}
          onToggle={() => {
            toggle(user.id);
          }}
          testID={`choose-${user.username}`}
        />
      ))}
    </Stack>
  );
}
