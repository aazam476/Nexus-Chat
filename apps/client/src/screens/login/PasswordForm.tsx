import { Button, Stack, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link PasswordForm}. */
export interface PasswordFormProps {
  busy: boolean;
  onSubmit: (login: string, password: string) => void;
}

/** Username or email, and password. */
export function PasswordForm({ busy, onSubmit }: PasswordFormProps): ReactNode {
  const { t } = useTranslation();
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const submit = (): void => {
    onSubmit(login, password);
  };
  return (
    <Stack>
      <TextField
        label={t("login.login")}
        value={login}
        onChangeText={setLogin}
        autoComplete="username"
        testID="login-name"
      />
      <TextField
        label={t("login.password")}
        value={password}
        onChangeText={setPassword}
        secure
        autoComplete="current-password"
        onSubmitEditing={submit}
        testID="login-password"
      />
      <Button label={t("login.submit")} busy={busy} onPress={submit} testID="login-submit" />
    </Stack>
  );
}
