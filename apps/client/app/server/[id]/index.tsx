import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { isolate } from "../../../src/i18n/bidi";
import { ChannelList } from "../../../src/screens/servers/ChannelList";
import { ServerActions } from "../../../src/screens/servers/ServerActions";
import { useServer } from "../../../src/servers/useServers";

/** A server: its description, its channels, and what the user may do there. */
export default function ServerScreen(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { server } = useServer(id);
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="server-back"
        />
        {server === undefined ? <Text tone="muted">{t("servers.loadingServer")}</Text> : null}
        {server === null ? <Text tone="muted">{t("servers.gone")}</Text> : null}
        {server ? <Heading>{isolate(server.details.server.name)}</Heading> : null}
        {server && server.details.server.description !== "" ? (
          <Text tone="muted">{isolate(server.details.server.description)}</Text>
        ) : null}
      </Stack>
      {server ? <ChannelList serverId={id} channels={server.details.channels} /> : null}
      {server ? <ServerActions details={server.details} /> : null}
    </Screen>
  );
}
