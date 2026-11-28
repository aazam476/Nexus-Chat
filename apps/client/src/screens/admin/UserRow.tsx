import type { AdminUser, Client } from "@nexus-chat/api-client";
import { grantInstanceAdmin, reinstateUser, revokeInstanceAdmin, suspendUser } from "@nexus-chat/api-client";
import { Button, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { isolate } from "../../i18n/bidi";
import { resultOf } from "../../servers/serverActions";
import type { ServerAction, ServerRunner } from "../servers/useServerRunner";

/** Props of {@link UserRow}. */
export interface UserRowProps {
  user: AdminUser;
  self: boolean;
  runner: ServerRunner;
  changed: () => void;
}

const act =
  (
    call: (client: Client) => Promise<{ response?: Response; error?: unknown; data?: unknown }>,
    id: string,
  ): ServerAction =>
  async (client) =>
    resultOf(await call(client), id);

/** Reinstating a suspended account. */
function Reinstate({ user, runner, changed }: UserRowProps): ReactNode {
  const { t } = useTranslation();
  return (
    <Button
      label={t("admin.users.reinstate")}
      variant="secondary"
      disabled={runner.busy}
      onPress={() => {
        runner.run(
          act((client) => reinstateUser({ client, path: { userId: user.id } }), user.id),
          changed,
        );
      }}
      testID={`user-reinstate-${user.id}`}
    />
  );
}

/** Suspending the account with a reason, or reinstating it. */
function Suspension(props: UserRowProps): ReactNode {
  const { t } = useTranslation();
  const [reason, setReason] = useState("");
  const { user, runner, changed } = props;
  if (user.suspendedAt !== undefined) {
    return <Reinstate {...props} />;
  }
  const body = { reason: reason.trim() };
  return (
    <>
      <TextField
        label={t("admin.users.reason")}
        value={reason}
        onChangeText={setReason}
        testID={`user-reason-${user.id}`}
      />
      <Button
        label={t("admin.users.suspend")}
        variant="secondary"
        disabled={runner.busy}
        onPress={() => {
          runner.run(
            act((client) => suspendUser({ client, path: { userId: user.id }, body }), user.id),
            changed,
          );
        }}
        testID={`user-suspend-${user.id}`}
      />
    </>
  );
}

/** One account: who, since when, whether suspended or an admin, and changing that. */
export function UserRow(props: UserRowProps): ReactNode {
  const { t } = useTranslation();
  const { user, self, runner, changed } = props;
  const path = { userId: user.id };
  return (
    <Stack gap="xs">
      <Text variant="label">{isolate(user.displayName)}</Text>
      <Text variant="small" tone="muted">
        {t("admin.users.who", { username: isolate(user.username), email: isolate(user.email) })}
      </Text>
      {user.isAdmin ? <Text variant="small">{t("admin.users.admin")}</Text> : null}
      {user.suspendedAt === undefined ? null : (
        <Text variant="small" tone="danger">
          {t("admin.users.suspended", {
            reason: isolate(user.suspensionReason || t("admin.users.noReason")),
          })}
        </Text>
      )}
      {self ? null : (
        <>
          <Suspension {...props} />
          <Button
            label={t(user.isAdmin ? "admin.users.revokeAdmin" : "admin.users.makeAdmin")}
            variant="secondary"
            disabled={runner.busy}
            onPress={() => {
              const call = user.isAdmin ? revokeInstanceAdmin : grantInstanceAdmin;
              runner.run(
                act((client) => call({ client, path }), user.id),
                changed,
              );
            }}
            testID={`user-admin-${user.id}`}
          />
        </>
      )}
    </Stack>
  );
}
