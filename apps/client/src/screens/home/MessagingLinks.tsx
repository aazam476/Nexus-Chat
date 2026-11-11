import { Button, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useMessaging } from "../../messaging/MessagingContext";

function SetupFailed({ retry }: { retry: () => void }): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      <Text tone="danger" live>
        {t("home.messagingFailed")}
      </Text>
      <Button label={t("home.messagingRetry")} variant="secondary" onPress={retry} />
    </Stack>
  );
}

/**
 * What the user must do before messaging: write down a new recovery
 * phrase, or restore on a new device. It leads the screen as its main
 * action rather than taking the user away from it.
 */
function NextStep({ restore }: { restore: boolean }): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      <Text>{restore ? t("home.restoreNeeded") : t("home.phraseNeeded")}</Text>
      <Button
        label={restore ? t("home.restore") : t("home.writePhrase")}
        onPress={() => {
          router.push(restore ? "/restore" : "/recovery");
        }}
        testID="messaging-next-step"
      />
    </Stack>
  );
}

/** The way to conversations and friends, once encryption is set up on this device. */
export function MessagingLinks(): ReactNode {
  const { t } = useTranslation();
  const messaging = useMessaging();
  if (messaging.status === "starting") {
    return <Text tone="muted">{t("home.messagingStarting")}</Text>;
  }
  if (messaging.status === "failed") {
    return <SetupFailed retry={messaging.retry} />;
  }
  if (messaging.status === "recovery-phrase" || messaging.status === "needs-restore") {
    return <NextStep restore={messaging.status === "needs-restore"} />;
  }
  if (messaging.status !== "ready") {
    return null;
  }
  return (
    <Stack gap="sm">
      <Button
        label={t("home.messages")}
        onPress={() => {
          router.push("/conversations");
        }}
        testID="open-messages"
      />
      <Button
        label={t("home.friends")}
        variant="secondary"
        onPress={() => {
          router.push("/friends");
        }}
        testID="open-friends"
      />
    </Stack>
  );
}
