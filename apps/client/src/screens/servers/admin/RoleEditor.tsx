import type { PermissionName, ServerDetails, ServerRole } from "@nexus-chat/api-client";
import { Button, Heading, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { OptionPicker } from "../../../components/OptionPicker";
import { grantable } from "../../../servers/hierarchy";
import { editRole } from "../../../servers/manageActions";
import { permissionNames } from "../../../servers/permissions";
import type { ServerRunner } from "../useServerRunner";

/** Colors to choose from, as 0xRRGGBB, each with a name, since color is never the only signal. */
const palette = [
  { name: "red", value: 0xe5484d },
  { name: "orange", value: 0xf76b15 },
  { name: "yellow", value: 0xffc53d },
  { name: "green", value: 0x30a46c },
  { name: "teal", value: 0x12a594 },
  { name: "blue", value: 0x0090ff },
  { name: "purple", value: 0x8e4ec6 },
  { name: "pink", value: 0xd6409f },
  { name: "gray", value: 0x8b8d98 },
] as const;

/** Props of {@link RoleEditor}. */
export interface RoleEditorProps {
  details: ServerDetails;
  role: ServerRole;
  runner: ServerRunner;
  saved: () => void;
}

/** Each permission as a check box the user can tick if they hold it themselves. */
function PermissionChecks(props: {
  granted: ReadonlySet<PermissionName>;
  editable: readonly PermissionName[];
  toggle: (permission: PermissionName) => void;
}): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="xs">
      {permissionNames.map((permission) => {
        const on = props.granted.has(permission);
        const label = t(`permissions.${permission}`);
        return (
          <Button
            key={permission}
            label={on ? `✓ ${label}` : label}
            variant={on ? "primary" : "secondary"}
            disabled={!props.editable.includes(permission)}
            onPress={() => {
              props.toggle(permission);
            }}
            testID={`role-perm-${permission}`}
          />
        );
      })}
    </Stack>
  );
}

/** The editor's fields, and saving them. */
function useRoleForm({ details, role, runner, saved }: RoleEditorProps) {
  const [name, setName] = useState(role.name);
  const [color, setColor] = useState<number | undefined>(role.color);
  const [granted, setGranted] = useState<ReadonlySet<PermissionName>>(() => new Set(role.permissions));
  const toggle = (permission: PermissionName): void => {
    setGranted((current) => {
      const next = new Set(current);
      if (!next.delete(permission)) {
        next.add(permission);
      }
      return next;
    });
  };
  const save = (): void => {
    const spec = { name: name.trim(), permissions: [...granted], ...(color === undefined ? {} : { color }) };
    runner.run((client) => editRole(client, { serverId: details.server.id, roleId: role.id }, spec), saved);
  };
  return { name, setName, color, setColor, granted, toggle, save };
}

/** A role's name and color; `@everyone` has neither to change. */
function RoleLook({ form }: { form: ReturnType<typeof useRoleForm> }): ReactNode {
  const { t } = useTranslation();
  const colors = [
    { value: undefined, label: t("servers.roleAdmin.noColor") },
    ...palette.map((swatch) => ({ value: swatch.value, label: t(`servers.colors.${swatch.name}`) })),
  ];
  return (
    <>
      <TextField
        label={t("servers.roleAdmin.name")}
        value={form.name}
        onChangeText={form.setName}
        testID="role-name"
      />
      <OptionPicker
        label={t("servers.roleAdmin.color")}
        options={colors}
        value={form.color}
        onChange={form.setColor}
        testID="role-color"
      />
    </>
  );
}

/** A role's name, color and permissions; `@everyone` keeps its name. */
export function RoleEditor(props: RoleEditorProps): ReactNode {
  const { t } = useTranslation();
  const form = useRoleForm(props);
  return (
    <Stack gap="md">
      {props.role.everyone ? null : <RoleLook form={form} />}
      <Heading level={2}>{t("servers.roleAdmin.permissions")}</Heading>
      <Text tone="muted">{t("servers.channelAdmin.notYours")}</Text>
      <PermissionChecks granted={form.granted} editable={grantable(props.details)} toggle={form.toggle} />
      <Button
        label={t("servers.manage.save")}
        onPress={form.save}
        busy={props.runner.busy}
        disabled={form.name.trim() === ""}
        testID="role-save"
      />
    </Stack>
  );
}
