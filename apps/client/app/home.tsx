import { Button, Heading, Screen, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { useConnection } from "../src/connection/ConnectionContext";
import { hostOf } from "../src/instance/address";
import { useGatewayStatus } from "../src/realtime/GatewayContext";
import { AdminLink } from "../src/screens/home/AdminLink";
import { CustomStatusEditor } from "../src/screens/home/CustomStatusEditor";
import { StatusPicker } from "../src/screens/home/StatusPicker";
import { isolate } from "../src/i18n/bidi";

/** The signed-in start screen: who you are, your status, and logging out. */
export default function Home(): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const presenceStatus = useGatewayStatus();
  useEffect(() => {
    if (!connection.loading && connection.user === undefined) {
      router.replace("/");
    }
  }, [connection.loading, connection.user]);
  const user = connection.user;
  if (user === undefined) {
    return null;
  }
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Heading>{t("home.title", { name: isolate(user.displayName) })}</Heading>
        <Text tone="muted">
          {t("home.intro", { instance: hostOf(connection.address ?? "http://localhost") })}
        </Text>
      </Stack>
      <StatusPicker current={user.status} presence={presenceStatus} />
      <CustomStatusEditor />
      <AdminLink admin={user.isInstanceAdmin} />
      <Button
        label={t("home.logout")}
        variant="secondary"
        onPress={() => void connection.signOut()}
        testID="logout"
      />
    </Screen>
  );
}
