import type { OverrideTargetKind } from "@nexus-chat/api-client";
import { Button, Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../../../src/components/HttpsWarning";
import { isolate } from "../../../../../src/i18n/bidi";
import { PermissionStates } from "../../../../../src/screens/servers/admin/PermissionStates";
import {
  useOverrideEditor,
  type OverrideWhere,
} from "../../../../../src/screens/servers/admin/useOverrideEditor";
import { useServerRunner, type ServerRunner } from "../../../../../src/screens/servers/useServerRunner";
import { grantable } from "../../../../../src/servers/hierarchy";
import { targetName } from "../../../../../src/servers/overrides";
import { useServer } from "../../../../../src/servers/useServers";

/** The editor for an override of a loaded server. */
function OverrideForm({ where, runner }: { where: OverrideWhere; runner: ServerRunner }): ReactNode {
  const { t } = useTranslation();
  const editor = useOverrideEditor(where, runner);
  const { details, members } = where.server;
  const channel = details.channels.find((candidate) => candidate.id === where.channelId);
  const who = isolate(targetName(details, members, where));
  return (
    <Stack gap="md">
      <Heading>
        {t("servers.channelAdmin.overrideTitle", { name: who, channel: isolate(channel?.name ?? "") })}
      </Heading>
      <Text tone="muted">{t("servers.channelAdmin.notYours")}</Text>
      <PermissionStates states={editor.states} editable={grantable(details)} onChange={editor.set} />
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
      <Button
        label={t("servers.manage.save")}
        onPress={() => {
          editor.save(() => {
            router.back();
          });
        }}
        busy={runner.busy}
        testID="override-save"
      />
    </Stack>
  );
}

/** What a channel allows and denies for one role or person. */
export default function ChannelOverride(): ReactNode {
  const { t } = useTranslation();
  const { id, channelId, kind, target } = useLocalSearchParams<{
    id: string;
    channelId: string;
    kind: OverrideTargetKind;
    target: string;
  }>();
  const { server } = useServer(id);
  const runner = useServerRunner();
  return (
    <Screen>
      <HttpsWarning />
      <Link
        label={t("servers.back")}
        onPress={() => {
          router.back();
        }}
        testID="override-back"
      />
      {server ? (
        <OverrideForm where={{ server, channelId, targetKind: kind, targetId: target }} runner={runner} />
      ) : null}
    </Screen>
  );
}
