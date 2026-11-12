import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../src/components/HttpsWarning";
import { useConnection } from "../../src/connection/ConnectionContext";
import { conversationName } from "../../src/messaging/conversationName";
import { useConversation } from "../../src/messaging/useConversation";
import { GroupIdentity, type GroupIdentityProps } from "../../src/screens/groups/GroupIdentity";
import { GroupMembers } from "../../src/screens/groups/GroupMembers";
import { LeaveGroup } from "../../src/screens/groups/LeaveGroup";
import { useGroupSettings } from "../../src/screens/groups/useGroupSettings";

function GroupSections({ conversation, title, settings }: GroupIdentityProps): ReactNode {
  return (
    <>
      <GroupIdentity
        key={conversation.name ?? ""}
        conversation={conversation}
        title={title}
        settings={settings}
      />
      <GroupMembers conversation={conversation} settings={settings} />
      <LeaveGroup leave={settings.leave} busy={settings.busy} />
    </>
  );
}

/** A group DM's name, icon and members, and leaving it. */
export default function GroupSettingsScreen(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useConnection();
  const { conversation } = useConversation(id);
  const settings = useGroupSettings(id);
  const title =
    conversation === undefined || user === undefined ? "" : conversationName(conversation, user.id, t);
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("groups.back")}
          onPress={() => {
            router.back();
          }}
          testID="group-back"
        />
        <Heading>{t("groups.settingsTitle")}</Heading>
      </Stack>
      {settings.error === undefined ? null : (
        <Text tone="danger" live>
          {settings.error}
        </Text>
      )}
      {conversation === undefined ? null : (
        <GroupSections conversation={conversation} title={title} settings={settings} />
      )}
    </Screen>
  );
}
