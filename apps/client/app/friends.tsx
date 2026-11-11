import { Heading, Link, Screen, Stack } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { useConnection } from "../src/connection/ConnectionContext";
import { AddFriend } from "../src/screens/friends/AddFriend";
import { FriendSections } from "../src/screens/friends/FriendSections";
import { useFriends } from "../src/screens/friends/useFriends";

/** Friends, friend requests both ways, and starting a DM. */
export default function FriendsScreen(): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const state = useFriends();
  useEffect(() => {
    if (!connection.loading && connection.user === undefined) {
      router.replace("/");
    }
  }, [connection.loading, connection.user]);
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("friends.back")}
          onPress={() => {
            router.back();
          }}
          testID="friends-back"
        />
        <Heading>{t("friends.title")}</Heading>
      </Stack>
      <AddFriend request={state.request} />
      <FriendSections state={state} />
    </Screen>
  );
}
