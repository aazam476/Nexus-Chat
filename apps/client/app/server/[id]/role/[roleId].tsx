import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { ConfirmAction } from "../../../../src/components/ConfirmAction";
import { HttpsWarning } from "../../../../src/components/HttpsWarning";
import { isolate } from "../../../../src/i18n/bidi";
import { RoleEditor } from "../../../../src/screens/servers/admin/RoleEditor";
import { useServerRunner } from "../../../../src/screens/servers/useServerRunner";
import { removeRole } from "../../../../src/servers/manageActions";
import { useServer } from "../../../../src/servers/useServers";

/** Editing one role, and deleting it. */
export default function EditRole(): ReactNode {
  const { t } = useTranslation();
  const { id, roleId } = useLocalSearchParams<{ id: string; roleId: string }>();
  const { server } = useServer(id);
  const runner = useServerRunner();
  const role = server?.details.roles.find((candidate) => candidate.id === roleId);
  const back = (): void => {
    router.back();
  };
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link label={t("servers.back")} onPress={back} testID="edit-role-back" />
        <Heading>{t("servers.roleAdmin.editTitle")}</Heading>
      </Stack>
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
      {server && role ? (
        <RoleEditor key={role.id} details={server.details} role={role} runner={runner} saved={back} />
      ) : null}
      {role === undefined || role.everyone ? null : (
        <ConfirmAction
          label={t("servers.roleAdmin.delete")}
          question={t("servers.roleAdmin.deleteConfirm", { name: isolate(role.name) })}
          confirmLabel={t("servers.roleAdmin.delete")}
          cancelLabel={t("servers.manage.keep")}
          busy={runner.busy}
          onConfirm={() => {
            runner.run((client) => removeRole(client, id, role.id), back);
          }}
          testID="role-delete"
        />
      )}
    </Screen>
  );
}
