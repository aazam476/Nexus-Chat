import type { ServerChannel } from "@nexus-chat/api-client";
import { Button, Heading, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { TFunction } from "i18next";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { isolate } from "../../../i18n/bidi";
import { moved } from "../../../servers/channelOrder";
import { channelSections } from "../../../servers/channelTree";
import { placeChannels } from "../../../servers/manageActions";
import type { ServerRunner } from "../useServerRunner";

/** Props of {@link ChannelControls}. */
interface ChannelControlsProps {
  serverId: string;
  channel: ServerChannel;
  channels: readonly ServerChannel[];
  runner: ServerRunner;
}

/** How a channel is named in the controls: #name, or the category's name. */
function shownName(channel: ServerChannel, t: TFunction): string {
  return channel.kind === "text"
    ? t("servers.textChannel", { name: isolate(channel.name) })
    : isolate(channel.name);
}

/** The buttons that move a channel one place, where it can go that way. */
function MoveButtons({
  serverId,
  channel,
  channels,
  runner,
  name,
}: ChannelControlsProps & { name: string }): ReactNode {
  const { t } = useTranslation();
  const buttons = [
    { by: -1 as const, label: "servers.channelAdmin.moveUp", id: "up" },
    { by: 1 as const, label: "servers.channelAdmin.moveDown", id: "down" },
  ];
  return buttons.map(({ by, label, id }) => {
    const order = moved(channels, channel.id, by);
    return order === undefined ? null : (
      <Button
        key={id}
        label={t(label, { name })}
        variant="secondary"
        onPress={() => {
          runner.run((client) => placeChannels(client, serverId, order));
        }}
        disabled={runner.busy}
        testID={`channel-${id}-${channel.id}`}
      />
    );
  });
}

/** Moving one channel up or down, and editing it. Moves that cannot happen are not offered. */
function ChannelControls(props: ChannelControlsProps): ReactNode {
  const { t } = useTranslation();
  const { serverId, channel } = props;
  const name = shownName(channel, t);
  return (
    <Stack gap="xs">
      <Text variant={channel.kind === "category" ? "label" : "body"}>{name}</Text>
      <Stack gap="xs" direction="row">
        <MoveButtons {...props} name={name} />
        <Button
          label={t("servers.channelAdmin.edit", { name })}
          variant="secondary"
          onPress={() => {
            router.push({
              pathname: "/server/[id]/channel/[channelId]/edit",
              params: { id: serverId, channelId: channel.id },
            });
          }}
          testID={`channel-edit-${channel.id}`}
        />
      </Stack>
    </Stack>
  );
}

/** Every channel the manager can see, under their categories, with its controls. */
export function ChannelAdminList({
  serverId,
  channels,
  runner,
}: Omit<ChannelControlsProps, "channel">): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="lg">
      {channelSections(channels).map(({ category, channels: inSection }) => (
        <Stack gap="md" key={category?.id ?? "loose"}>
          {category === undefined ? (
            <Heading level={2}>{t("servers.uncategorized")}</Heading>
          ) : (
            <ChannelControls serverId={serverId} channel={category} channels={channels} runner={runner} />
          )}
          {inSection.map((channel) => (
            <ChannelControls
              key={channel.id}
              serverId={serverId}
              channel={channel}
              channels={channels}
              runner={runner}
            />
          ))}
        </Stack>
      ))}
    </Stack>
  );
}
