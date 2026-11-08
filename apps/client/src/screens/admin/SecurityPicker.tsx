import type { SmtpSecurity } from "@nexus-chat/api-client";
import { Button, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link SecurityPicker}. */
export interface SecurityPickerProps {
  value: SmtpSecurity;
  onChange: (security: SmtpSecurity) => void;
}

const choices: { security: SmtpSecurity; label: string }[] = [
  { security: "starttls", label: "admin.smtp.starttls" },
  { security: "tls", label: "admin.smtp.tls" },
  { security: "none", label: "admin.smtp.none" },
];

/** How the connection to the mail server is encrypted; the chosen one is marked, not only colored. */
export function SecurityPicker({ value, onChange }: SecurityPickerProps): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      <Text variant="label">{t("admin.smtp.security")}</Text>
      {choices.map(({ security, label }) => (
        <Button
          key={security}
          label={security === value ? `✓ ${t(label)}` : t(label)}
          variant={security === value ? "primary" : "secondary"}
          onPress={() => {
            onChange(security);
          }}
          testID={`smtp-security-${security}`}
        />
      ))}
    </Stack>
  );
}
