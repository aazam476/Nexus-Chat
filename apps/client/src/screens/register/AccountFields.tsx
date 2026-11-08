import { TextField } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { FieldBinding } from "../useForm";

/** Props of {@link AccountFields}. */
export interface AccountFieldsProps {
  username: FieldBinding;
  displayName: FieldBinding;
  email: FieldBinding;
  password: FieldBinding;
}

/** The fields of a new account, shared by sign-up and the setup wizard. */
export function AccountFields({ username, displayName, email, password }: AccountFieldsProps): ReactNode {
  const { t } = useTranslation();
  return (
    <>
      <TextField
        label={t("register.username")}
        hint={t("register.usernameHint")}
        autoComplete="username"
        {...username}
      />
      <TextField
        label={t("register.displayName")}
        autoCapitalize="words"
        autoComplete="name"
        {...displayName}
      />
      <TextField label={t("register.email")} keyboardType="email-address" autoComplete="email" {...email} />
      <TextField
        label={t("register.password")}
        hint={t("register.passwordHint")}
        secure
        autoComplete="new-password"
        {...password}
      />
    </>
  );
}
