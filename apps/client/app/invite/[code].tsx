import { Heading, Screen, Stack } from "@nexus-chat/ui";
import { useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../src/components/HttpsWarning";
import { JoinServer } from "../../src/screens/servers/JoinServer";

/** An invite link, opened in the app: what it leads to, and joining. */
export default function Invite(): ReactNode {
  const { t } = useTranslation();
  const { code } = useLocalSearchParams<{ code: string }>();
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Heading>{t("servers.joinTitle")}</Heading>
      </Stack>
      <JoinServer initialCode={code} />
    </Screen>
  );
}
