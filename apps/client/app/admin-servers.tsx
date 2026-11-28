import { deleteAdminServer, listAdminServers, type AdminServer } from "@nexus-chat/api-client";
import { Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { ConfirmAction } from "../src/components/ConfirmAction";
import { useLoaded } from "../src/hooks/useLoaded";
import { isolate } from "../src/i18n/bidi";
import { AdminPage } from "../src/screens/admin/AdminPage";
import { useServerRunner, type ServerRunner } from "../src/screens/servers/useServerRunner";
import { resultOf } from "../src/servers/serverActions";

/** One server: its name, owner and size, and deleting it. */
function ServerRow({
  server,
  runner,
  deleted,
}: {
  server: AdminServer;
  runner: ServerRunner;
  deleted: () => void;
}): ReactNode {
  const { t } = useTranslation();
  const name = isolate(server.name);
  return (
    <Stack gap="xs">
      <Text variant="label">{name}</Text>
      <Text variant="small" tone="muted">
        {t("admin.servers.about", {
          count: server.members,
          owner: isolate(server.ownerUsername ?? t("moderation.audit.someone")),
        })}
      </Text>
      <ConfirmAction
        label={t("admin.servers.delete")}
        accessibilityLabel={t("admin.servers.deleteNamed", { name })}
        question={t("admin.servers.deleteConfirm", { name })}
        confirmLabel={t("admin.servers.deleteYes")}
        cancelLabel={t("moderation.cancel")}
        busy={runner.busy}
        onConfirm={() => {
          runner.run(
            async (client) => resultOf(await deleteAdminServer({ client, path: { serverId: server.id } })),
            deleted,
          );
        }}
        testID={`admin-server-delete-${server.id}`}
      />
    </Stack>
  );
}

/** Every server on the instance, to find and delete. */
export default function AdminServers(): ReactNode {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const runner = useServerRunner();
  const servers = useLoaded(
    async (client) => (await listAdminServers({ client, query: { q: query } })).data,
    query,
  );
  return (
    <AdminPage title={t("admin.servers.title")} intro={t("admin.servers.intro")}>
      <TextField
        label={t("admin.servers.search")}
        value={query}
        onChangeText={setQuery}
        testID="admin-servers-search"
      />
      {servers.value?.length === 0 ? <Text tone="muted">{t("admin.servers.none")}</Text> : null}
      {servers.value?.map((server) => (
        <ServerRow key={server.id} server={server} runner={runner} deleted={servers.reload} />
      ))}
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
    </AdminPage>
  );
}
