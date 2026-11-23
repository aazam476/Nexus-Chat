import type { ServerChannel, ServerChannelKind } from "@nexus-chat/api-client";
import { Button, Heading, Stack, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { OptionPicker } from "../../../components/OptionPicker";
import { isolate } from "../../../i18n/bidi";
import { addChannel } from "../../../servers/manageActions";
import type { ServerRunner } from "../useServerRunner";

const kinds: { value: ServerChannelKind; label: string }[] = [
  { value: "text", label: "servers.channelAdmin.kindText" },
  { value: "voice", label: "servers.channelAdmin.kindVoice" },
  { value: "category", label: "servers.channelAdmin.kindCategory" },
];

/** Props of {@link NewChannelForm}. */
export interface NewChannelFormProps {
  serverId: string;
  channels: readonly ServerChannel[];
  runner: ServerRunner;
}

/** The category a new channel goes in; categories themselves never sit in one. */
function CategoryPicker(props: {
  categories: ServerChannel[];
  value: string | undefined;
  onChange: (id: string | undefined) => void;
}): ReactNode {
  const { t } = useTranslation();
  const options = [
    { value: undefined, label: t("servers.channelAdmin.noCategory") },
    ...props.categories.map((category) => ({ value: category.id, label: isolate(category.name) })),
  ];
  return (
    <OptionPicker
      label={t("servers.channelAdmin.category")}
      options={options}
      value={props.value}
      onChange={props.onChange}
      testID="new-channel-category"
    />
  );
}

/** The new channel's fields, and creating it. */
function useNewChannel({ serverId, channels, runner }: NewChannelFormProps) {
  const categories = channels.filter((channel) => channel.kind === "category");
  const [kind, setKind] = useState<ServerChannelKind>("text");
  const [name, setName] = useState("");
  const [parentId, setParentId] = useState<string | undefined>(categories[0]?.id);
  const create = (): void => {
    const inCategory = kind === "category" || parentId === undefined ? {} : { parentId };
    runner.run(
      (client) => addChannel(client, serverId, { kind, name: name.trim(), ...inCategory }),
      () => {
        setName("");
      },
    );
  };
  return { categories, kind, setKind, name, setName, parentId, setParentId, create };
}

/** Adding a text or voice channel, or a category. */
export function NewChannelForm(props: NewChannelFormProps): ReactNode {
  const { t } = useTranslation();
  const form = useNewChannel(props);
  return (
    <Stack gap="md">
      <Heading level={2}>{t("servers.channelAdmin.newTitle")}</Heading>
      <OptionPicker
        label={t("servers.channelAdmin.kind")}
        options={kinds.map((option) => ({ ...option, label: t(option.label) }))}
        value={form.kind}
        onChange={form.setKind}
        testID="new-channel-kind"
      />
      <TextField
        label={t("servers.name")}
        hint={t("servers.nameHint")}
        value={form.name}
        onChangeText={form.setName}
        testID="new-channel-name"
      />
      {form.kind === "category" ? null : (
        <CategoryPicker categories={form.categories} value={form.parentId} onChange={form.setParentId} />
      )}
      <Button
        label={t("servers.channelAdmin.create")}
        onPress={form.create}
        busy={props.runner.busy}
        disabled={form.name.trim() === ""}
        testID="new-channel-create"
      />
    </Stack>
  );
}
