import type { Conversation } from "@nexus-chat/api-client";
import { Avatar, Button, Heading, Stack, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import type { GroupSettings } from "./useGroupSettings";

/** Props of {@link GroupIdentity}. */
export interface GroupIdentityProps {
  conversation: Conversation;
  /** The group's name as lists show it. */
  title: string;
  settings: GroupSettings;
}

function GroupIcon({ conversation, title, settings }: GroupIdentityProps): ReactNode {
  const { t } = useTranslation();
  const { address } = useConnection();
  const icon =
    conversation.iconUrl === undefined || address === undefined
      ? undefined
      : `${address}${conversation.iconUrl}`;
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("groups.icon")}</Heading>
      <Avatar name={title} size={96} {...(icon === undefined ? {} : { imageUrl: icon })} />
      <Button
        label={t("groups.chooseIcon")}
        variant="secondary"
        onPress={settings.chooseIcon}
        disabled={settings.busy}
        testID="group-choose-icon"
      />
      {icon === undefined ? null : (
        <Button
          label={t("groups.removeIcon")}
          variant="secondary"
          onPress={settings.removeIcon}
          disabled={settings.busy}
          testID="group-remove-icon"
        />
      )}
    </Stack>
  );
}

function GroupName({ conversation, settings }: Omit<GroupIdentityProps, "title">): ReactNode {
  const { t } = useTranslation();
  const [name, setName] = useState(conversation.name ?? "");
  return (
    <Stack gap="sm">
      <TextField
        label={t("groups.name")}
        hint={t("groups.nameHint")}
        value={name}
        onChangeText={setName}
        testID="group-rename"
      />
      <Button
        label={t("groups.rename")}
        variant="secondary"
        onPress={() => {
          settings.rename(name);
        }}
        disabled={settings.busy}
        testID="group-save-name"
      />
    </Stack>
  );
}

/** The group's icon and name, and changing them. */
export function GroupIdentity(props: GroupIdentityProps): ReactNode {
  return (
    <Stack gap="lg">
      <GroupIcon {...props} />
      <GroupName conversation={props.conversation} settings={props.settings} />
    </Stack>
  );
}
