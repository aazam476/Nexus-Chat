import type { ChannelPlacement, ServerChannel } from "@nexus-chat/api-client";
import { Button, Heading, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { TFunction } from "i18next";
import { type ReactNode, useState } from "react";
import { useTranslation } from "react-i18next";

import { DraggableRow, useRowLayouts } from "../../../components/DraggableRow";
import { isolate } from "../../../i18n/bidi";
import { moved } from "../../../servers/channelOrder";
import { channelRows, droppedChannel, placedChannels } from "../../../servers/dropOrder";
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

/** The channel order shown: the one just dropped until the server sends the new channels, or it fails. */
function useShownChannels(channels: readonly ServerChannel[], runner: ServerRunner) {
  const [dropped, setDropped] = useState<{
    base: readonly ServerChannel[];
    placements: ChannelPlacement[];
  }>();
  const shown =
    dropped?.base === channels && runner.error === undefined
      ? placedChannels(channels, dropped.placements)
      : channels;
  const place = (placements: ChannelPlacement[]): void => {
    setDropped({ base: channels, placements });
  };
  return { shown, place };
}

/**
 * Every channel the manager can see, under their categories, with its
 * controls. Rows can also be held and dragged: a channel into another place
 * or category, a category with its channels.
 */
export function ChannelAdminList({
  serverId,
  channels,
  runner,
}: Omit<ChannelControlsProps, "channel">): ReactNode {
  const { t } = useTranslation();
  const { shown, place } = useShownChannels(channels, runner);
  const layouts = useRowLayouts();
  const rows = channelRows(shown);
  const drop = (channel: ServerChannel, from: number, translationY: number): void => {
    const placements = droppedChannel(shown, channel.id, layouts.target(from, translationY));
    if (placements !== undefined) {
      place(placements);
      runner.run((client) => placeChannels(client, serverId, placements));
    }
  };
  return (
    <Stack gap="md">
      {rows[0] === undefined || rows[0].kind === "category" ? null : (
        <Heading level={2}>{t("servers.uncategorized")}</Heading>
      )}
      {rows.map((channel, index) => (
        <DraggableRow
          key={channel.id}
          enabled={!runner.busy}
          onLayout={layouts.track(index)}
          onDrop={(translationY) => {
            drop(channel, index, translationY);
          }}
          testID={`channel-drag-${channel.id}`}
        >
          <ChannelControls serverId={serverId} channel={channel} channels={shown} runner={runner} />
        </DraggableRow>
      ))}
    </Stack>
  );
}
