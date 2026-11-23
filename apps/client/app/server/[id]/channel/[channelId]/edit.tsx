import type { ServerChannel } from "@nexus-chat/api-client";
import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { ConfirmAction } from "../../../../../src/components/ConfirmAction";
import { HttpsWarning } from "../../../../../src/components/HttpsWarning";
import { isolate } from "../../../../../src/i18n/bidi";
import { ChannelDetailsForm } from "../../../../../src/screens/servers/admin/ChannelDetailsForm";
import { ChannelOverrides } from "../../../../../src/screens/servers/admin/ChannelOverrides";
import { useServerRunner, type ServerRunner } from "../../../../../src/screens/servers/useServerRunner";
import { removeChannel } from "../../../../../src/servers/manageActions";
import { useServer, type LoadedServer } from "../../../../../src/servers/useServers";

/** Props of {@link ChannelEditor}. */
interface ChannelEditorProps {
  server: LoadedServer;
  channel: ServerChannel;
  runner: ServerRunner;
}

/** The forms for a channel that exists: details, permissions and deleting it. */
function ChannelEditor({ server, channel, runner }: ChannelEditorProps): ReactNode {
  const { t } = useTranslation();
  const serverId = server.details.server.id;
  return (
    <>
      <ChannelDetailsForm
        key={channel.id}
        serverId={serverId}
        channel={channel}
        channels={server.details.channels}
        runner={runner}
      />
      {channel.kind === "category" ? null : (
        <ChannelOverrides details={server.details} members={server.members} channel={channel} />
      )}
      <ConfirmAction
        label={t("servers.channelAdmin.delete")}
        question={t("servers.channelAdmin.deleteConfirm", { name: isolate(channel.name) })}
        confirmLabel={t("servers.channelAdmin.delete")}
        cancelLabel={t("servers.manage.keep")}
        busy={runner.busy}
        onConfirm={() => {
          runner.run(
            (client) => removeChannel(client, serverId, channel.id),
            () => {
              router.back();
            },
          );
        }}
        testID="channel-delete"
      />
    </>
  );
}

/** Editing one channel: its name, topic and category, its permissions, and deleting it. */
export default function EditChannel(): ReactNode {
  const { t } = useTranslation();
  const { id, channelId } = useLocalSearchParams<{ id: string; channelId: string }>();
  const { server } = useServer(id);
  const runner = useServerRunner();
  const channel = server?.details.channels.find((candidate) => candidate.id === channelId);
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="edit-channel-back"
        />
        <Heading>{t("servers.channelAdmin.editTitle")}</Heading>
        {server !== undefined && channel === undefined ? (
          <Text tone="muted">{t("servers.channelGone")}</Text>
        ) : null}
      </Stack>
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
      {server && channel ? <ChannelEditor server={server} channel={channel} runner={runner} /> : null}
    </Screen>
  );
}
