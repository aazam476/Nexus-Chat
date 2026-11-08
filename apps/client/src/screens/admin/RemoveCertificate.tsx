import { Button, Stack, Text } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link RemoveCertificate}. */
export interface RemoveCertificateProps {
  busy: boolean;
  onRemove: () => Promise<void>;
}

/**
 * Removing the certificate turns HTTPS off for everyone, so it takes two
 * presses: the first explains the effect, the second does it.
 */
export function RemoveCertificate({ busy, onRemove }: RemoveCertificateProps): ReactNode {
  const { t } = useTranslation();
  const [confirming, setConfirming] = useState(false);
  if (!confirming) {
    return (
      <Button
        label={t("admin.tls.remove")}
        variant="secondary"
        onPress={() => {
          setConfirming(true);
        }}
        testID="tls-remove"
      />
    );
  }
  return (
    <Stack gap="sm">
      <Text live>{t("admin.tls.removeExplained")}</Text>
      <Button
        label={t("admin.tls.removeConfirm")}
        variant="danger"
        busy={busy}
        onPress={() =>
          void onRemove().then(() => {
            setConfirming(false);
          })
        }
        testID="tls-remove-confirm"
      />
      <Button
        label={t("common.cancel")}
        variant="secondary"
        onPress={() => {
          setConfirming(false);
        }}
      />
    </Stack>
  );
}
