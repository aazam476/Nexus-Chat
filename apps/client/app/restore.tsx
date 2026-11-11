import { Button, Heading, Screen, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../src/connection/ConnectionContext";
import { useMessaging } from "../src/messaging/MessagingContext";
import { RestoreForm } from "../src/screens/restore/RestoreForm";

/** A new device of an existing account: restore the keys from the recovery phrase. */
export default function Restore(): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const messaging = useMessaging();
  const needed = messaging.status === "needs-restore";
  useEffect(() => {
    if (!needed) {
      router.replace("/home");
    }
  }, [needed]);
  if (!needed) {
    return null;
  }
  return (
    <Screen>
      <Stack gap="md">
        <Heading>{t("restore.title")}</Heading>
        <Text tone="muted">{t("restore.intro")}</Text>
        <RestoreForm restore={messaging.restore} />
        <Button label={t("restore.logout")} variant="secondary" onPress={() => void connection.signOut()} />
      </Stack>
    </Screen>
  );
}
