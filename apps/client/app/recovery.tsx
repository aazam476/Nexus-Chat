import { Button, Heading, Screen, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useMessaging } from "../src/messaging/MessagingContext";
import { PhraseCheck } from "../src/screens/recovery/PhraseCheck";
import { PhraseWords } from "../src/screens/recovery/PhraseWords";

/**
 * The new recovery phrase, shown until the user proves they wrote it down
 * by typing a few of its words back (docs/e2ee-design.md §5).
 */
export default function Recovery(): ReactNode {
  const { t } = useTranslation();
  const messaging = useMessaging();
  const [checking, setChecking] = useState(false);
  const showing = messaging.status === "recovery-phrase";
  useEffect(() => {
    if (!showing) {
      router.replace("/home");
    }
  }, [showing]);
  if (messaging.status !== "recovery-phrase") {
    return null;
  }
  return (
    <Screen>
      {checking ? (
        <PhraseCheck
          phrase={messaging.phrase}
          onConfirmed={messaging.confirmPhrase}
          onShowAgain={() => {
            setChecking(false);
          }}
        />
      ) : (
        <Stack gap="md">
          <Heading>{t("recovery.title")}</Heading>
          <Text tone="muted">{t("recovery.intro")}</Text>
          <PhraseWords phrase={messaging.phrase} />
          <Button
            label={t("recovery.wroteItDown")}
            onPress={() => {
              setChecking(true);
            }}
            testID="recovery-wrote"
          />
        </Stack>
      )}
    </Screen>
  );
}
