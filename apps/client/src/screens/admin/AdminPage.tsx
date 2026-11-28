import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../components/HttpsWarning";
import { useConnection } from "../../connection/ConnectionContext";

/** An admin page: only instance admins see it (the server checks too), with a way back to the panel. */
export function AdminPage({
  title,
  intro,
  children,
}: {
  title: string;
  intro?: string;
  children: ReactNode;
}): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const allowed = connection.user?.isInstanceAdmin === true;
  useEffect(() => {
    if (!connection.loading && !allowed) {
      router.replace("/");
    }
  }, [connection.loading, allowed]);
  if (!allowed) {
    return null;
  }
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("admin.toPanel")}
          onPress={() => {
            router.back();
          }}
          testID="admin-page-back"
        />
        <Heading>{title}</Heading>
        {intro === undefined ? null : <Text tone="muted">{intro}</Text>}
      </Stack>
      {children}
    </Screen>
  );
}
