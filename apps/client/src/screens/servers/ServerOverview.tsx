import type { Server } from "@nexus-chat/api-client";
import { Avatar, Button, Heading, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { OptionPicker } from "../../components/OptionPicker";
import { useConnection } from "../../connection/ConnectionContext";
import { isolate } from "../../i18n/bidi";
import { pickSquareImage } from "../../platform/pickImage";
import { removeIcon, saveSettings, uploadIcon } from "../../servers/manageActions";
import type { ServerRunner } from "./useServerRunner";

/** The server's icon, and changing it. */
function IconSetting({ server, runner }: { server: Server; runner: ServerRunner }): ReactNode {
  const { t } = useTranslation();
  const { address } = useConnection();
  const icon =
    server.iconUrl === undefined || address === undefined ? undefined : `${address}${server.iconUrl}`;
  return (
    <Stack gap="sm">
      <Text variant="label">{t("servers.manage.icon")}</Text>
      <Avatar name={isolate(server.name)} size={96} {...(icon === undefined ? {} : { imageUrl: icon })} />
      <Button
        label={t("servers.manage.chooseIcon")}
        variant="secondary"
        disabled={runner.busy}
        onPress={() => {
          runner.run(async (client) => {
            const image = await pickSquareImage();
            return image === undefined ? undefined : uploadIcon(client, server.id, image);
          });
        }}
        testID="server-choose-icon"
      />
      {icon === undefined ? null : (
        <Button
          label={t("servers.manage.removeIcon")}
          variant="secondary"
          disabled={runner.busy}
          onPress={() => {
            runner.run((client) => removeIcon(client, server.id));
          }}
          testID="server-remove-icon"
        />
      )}
    </Stack>
  );
}

/** Props of {@link ServerOverview}. */
export interface ServerOverviewProps {
  server: Server;
  runner: ServerRunner;
}

/** Whether members' apps compress photos here by default (ADR 0014). */
function MediaSetting({
  compress,
  setCompress,
}: {
  compress: boolean;
  setCompress: (on: boolean) => void;
}): ReactNode {
  const { t } = useTranslation();
  return (
    <>
      <Text tone="muted">{t("servers.manage.compressHint")}</Text>
      <OptionPicker
        label={t("servers.manage.compress")}
        options={[
          { value: true, label: t("servers.manage.compressOn") },
          { value: false, label: t("servers.manage.compressOff") },
        ]}
        value={compress}
        onChange={setCompress}
        testID="settings-compress"
      />
    </>
  );
}

/** The overview's fields, and saving them. */
function useOverview({ server, runner }: ServerOverviewProps) {
  const [name, setName] = useState(server.name);
  const [description, setDescription] = useState(server.description);
  const [compress, setCompress] = useState(server.compressMedia);
  const [saved, setSaved] = useState(false);
  const save = (): void => {
    setSaved(false);
    const settings = {
      name,
      description,
      compressMedia: compress,
      attachmentMaxBytes: server.attachmentMaxBytes,
    };
    runner.run(
      (client) => saveSettings(client, server.id, settings),
      () => {
        setSaved(true);
      },
    );
  };
  return { name, setName, description, setDescription, compress, setCompress, saved, save };
}

/** The server's icon, name, description and media setting, for those with MANAGE_SERVER. */
export function ServerOverview({ server, runner }: ServerOverviewProps): ReactNode {
  const { t } = useTranslation();
  const form = useOverview({ server, runner });
  return (
    <Stack gap="md">
      <Heading level={2}>{t("servers.manage.overview")}</Heading>
      <IconSetting server={server} runner={runner} />
      <TextField
        label={t("servers.name")}
        hint={t("servers.nameHint")}
        value={form.name}
        onChangeText={form.setName}
        testID="settings-name"
      />
      <TextField
        label={t("servers.description")}
        hint={t("servers.descriptionHint")}
        value={form.description}
        onChangeText={form.setDescription}
        lines={3}
        testID="settings-description"
      />
      <MediaSetting compress={form.compress} setCompress={form.setCompress} />
      <Button
        label={t("servers.manage.save")}
        onPress={form.save}
        busy={runner.busy}
        testID="settings-save"
      />
      {form.saved ? <Text live>{t("servers.manage.saved")}</Text> : null}
    </Stack>
  );
}
