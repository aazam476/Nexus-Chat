import { Button, Heading, Screen, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link SetupDone}. */
export interface SetupDoneProps {
  host: string;
}

/** Shown once setup has finished. */
export function SetupDone({ host }: SetupDoneProps): ReactNode {
  const { t } = useTranslation();
  return (
    <Screen>
      <Heading>{t("setup.title", { instance: host })}</Heading>
      <Text>{t("setup.done")}</Text>
      <Button
        label={t("login.submit")}
        onPress={() => {
          router.replace("/login");
        }}
        testID="setup-go-login"
      />
    </Screen>
  );
}
