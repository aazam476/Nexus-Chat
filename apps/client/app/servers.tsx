import { Button, Heading, Link, Screen, Stack } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { ServerList } from "../src/screens/servers/ServerList";
import { useServers } from "../src/servers/useServers";

/** The servers the user is in, and the ways to make or join another. */
export default function Servers(): ReactNode {
  const { t } = useTranslation();
  const { servers } = useServers();
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="servers-back"
        />
        <Heading>{t("servers.title")}</Heading>
      </Stack>
      <ServerList servers={servers} />
      <Stack gap="sm">
        <Button
          label={t("servers.create")}
          onPress={() => {
            router.push("/new-server");
          }}
          testID="server-new"
        />
        <Button
          label={t("servers.join")}
          variant="secondary"
          onPress={() => {
            router.push("/join");
          }}
          testID="server-join"
        />
      </Stack>
    </Screen>
  );
}
