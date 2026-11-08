import { Button, Link, Stack, TextField } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { Registration } from "../../auth/useRegister";
import type { Form } from "../useForm";
import { AccountFields } from "./AccountFields";

/** Props of {@link RegisterForm}. */
export interface RegisterFormProps {
  form: Form<Registration>;
  /** Invite-only instances ask for an invite code. */
  askForInvite: boolean;
  busy: boolean;
  onSubmit: () => void;
}

/** The sign-up fields and buttons. */
export function RegisterForm({ form, askForInvite, busy, onSubmit }: RegisterFormProps): ReactNode {
  const { t } = useTranslation();
  const invite = form.bind("inviteCode");
  return (
    <Stack>
      <AccountFields
        username={form.bind("username")}
        displayName={form.bind("displayName")}
        email={form.bind("email")}
        password={form.bind("password")}
      />
      {askForInvite && <TextField label={t("register.invite")} hint={t("register.inviteHint")} {...invite} />}
      <Button label={t("register.submit")} busy={busy} onPress={onSubmit} testID="register-submit" />
      <Link
        label={t("register.haveAccount")}
        onPress={() => {
          router.replace("/login");
        }}
      />
    </Stack>
  );
}
