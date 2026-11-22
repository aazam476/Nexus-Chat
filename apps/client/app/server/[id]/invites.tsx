import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { InviteList } from "../../../src/screens/servers/InviteList";
import { NewInvite } from "../../../src/screens/servers/NewInvite";
import { useInvites } from "../../../src/screens/servers/useInvites";
import { useServer } from "../../../src/servers/useServers";

/** Making invite links, and for those who manage the server, the ones that exist. */
export default function Invites(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { server } = useServer(id);
  const mayList = server?.details.permissions.includes("MANAGE_SERVER") ?? false;
  const invites = useInvites(id, mayList);
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="invites-back"
        />
        <Heading>{t("servers.invites.title")}</Heading>
        <Text tone="muted">{t("servers.invites.intro")}</Text>
      </Stack>
      {server ? <NewInvite serverId={id} serverName={server.details.server.name} made={invites.add} /> : null}
      {invites.invites === undefined ? null : (
        <InviteList invites={invites.invites} revoke={invites.revoke} />
      )}
    </Screen>
  );
}
