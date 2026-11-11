import { Heading, Link, Screen, Stack } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { useConnection } from "../src/connection/ConnectionContext";
import { useConversations } from "../src/messaging/useConversations";
import { ConversationList } from "../src/screens/conversations/ConversationList";

/** The user's direct and group messages. */
export default function Conversations(): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const { conversations } = useConversations();
  const user = connection.user;
  useEffect(() => {
    if (!connection.loading && user === undefined) {
      router.replace("/");
    }
  }, [connection.loading, user]);
  if (user === undefined) {
    return null;
  }
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("conversations.back")}
          onPress={() => {
            router.back();
          }}
          testID="conversations-back"
        />
        <Heading>{t("conversations.title")}</Heading>
        <Link
          label={t("conversations.friends")}
          onPress={() => {
            router.push("/friends");
          }}
        />
      </Stack>
      <ConversationList conversations={conversations} selfId={user.id} />
    </Screen>
  );
}
