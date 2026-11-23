import type { ServerDetails } from "@nexus-chat/api-client";
import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { ConfirmAction } from "../../../src/components/ConfirmAction";
import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { useConnection } from "../../../src/connection/ConnectionContext";
import { isolate } from "../../../src/i18n/bidi";
import { ServerOverview } from "../../../src/screens/servers/ServerOverview";
import { useServerRunner, type ServerRunner } from "../../../src/screens/servers/useServerRunner";
import { removeServer } from "../../../src/servers/manageActions";
import { useServer } from "../../../src/servers/useServers";

const sections = [
  {
    permission: "MANAGE_CHANNELS",
    label: "servers.manage.channels",
    pathname: "/server/[id]/channels",
    testID: "manage-channels",
  },
  {
    permission: "MANAGE_ROLES",
    label: "servers.manage.roles",
    pathname: "/server/[id]/roles",
    testID: "manage-roles",
  },
  {
    permission: "VIEW_CHANNEL",
    label: "servers.manage.members",
    pathname: "/server/[id]/members",
    testID: "manage-members",
  },
] as const;

/** Where the settings go: channels, roles and members, as far as the user may manage them. */
function SettingsLinks({ details }: { details: ServerDetails }): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      {sections
        .filter(({ permission }) => permission === "VIEW_CHANNEL" || details.permissions.includes(permission))
        .map(({ label, pathname, testID }) => (
          <Link
            key={pathname}
            label={t(label)}
            onPress={() => {
              router.push({ pathname, params: { id: details.server.id } });
            }}
            testID={testID}
          />
        ))}
    </Stack>
  );
}

/** Deleting the server, for its owner. */
function DeleteServer({ details, runner }: { details: ServerDetails; runner: ServerRunner }): ReactNode {
  const { t } = useTranslation();
  return (
    <ConfirmAction
      label={t("servers.manage.delete")}
      question={t("servers.manage.deleteConfirm", { name: isolate(details.server.name) })}
      confirmLabel={t("servers.manage.deleteYes")}
      cancelLabel={t("servers.manage.keep")}
      busy={runner.busy}
      onConfirm={() => {
        runner.run(
          (client) => removeServer(client, details.server.id),
          () => {
            router.replace("/servers");
          },
        );
      }}
      testID="server-delete"
    />
  );
}

/** A server's settings: its overview, where to manage the rest, and deleting it. */
export default function ServerSettings(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useConnection();
  const { server } = useServer(id);
  const runner = useServerRunner();
  const details = server?.details;
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="settings-back"
        />
        <Heading>{t("servers.manage.title")}</Heading>
      </Stack>
      {details?.permissions.includes("MANAGE_SERVER") === true ? (
        <ServerOverview key={JSON.stringify(details.server)} server={details.server} runner={runner} />
      ) : null}
      {details === undefined ? null : <SettingsLinks details={details} />}
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
      {details !== undefined && details.server.ownerId === user?.id ? (
        <DeleteServer details={details} runner={runner} />
      ) : null}
    </Screen>
  );
}
