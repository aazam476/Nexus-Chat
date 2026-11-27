import type { ServerEmoji } from "@nexus-chat/api-client";
import { Button, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { ConfirmAction } from "../../../components/ConfirmAction";
import { pickOriginalImage } from "../../../platform/pickImage";
import { addEmoji, removeEmoji } from "../../../servers/manageActions";
import { CustomEmoji, emojiSizes } from "../../conversation/CustomEmoji";
import type { ServerRunner } from "../useServerRunner";

/** Props of the emoji settings. */
export interface EmojiAdminProps {
  serverId: string;
  emoji: readonly ServerEmoji[];
  runner: ServerRunner;
  /** Loads the list again after a change. */
  reload: () => void;
}

/** One emoji: its picture and name, and removing it after confirming. */
function EmojiRow({ emoji, props }: { emoji: ServerEmoji; props: EmojiAdminProps }): ReactNode {
  const { t } = useTranslation();
  const { serverId, runner, reload } = props;
  return (
    <Stack gap="xs">
      <Stack gap="sm" direction="row">
        <CustomEmoji name={emoji.name} id={emoji.id} size={emojiSizes.alone} />
        <Text>{`:${emoji.name}:`}</Text>
      </Stack>
      <ConfirmAction
        label={t("servers.emojiAdmin.remove")}
        accessibilityLabel={t("servers.emojiAdmin.removeNamed", { name: emoji.name })}
        question={t("servers.emojiAdmin.removeConfirm", { name: emoji.name })}
        confirmLabel={t("servers.emojiAdmin.removeYes")}
        cancelLabel={t("servers.emojiAdmin.keep")}
        busy={runner.busy}
        onConfirm={() => {
          runner.run((client) => removeEmoji(client, serverId, emoji.id), reload);
        }}
        testID={`emoji-remove-${emoji.name}`}
      />
    </Stack>
  );
}

/** Naming a new emoji, then choosing its picture, which uploads it. */
function NewEmoji(props: EmojiAdminProps): ReactNode {
  const { t } = useTranslation();
  const { serverId, runner, reload } = props;
  const [name, setName] = useState("");
  const add = (): void => {
    runner.run(
      async (client) => {
        const image = await pickOriginalImage();
        return image === undefined ? undefined : addEmoji(client, serverId, name, image);
      },
      () => {
        setName("");
        reload();
      },
    );
  };
  return (
    <Stack gap="sm">
      <TextField
        label={t("servers.emojiAdmin.name")}
        hint={t("servers.emojiAdmin.nameHint")}
        value={name}
        onChangeText={(text) => {
          setName(text.toLowerCase());
        }}
        testID="emoji-name"
      />
      <Button
        label={t("servers.emojiAdmin.add")}
        onPress={add}
        busy={runner.busy}
        disabled={!/^[a-z0-9_]{2,32}$/.test(name)}
        testID="emoji-add"
      />
    </Stack>
  );
}

/** The server's emoji, how many of the 100 are used, and adding more. */
export function EmojiAdmin(props: EmojiAdminProps): ReactNode {
  const { t } = useTranslation();
  const { emoji } = props;
  return (
    <Stack gap="lg">
      <Text tone="muted">{t("servers.emojiAdmin.count", { count: emoji.length })}</Text>
      {emoji.length === 0 ? <Text>{t("servers.emojiAdmin.none")}</Text> : null}
      {emoji.map((item) => (
        <EmojiRow key={item.id} emoji={item} props={props} />
      ))}
      {emoji.length < 100 ? <NewEmoji {...props} /> : null}
    </Stack>
  );
}
