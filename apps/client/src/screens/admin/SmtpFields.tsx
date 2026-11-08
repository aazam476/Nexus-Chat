import { TextField } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { SmtpValues } from "../../admin/useSmtpSettings";
import type { Form } from "../useForm";

/** Props of {@link SmtpFields}. */
export interface SmtpFieldsProps {
  form: Form<SmtpValues>;
  passwordSaved: boolean;
}

/** The text fields of the email settings. */
export function SmtpFields({ form, passwordSaved }: SmtpFieldsProps): ReactNode {
  const { t } = useTranslation();
  const host = form.bind("host");
  const port = form.bind("port");
  const username = form.bind("username");
  const password = form.bind("password");
  const from = form.bind("from");
  const passwordHint = t(passwordSaved ? "admin.smtp.passwordKept" : "admin.smtp.passwordOptional");
  return (
    <>
      <TextField label={t("admin.smtp.host")} keyboardType="url" {...host} />
      <TextField label={t("admin.smtp.port")} keyboardType="number-pad" {...port} />
      <TextField label={t("admin.smtp.username")} autoComplete="off" {...username} />
      <TextField
        label={t("admin.smtp.password")}
        hint={passwordHint}
        secure
        autoComplete="off"
        {...password}
      />
      <TextField
        label={t("admin.smtp.from")}
        hint={t("admin.smtp.fromHint")}
        keyboardType="email-address"
        {...from}
      />
    </>
  );
}
