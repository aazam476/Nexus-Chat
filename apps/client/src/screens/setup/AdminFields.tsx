import { Heading, TextField } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { SetupForm } from "../../auth/useSetup";
import { AccountFields } from "../register/AccountFields";
import type { Form } from "../useForm";

/** Props of {@link AdminFields}. */
export interface AdminFieldsProps {
  form: Form<SetupForm>;
}

/** The setup token and the first admin's account. */
export function AdminFields({ form }: AdminFieldsProps): ReactNode {
  const { t } = useTranslation();
  const token = form.bind("token", "setupToken");
  return (
    <>
      <TextField label={t("setup.token")} {...token} />
      <Heading level={2}>{t("setup.adminSection")}</Heading>
      <AccountFields
        username={form.bind("username", "admin.username")}
        displayName={form.bind("displayName", "admin.displayName")}
        email={form.bind("email", "admin.email")}
        password={form.bind("password", "admin.password")}
      />
    </>
  );
}
