import type { PermissionName } from "@nexus-chat/api-client";
import { Button, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { OverrideState } from "../../../servers/overrides";

const choices: { state: OverrideState; label: string }[] = [
  { state: "allow", label: "servers.channelAdmin.allow" },
  { state: "neutral", label: "servers.channelAdmin.neutral" },
  { state: "deny", label: "servers.channelAdmin.deny" },
];

/** Props of {@link PermissionStates}. */
export interface PermissionStatesProps {
  states: ReadonlyMap<PermissionName, OverrideState>;
  /** The permissions the user may change; the others are shown, but locked. */
  editable: readonly PermissionName[];
  onChange: (permission: PermissionName, state: OverrideState) => void;
}

/** One permission: what it does, and allow, inherit or deny, the chosen one marked with a check. */
function PermissionState(props: {
  permission: PermissionName;
  state: OverrideState;
  editable: boolean;
  onChange: PermissionStatesProps["onChange"];
}): ReactNode {
  const { t } = useTranslation();
  const { permission, state, editable } = props;
  return (
    <Stack gap="xs">
      <Text>{t(`permissions.${permission}`)}</Text>
      <Stack gap="xs" direction="row">
        {choices.map((choice) => (
          <Button
            key={choice.state}
            label={choice.state === state ? `✓ ${t(choice.label)}` : t(choice.label)}
            variant={choice.state === state ? "primary" : "secondary"}
            disabled={!editable}
            onPress={() => {
              props.onChange(permission, choice.state);
            }}
            testID={`perm-${permission}-${choice.state}`}
          />
        ))}
      </Stack>
    </Stack>
  );
}

/** Every permission an override can change, each three ways. */
export function PermissionStates({ states, editable, onChange }: PermissionStatesProps): ReactNode {
  return (
    <Stack gap="md">
      {[...states].map(([permission, state]) => (
        <PermissionState
          key={permission}
          permission={permission}
          state={state}
          editable={editable.includes(permission)}
          onChange={onChange}
        />
      ))}
    </Stack>
  );
}
