import type { OverrideTargetKind, ServerChannel, ServerDetails, ServerMember } from "@nexus-chat/api-client";
import { Button, Heading, ListRow, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { isolate } from "../../../i18n/bidi";
import { targetName } from "../../../servers/overrides";
import { rolesHighestFirst } from "../../../servers/hierarchy";

/** Props of {@link ChannelOverrides}. */
export interface ChannelOverridesProps {
  details: ServerDetails;
  members: readonly ServerMember[];
  channel: ServerChannel;
}

/** Opens the override of a role or member in this channel, new or not. */
function openOverride(
  details: ServerDetails,
  channel: ServerChannel,
  kind: OverrideTargetKind,
  targetId: string,
): void {
  router.push({
    pathname: "/server/[id]/channel/[channelId]/override",
    params: { id: details.server.id, channelId: channel.id, kind, target: targetId },
  });
}

/** The roles and people without an override here yet, to add one for. */
function AddOverride({ details, members, channel }: ChannelOverridesProps): ReactNode {
  const { t } = useTranslation();
  const has = (kind: OverrideTargetKind, id: string) =>
    channel.overrides.some((override) => override.targetKind === kind && override.targetId === id);
  const roles = rolesHighestFirst(details).filter((role) => !has("role", role.id));
  const people = members.filter((member) => !has("member", member.user.id));
  return (
    <Stack gap="sm">
      {roles.length === 0 ? null : <Text variant="label">{t("servers.channelAdmin.addRole")}</Text>}
      {roles.map((role) => (
        <Button
          key={role.id}
          label={isolate(role.name)}
          variant="secondary"
          onPress={() => {
            openOverride(details, channel, "role", role.id);
          }}
          testID={`override-add-role-${role.id}`}
        />
      ))}
      {people.length === 0 ? null : <Text variant="label">{t("servers.channelAdmin.addMember")}</Text>}
      {people.map((member) => (
        <Button
          key={member.user.id}
          label={isolate(member.user.displayName)}
          variant="secondary"
          onPress={() => {
            openOverride(details, channel, "member", member.user.id);
          }}
          testID={`override-add-member-${member.user.id}`}
        />
      ))}
    </Stack>
  );
}

/** The channel's overrides, each opening its editor, and adding another. */
export function ChannelOverrides(props: ChannelOverridesProps): ReactNode {
  const { t } = useTranslation();
  const { details, members, channel } = props;
  return (
    <Stack gap="md">
      <Heading level={2}>{t("servers.channelAdmin.permissions")}</Heading>
      <Text tone="muted">{t("servers.channelAdmin.permissionsIntro")}</Text>
      {channel.overrides.length === 0 ? (
        <Text tone="muted">{t("servers.channelAdmin.noOverrides")}</Text>
      ) : null}
      {channel.overrides.map((override) => (
        <ListRow
          key={`${override.targetKind}-${override.targetId}`}
          title={isolate(targetName(details, members, override))}
          subtitle={[
            ...override.allow.map((name) => `+${name}`),
            ...override.deny.map((name) => `−${name}`),
          ].join(" ")}
          onPress={() => {
            openOverride(details, channel, override.targetKind, override.targetId);
          }}
          testID={`override-${override.targetId}`}
        />
      ))}
      <AddOverride {...props} />
    </Stack>
  );
}
