import type { ServerDetails } from "@nexus-chat/api-client";
import { Link, Stack } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { LeaveServer } from "./LeaveServer";

const managing = ["MANAGE_SERVER", "MANAGE_CHANNELS", "MANAGE_ROLES"] as const;

/** What the user may do with the server: invite, manage it, or leave. */
export function ServerActions({ details }: { details: ServerDetails }): ReactNode {
  const { t } = useTranslation();
  const { user } = useConnection();
  const id = details.server.id;
  const may = (permission: string): boolean => details.permissions.some((held) => held === permission);
  return (
    <Stack gap="sm">
      {may("CREATE_INVITES") ? (
        <Link
          label={t("servers.invite")}
          onPress={() => {
            router.push({ pathname: "/server/[id]/invites", params: { id } });
          }}
          testID="server-invites"
        />
      ) : null}
      {managing.some(may) ? (
        <Link
          label={t("servers.settings")}
          onPress={() => {
            router.push({ pathname: "/server/[id]/settings", params: { id } });
          }}
          testID="server-settings"
        />
      ) : null}
      {details.server.ownerId === user?.id ? null : <LeaveServer serverId={id} name={details.server.name} />}
    </Stack>
  );
}
