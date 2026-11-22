import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { JoinServer } from "../src/screens/servers/JoinServer";

/** Joins a server with an invite someone sent. */
export default function Join(): ReactNode {
  const { t } = useTranslation();
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="join-back"
        />
        <Heading>{t("servers.joinTitle")}</Heading>
        <Text tone="muted">{t("servers.joinIntro")}</Text>
      </Stack>
      <JoinServer />
    </Screen>
  );
}
