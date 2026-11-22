import type { ServerChannel } from "@nexus-chat/api-client";
import { Heading, ListRow, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { isolate } from "../../i18n/bidi";
import { channelSections } from "../../servers/channelTree";

/** One channel: a text channel opens; a voice channel waits for calls. */
function ChannelRow({ serverId, channel }: { serverId: string; channel: ServerChannel }): ReactNode {
  const { t } = useTranslation();
  if (channel.kind === "voice") {
    return (
      <ListRow
        title={t("servers.voiceChannel", { name: isolate(channel.name) })}
        subtitle={t("servers.voiceSoon")}
        testID={`channel-${channel.id}`}
      />
    );
  }
  return (
    <ListRow
      title={t("servers.textChannel", { name: isolate(channel.name) })}
      {...(channel.topic === "" ? {} : { subtitle: isolate(channel.topic) })}
      onPress={() => {
        router.push({
          pathname: "/server/[id]/channel/[channelId]",
          params: { id: serverId, channelId: channel.id },
        });
      }}
      testID={`channel-${channel.id}`}
    />
  );
}

/** The channels the user can see, under their categories. */
export function ChannelList({
  serverId,
  channels,
}: {
  serverId: string;
  channels: readonly ServerChannel[];
}): ReactNode {
  const { t } = useTranslation();
  const sections = channelSections(channels);
  if (sections.every((section) => section.channels.length === 0)) {
    return <Text tone="muted">{t("servers.noChannels")}</Text>;
  }
  return (
    <Stack gap="md">
      {sections.map(({ category, channels: inSection }) =>
        inSection.length === 0 ? null : (
          <Stack gap="xs" key={category?.id ?? "loose"}>
            <Heading level={2}>
              {category === undefined ? t("servers.uncategorized") : isolate(category.name)}
            </Heading>
            {inSection.map((channel) => (
              <ChannelRow key={channel.id} serverId={serverId} channel={channel} />
            ))}
          </Stack>
        ),
      )}
    </Stack>
  );
}
