import { listAdminUsers } from "@nexus-chat/api-client";
import { Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../src/connection/ConnectionContext";
import { useLoaded } from "../src/hooks/useLoaded";
import { AdminPage } from "../src/screens/admin/AdminPage";
import { UserRow } from "../src/screens/admin/UserRow";
import { useServerRunner } from "../src/screens/servers/useServerRunner";

/** The instance's accounts, to find and to suspend or reinstate. */
export default function AdminUsers(): ReactNode {
  const { t } = useTranslation();
  const { user } = useConnection();
  const [query, setQuery] = useState("");
  const runner = useServerRunner();
  const users = useLoaded(
    async (client) => (await listAdminUsers({ client, query: { q: query } })).data,
    query,
  );
  return (
    <AdminPage title={t("admin.users.title")} intro={t("admin.users.intro")}>
      <TextField
        label={t("admin.users.search")}
        value={query}
        onChangeText={setQuery}
        testID="admin-users-search"
      />
      {users.value?.length === 0 ? <Text tone="muted">{t("admin.users.none")}</Text> : null}
      {users.value?.map((account) => (
        <UserRow
          key={account.id}
          user={account}
          self={account.id === user?.id}
          runner={runner}
          changed={users.reload}
        />
      ))}
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
    </AdminPage>
  );
}
