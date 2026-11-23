import type { ServerChannel } from "@nexus-chat/api-client";
import { Button, Stack, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { OptionPicker } from "../../../components/OptionPicker";
import { isolate } from "../../../i18n/bidi";
import { movedInto } from "../../../servers/channelOrder";
import { editChannel, placeChannels } from "../../../servers/manageActions";
import type { ServerRunner } from "../useServerRunner";

/** Props of {@link ChannelDetailsForm}. */
export interface ChannelDetailsFormProps {
  serverId: string;
  channel: ServerChannel;
  channels: readonly ServerChannel[];
  runner: ServerRunner;
}

/** The category the channel is in; changing it moves the channel at once. */
function CategoryChoice({ serverId, channel, channels, runner }: ChannelDetailsFormProps): ReactNode {
  const { t } = useTranslation();
  const options = [
    { value: undefined, label: t("servers.channelAdmin.noCategory") },
    ...channels
      .filter((candidate) => candidate.kind === "category")
      .map((category) => ({ value: category.id, label: isolate(category.name) })),
  ];
  return (
    <OptionPicker
      label={t("servers.channelAdmin.category")}
      options={options}
      value={channel.parentId}
      onChange={(parentId) => {
        runner.run((client) => placeChannels(client, serverId, movedInto(channels, channel.id, parentId)));
      }}
      testID="channel-category"
    />
  );
}

/** A channel's name and topic, and for a text or voice channel, its category. */
export function ChannelDetailsForm(props: ChannelDetailsFormProps): ReactNode {
  const { t } = useTranslation();
  const { serverId, channel, runner } = props;
  const [name, setName] = useState(channel.name);
  const [topic, setTopic] = useState(channel.topic);
  const save = (): void => {
    runner.run((client) =>
      editChannel(client, { serverId, channelId: channel.id }, { name: name.trim(), topic }),
    );
  };
  return (
    <Stack gap="md">
      <TextField
        label={t("servers.name")}
        hint={t("servers.nameHint")}
        value={name}
        onChangeText={setName}
        testID="channel-name"
      />
      {channel.kind === "category" ? null : (
        <TextField
          label={t("servers.channelAdmin.topic")}
          hint={t("servers.channelAdmin.topicHint")}
          value={topic}
          onChangeText={setTopic}
          testID="channel-topic"
        />
      )}
      <Button
        label={t("servers.manage.save")}
        onPress={save}
        busy={runner.busy}
        disabled={name.trim() === ""}
        testID="channel-save"
      />
      {channel.kind === "category" ? null : <CategoryChoice {...props} />}
    </Stack>
  );
}
