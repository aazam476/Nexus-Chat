import type { ServerDetails, ServerMember, ServerRole } from "@nexus-chat/api-client";
import { Button, Heading, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../../connection/ConnectionContext";
import { isolate } from "../../../i18n/bidi";
import { mayManageRole, rolesHighestFirst } from "../../../servers/hierarchy";
import { setMemberRole } from "../../../servers/manageActions";
import type { ServerRunner } from "../useServerRunner";

/** Props of {@link MemberRoles}. */
export interface MemberRolesProps {
  details: ServerDetails;
  member: ServerMember;
  runner: ServerRunner;
}

/** One role of the member: a button to give or take it, or only whether they have it. */
function RoleToggle({ details, member, runner, role }: MemberRolesProps & { role: ServerRole }): ReactNode {
  const { t } = useTranslation();
  const { user } = useConnection();
  const has = member.roleIds.includes(role.id);
  const name = isolate(role.name);
  if (!mayManageRole(details, user?.id ?? "", role)) {
    return <Text>{t(has ? "servers.memberAdmin.has" : "servers.memberAdmin.lacks", { role: name })}</Text>;
  }
  const where = { serverId: details.server.id, userId: member.user.id, roleId: role.id };
  return (
    <Button
      label={
        has
          ? `✓ ${t("servers.memberAdmin.take", { role: name })}`
          : t("servers.memberAdmin.give", { role: name })
      }
      variant={has ? "primary" : "secondary"}
      disabled={runner.busy}
      onPress={() => {
        runner.run((client) => setMemberRole(client, where, !has));
      }}
      testID={`member-role-${role.id}`}
    />
  );
}

/** A member's roles: each one given or taken, where the user may; the rest only shown. */
export function MemberRoles(props: MemberRolesProps): ReactNode {
  const { t } = useTranslation();
  const roles = rolesHighestFirst(props.details).filter((role) => !role.everyone);
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("servers.memberAdmin.roles")}</Heading>
      {roles.length === 0 ? <Text tone="muted">{t("servers.memberAdmin.noRoles")}</Text> : null}
      {roles.map((role) => (
        <RoleToggle key={role.id} {...props} role={role} />
      ))}
    </Stack>
  );
}
