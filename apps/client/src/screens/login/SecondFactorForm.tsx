import type { MfaMethod } from "@nexus-chat/api-client";
import { Button, Heading, Link, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link SecondFactorForm}. */
export interface SecondFactorFormProps {
  methods: MfaMethod[];
  busy: boolean;
  onSubmit: (method: MfaMethod, code: string) => void;
  onCancel: () => void;
}

/**
 * The second step of a login: a code from an authenticator app, or a backup
 * code. Security keys are offered with passkeys once the browser flow for
 * them lands.
 */
export function SecondFactorForm({ methods, busy, onSubmit, onCancel }: SecondFactorFormProps): ReactNode {
  const { t } = useTranslation();
  const [method, setMethod] = useState<MfaMethod>(methods.includes("totp") ? "totp" : "backup_code");
  const [code, setCode] = useState("");
  const other: MfaMethod = method === "totp" ? "backup_code" : "totp";
  const submit = (): void => {
    onSubmit(method, code);
  };
  return (
    <Stack>
      <Heading level={2}>{t("mfa.title")}</Heading>
      <Text tone="muted">{t("mfa.intro")}</Text>
      <TextField
        label={method === "totp" ? t("mfa.code") : t("mfa.backupCode")}
        value={code}
        onChangeText={setCode}
        keyboardType={method === "totp" ? "number-pad" : "default"}
        autoComplete="one-time-code"
        onSubmitEditing={submit}
        testID="mfa-code"
      />
      <Button label={t("mfa.submit")} busy={busy} onPress={submit} testID="mfa-submit" />
      {methods.includes(other) && (
        <Link
          label={other === "totp" ? t("mfa.useAuthenticator") : t("mfa.useBackupCode")}
          onPress={() => {
            setMethod(other);
            setCode("");
          }}
        />
      )}
      <Link label={t("common.back")} onPress={onCancel} />
    </Stack>
  );
}
