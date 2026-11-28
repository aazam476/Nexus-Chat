import { listServerBans, type ServerBan } from "@nexus-chat/api-client";
import { Button, Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { useLoaded } from "../../../src/hooks/useLoaded";
import { isolate } from "../../../src/i18n/bidi";
import { useServerRunner, type ServerRunner } from "../../../src/screens/servers/useServerRunner";
import { unban } from "../../../src/servers/moderationActions";

/** One ban: who, why, since when, and lifting it. */
function BanRow({
  ban,
  serverId,
  runner,
  lifted,
}: {
  ban: ServerBan;
  serverId: string;
  runner: ServerRunner;
  lifted: () => void;
}): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="xs">
      <Text variant="label">{isolate(ban.user.displayName)}</Text>
      <Text variant="small" tone="muted">
        {t("moderation.bans.since", {
          username: isolate(ban.user.username),
          date: new Date(ban.createdAt).toLocaleDateString(),
        })}
      </Text>
      {ban.reason === "" ? null : <Text>{t("moderation.bans.reason", { reason: isolate(ban.reason) })}</Text>}
      <Button
        label={t("moderation.bans.lift")}
        accessibilityLabel={t("moderation.bans.liftFor", { name: isolate(ban.user.displayName) })}
        variant="secondary"
        disabled={runner.busy}
        onPress={() => {
          runner.run((client) => unban(client, { serverId, userId: ban.user.id }), lifted);
        }}
        testID={`ban-lift-${ban.user.id}`}
      />
    </Stack>
  );
}

/** Who is banned from the server (BAN_MEMBERS), and lifting bans. */
export default function Bans(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const runner = useServerRunner();
  const bans = useLoaded(
    async (client) => (await listServerBans({ client, path: { serverId: id } })).data,
    id,
  );
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="bans-back"
        />
        <Heading>{t("moderation.bans.title")}</Heading>
      </Stack>
      {bans.value === undefined ? <Text tone="muted">{t("common.loading")}</Text> : null}
      {bans.value?.length === 0 ? <Text tone="muted">{t("moderation.bans.none")}</Text> : null}
      {bans.value?.map((ban) => (
        <BanRow key={ban.user.id} ban={ban} serverId={id} runner={runner} lifted={bans.reload} />
      ))}
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
    </Screen>
  );
}
