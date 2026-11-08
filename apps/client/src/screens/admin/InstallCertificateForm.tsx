import { Button, Stack, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link InstallCertificateForm}. */
export interface InstallCertificateFormProps {
  busy: boolean;
  /** Resolves true when the server accepted the certificate. */
  onInstall: (certificate: string, privateKey: string) => Promise<boolean>;
}

/**
 * Two fields for the PEM files, pasted as text. The private key is cleared
 * once the server has it, so it does not stay on screen.
 */
export function InstallCertificateForm({ busy, onInstall }: InstallCertificateFormProps): ReactNode {
  const { t } = useTranslation();
  const [certificate, setCertificate] = useState("");
  const [privateKey, setPrivateKey] = useState("");
  const install = async (): Promise<void> => {
    if (await onInstall(certificate, privateKey)) {
      setCertificate("");
      setPrivateKey("");
    }
  };
  return (
    <Stack>
      <TextField
        label={t("admin.tls.certificate")}
        hint={t("admin.tls.certificateHint")}
        value={certificate}
        onChangeText={setCertificate}
        lines={6}
        testID="tls-certificate"
      />
      <TextField
        label={t("admin.tls.privateKey")}
        hint={t("admin.tls.privateKeyHint")}
        value={privateKey}
        onChangeText={setPrivateKey}
        lines={6}
        testID="tls-private-key"
      />
      <Button
        label={t("admin.tls.install")}
        busy={busy}
        onPress={() => void install()}
        testID="tls-install"
      />
    </Stack>
  );
}
