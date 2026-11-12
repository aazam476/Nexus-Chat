import type { Contact, Conversation } from "@nexus-chat/api-client";
import { Avatar, Heading, ListRow, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { isolate } from "../../i18n/bidi";
import { ActionButton } from "../conversation/ActionButton";
import { useFriends } from "../friends/useFriends";
import type { GroupSettings } from "./useGroupSettings";

/** Props of {@link GroupMembers}. */
export interface GroupMembersProps {
  conversation: Conversation;
  settings: GroupSettings;
}

/** Friends who could join, while there is room. */
function AddMembers({
  addable,
  full,
  settings,
}: {
  addable: Contact[];
  full: boolean;
  settings: GroupSettings;
}): ReactNode {
  const { t } = useTranslation();
  if (addable.length === 0) {
    return null;
  }
  return (
    <Stack gap="xs">
      <Heading level={2}>{t("groups.add")}</Heading>
      {full ? <Text tone="muted">{t("groups.full")}</Text> : null}
      {full
        ? null
        : addable.map(({ user }) => (
            <ActionButton
              key={user.id}
              label={t("groups.addMember", { name: isolate(user.displayName) })}
              onPress={() => {
                settings.add(user.id);
              }}
              testID={`add-${user.username}`}
            />
          ))}
    </Stack>
  );
}

/** Who is in the group, and friends who could be added while there is room. */
export function GroupMembers({ conversation, settings }: GroupMembersProps): ReactNode {
  const { t } = useTranslation();
  const { address, instance } = useConnection();
  const { friends } = useFriends();
  const inGroup = new Set(conversation.members.map((member) => member.id));
  const addable = (friends?.friends ?? []).filter(({ user }) => !inGroup.has(user.id));
  const full = conversation.members.length >= (instance?.info.limits.groupDmMaxMembers ?? 25);
  return (
    <Stack gap="xs">
      <Heading level={2}>{t("groups.members")}</Heading>
      {conversation.members.map((member) => (
        <ListRow
          key={member.id}
          title={isolate(member.displayName)}
          subtitle={`@${member.username}`}
          leading={
            <Avatar
              name={member.displayName}
              {...(member.avatarUrl === undefined || address === undefined
                ? {}
                : { imageUrl: `${address}${member.avatarUrl}` })}
            />
          }
        />
      ))}
      <AddMembers addable={addable} full={full} settings={settings} />
    </Stack>
  );
}
