import type { RegistrationMode } from "@nexus-chat/api-client";
import { Button, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link RegistrationModePicker}. */
export interface RegistrationModePickerProps {
  value: RegistrationMode;
  onChange: (mode: RegistrationMode) => void;
}

const modes: { mode: RegistrationMode; label: string }[] = [
  { mode: "invite_only", label: "setup.modeInviteOnly" },
  { mode: "open", label: "setup.modeOpen" },
  { mode: "closed", label: "setup.modeClosed" },
];

/**
 * A choice of three, shown as buttons. The chosen one is the primary
 * button and is also announced as selected, so the choice never depends on
 * color alone.
 */
export function RegistrationModePicker({ value, onChange }: RegistrationModePickerProps): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      <Text variant="label">{t("setup.registrationMode")}</Text>
      {modes.map(({ mode, label }) => (
        <Button
          key={mode}
          label={mode === value ? `✓ ${t(label)}` : t(label)}
          variant={mode === value ? "primary" : "secondary"}
          onPress={() => {
            onChange(mode);
          }}
          testID={`mode-${mode}`}
        />
      ))}
    </Stack>
  );
}
