import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { ChannelAdminList } from "../../../src/screens/servers/admin/ChannelAdminList";
import { NewChannelForm } from "../../../src/screens/servers/admin/NewChannelForm";
import { useServerRunner } from "../../../src/screens/servers/useServerRunner";
import { useServer } from "../../../src/servers/useServers";

/** Managing channels: their order and categories, and adding new ones (MANAGE_CHANNELS). */
export default function ChannelAdmin(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { server } = useServer(id);
  const runner = useServerRunner();
  const channels = server?.details.channels ?? [];
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="channel-admin-back"
        />
        <Heading>{t("servers.channelAdmin.title")}</Heading>
        <Text tone="muted">{t("servers.channelAdmin.intro")}</Text>
      </Stack>
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
      <ChannelAdminList serverId={id} channels={channels} runner={runner} />
      {server ? <NewChannelForm serverId={id} channels={channels} runner={runner} /> : null}
    </Screen>
  );
}
