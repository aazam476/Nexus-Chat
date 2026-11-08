import { Screen, Text } from "@nexus-chat/ui";
import { Redirect } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../src/connection/ConnectionContext";
import { startPath } from "../src/navigation/startPath";

/** Sends the user to the right first screen for where they left off. */
export default function Start(): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  if (connection.loading) {
    return (
      <Screen>
        <Text>{t("common.loading")}</Text>
      </Screen>
    );
  }
  return <Redirect href={startPath(connection)} />;
}
