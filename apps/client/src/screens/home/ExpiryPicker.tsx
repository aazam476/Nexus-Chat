import { Button, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** When a custom status clears itself, in minutes; 0 means it stays until changed. */
export type ExpiryChoice = 0 | 60 | 240 | 1440;

const choices: { minutes: ExpiryChoice; label: string }[] = [
  { minutes: 0, label: "home.expiryNever" },
  { minutes: 60, label: "home.expiryHour" },
  { minutes: 240, label: "home.expiryFourHours" },
  { minutes: 1440, label: "home.expiryDay" },
];

/** Props of {@link ExpiryPicker}. */
export interface ExpiryPickerProps {
  value: ExpiryChoice;
  onChange: (minutes: ExpiryChoice) => void;
}

/** How long the custom status lasts; the chosen one is marked, not only colored. */
export function ExpiryPicker({ value, onChange }: ExpiryPickerProps): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      <Text variant="label">{t("home.expiry")}</Text>
      {choices.map(({ minutes, label }) => (
        <Button
          key={minutes}
          label={minutes === value ? `✓ ${t(label)}` : t(label)}
          variant={minutes === value ? "primary" : "secondary"}
          onPress={() => {
            onChange(minutes);
          }}
          testID={`custom-expiry-${String(minutes)}`}
        />
      ))}
    </Stack>
  );
}
