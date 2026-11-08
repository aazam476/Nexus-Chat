import type { SmtpSecurity, SmtpSettings } from "@nexus-chat/api-client";
import { Button, Stack } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { FieldMessages } from "../../auth/errors";
import type { SmtpValues } from "../../admin/useSmtpSettings";
import { useForm } from "../useForm";
import { SecurityPicker } from "./SecurityPicker";
import { SmtpFields } from "./SmtpFields";

/** Props of {@link SmtpForm}. */
export interface SmtpFormProps {
  saved: SmtpSettings;
  busy: boolean;
  fields: FieldMessages;
  onSave: (values: SmtpValues, security: SmtpSecurity) => void;
}

/** The mail server's address, sign-in and sender, starting from what is saved. */
export function SmtpForm({ saved, busy, fields, onSave }: SmtpFormProps): ReactNode {
  const { t } = useTranslation();
  const form = useForm<SmtpValues>(initialValues(saved), fields, "smtp");
  const [security, setSecurity] = useState<SmtpSecurity>(saved.configured ? saved.security : "starttls");
  return (
    <Stack>
      <SmtpFields form={form} passwordSaved={saved.passwordSet} />
      <SecurityPicker value={security} onChange={setSecurity} />
      <Button
        label={t("admin.smtp.save")}
        busy={busy}
        onPress={() => {
          onSave(form.values, security);
        }}
        testID="smtp-save"
      />
    </Stack>
  );
}

/** The saved settings as form values; a new setup starts on the usual port. */
function initialValues(saved: SmtpSettings): SmtpValues {
  return {
    host: saved.host,
    port: saved.configured ? String(saved.port) : "587",
    username: saved.username,
    password: "",
    from: saved.from,
  };
}
