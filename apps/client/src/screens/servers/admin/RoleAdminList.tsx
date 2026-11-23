import type { ServerDetails, ServerMember, ServerRole } from "@nexus-chat/api-client";
import { Button, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import { type ReactNode, useState } from "react";
import { useTranslation } from "react-i18next";

import { DraggableRow, useRowLayouts } from "../../../components/DraggableRow";
import { useConnection } from "../../../connection/ConnectionContext";
import { isolate } from "../../../i18n/bidi";
import { droppedRole, placedRoles } from "../../../servers/dropOrder";
import { mayManageRole, rolesHighestFirst } from "../../../servers/hierarchy";
import { placeRoles } from "../../../servers/manageActions";
import { movedRole } from "../../../servers/roleOrder";
import type { ServerRunner } from "../useServerRunner";

/** Props of {@link RoleAdminList}. */
export interface RoleAdminListProps {
  details: ServerDetails;
  members: readonly ServerMember[];
  runner: ServerRunner;
}

/** Moving a role past its neighbor, where the user may. */
function RoleMoves({
  role,
  name,
  props,
}: {
  role: ServerRole;
  name: string;
  props: RoleAdminListProps;
}): ReactNode {
  const { t } = useTranslation();
  const { user } = useConnection();
  const { details, runner } = props;
  const moves = [
    { by: -1 as const, label: "servers.roleAdmin.moveUp", named: "servers.roleAdmin.moveUpNamed", id: "up" },
    {
      by: 1 as const,
      label: "servers.roleAdmin.moveDown",
      named: "servers.roleAdmin.moveDownNamed",
      id: "down",
    },
  ];
  return moves.map(({ by, label, named, id }) => {
    const order = movedRole(details, user?.id ?? "", role.id, by);
    return order === undefined ? null : (
      <Button
        key={id}
        label={t(label)}
        accessibilityLabel={t(named, { name })}
        variant="secondary"
        disabled={runner.busy}
        onPress={() => {
          runner.run((client) => placeRoles(client, details.server.id, order));
        }}
        testID={`role-${id}-${role.id}`}
      />
    );
  });
}

/** One role: its name, how many have it, and what the user may do with it. */
function RoleRow({ role, props }: { role: ServerRole; props: RoleAdminListProps }): ReactNode {
  const { t } = useTranslation();
  const { user } = useConnection();
  const { details, members } = props;
  const name = role.everyone ? t("servers.roleAdmin.everyone") : isolate(role.name);
  const count = role.everyone
    ? members.length
    : members.filter((member) => member.roleIds.includes(role.id)).length;
  return (
    <Stack gap="xs">
      <Text>{name}</Text>
      <Text variant="small" tone="muted">
        {t("servers.roleAdmin.members", { count })}
      </Text>
      <Stack gap="xs" direction="row">
        <RoleMoves role={role} name={name} props={props} />
        {mayManageRole(details, user?.id ?? "", role) ? (
          <Button
            label={t("servers.channelAdmin.edit")}
            accessibilityLabel={t("servers.channelAdmin.editNamed", { name })}
            variant="secondary"
            onPress={() => {
              router.push({
                pathname: "/server/[id]/role/[roleId]",
                params: { id: details.server.id, roleId: role.id },
              });
            }}
            testID={`role-edit-${role.id}`}
          />
        ) : null}
      </Stack>
    </Stack>
  );
}

/**
 * The server's roles, highest first. Roles the user may move can also be
 * held and dragged, but never past a role at or above their own.
 */
export function RoleAdminList(props: RoleAdminListProps): ReactNode {
  const { user } = useConnection();
  const [dropped, setDropped] = useState<{ base: ServerDetails; ids: string[] }>();
  const details =
    dropped?.base === props.details && props.runner.error === undefined
      ? placedRoles(props.details, dropped.ids)
      : props.details;
  const layouts = useRowLayouts();
  const drop = (role: ServerRole, from: number, translationY: number): void => {
    const ids = droppedRole(details, user?.id ?? "", role.id, layouts.target(from, translationY));
    if (ids !== undefined) {
      setDropped({ base: props.details, ids });
      props.runner.run((client) => placeRoles(client, details.server.id, ids));
    }
  };
  const shownProps = { ...props, details };
  return (
    <Stack gap="md">
      {rolesHighestFirst(details).map((role, index) =>
        role.everyone ? (
          <RoleRow key={role.id} role={role} props={shownProps} />
        ) : (
          <DraggableRow
            key={role.id}
            enabled={!props.runner.busy && mayManageRole(details, user?.id ?? "", role)}
            onLayout={layouts.track(index)}
            onDrop={(translationY) => {
              drop(role, index, translationY);
            }}
            testID={`role-drag-${role.id}`}
          >
            <RoleRow role={role} props={shownProps} />
          </DraggableRow>
        ),
      )}
    </Stack>
  );
}
