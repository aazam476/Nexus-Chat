import { Button, Heading, Link, Screen, Stack, Text, TextField } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { useNewServer, type NewServer as NewServerState } from "../src/screens/servers/useNewServer";

/** Why the last attempt failed, and the button that creates the server. */
function CreateServer({ server }: { server: NewServerState }): ReactNode {
  const { t } = useTranslation();
  return (
    <>
      {server.error === undefined ? null : (
        <Text tone="danger" live>
          {server.error}
        </Text>
      )}
      <Button
        label={t("servers.createButton")}
        onPress={server.create}
        busy={server.busy}
        disabled={server.name.trim() === ""}
        testID="server-create"
      />
    </>
  );
}

/** Makes a server: a name and, if the user likes, a description. */
export default function NewServer(): ReactNode {
  const { t } = useTranslation();
  const server = useNewServer();
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="new-server-back"
        />
        <Heading>{t("servers.newTitle")}</Heading>
        <Text tone="muted">{t("servers.newIntro")}</Text>
      </Stack>
      <TextField
        label={t("servers.name")}
        hint={t("servers.nameHint")}
        value={server.name}
        onChangeText={server.setName}
        testID="server-name"
      />
      <TextField
        label={t("servers.description")}
        hint={t("servers.descriptionHint")}
        value={server.description}
        onChangeText={server.setDescription}
        lines={3}
        testID="server-description"
      />
      <CreateServer server={server} />
    </Screen>
  );
}
