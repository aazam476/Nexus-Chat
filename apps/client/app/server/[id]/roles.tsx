import { Button, Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { RoleAdminList } from "../../../src/screens/servers/admin/RoleAdminList";
import { useServerRunner } from "../../../src/screens/servers/useServerRunner";
import { addRole } from "../../../src/servers/manageActions";
import { useServer } from "../../../src/servers/useServers";

/** Managing roles: their order, editing them, and adding new ones (MANAGE_ROLES). */
export default function RoleAdmin(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { server } = useServer(id);
  const runner = useServerRunner();
  const create = (): void => {
    runner.run(
      (client) => addRole(client, id, { name: t("servers.roleAdmin.newName"), permissions: [] }),
      (roleId) => {
        router.push({ pathname: "/server/[id]/role/[roleId]", params: { id, roleId } });
      },
    );
  };
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="role-admin-back"
        />
        <Heading>{t("servers.roleAdmin.title")}</Heading>
        <Text tone="muted">{t("servers.roleAdmin.intro")}</Text>
      </Stack>
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
      {server ? <RoleAdminList details={server.details} members={server.members} runner={runner} /> : null}
      <Button label={t("servers.roleAdmin.new")} onPress={create} busy={runner.busy} testID="role-new" />
    </Screen>
  );
}
