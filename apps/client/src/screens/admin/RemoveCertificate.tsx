import { Button, FadeIn, Stack, Text } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link RemoveCertificate}. */
export interface RemoveCertificateProps {
  busy: boolean;
  onRemove: () => Promise<void>;
}

/** Props of the confirmation step. */
interface ConfirmRemovalProps extends RemoveCertificateProps {
  onCancel: () => void;
}

/** What removing does, and the buttons to do it or not. It fades in as a new step. */
function ConfirmRemoval({ busy, onRemove, onCancel }: ConfirmRemovalProps): ReactNode {
  const { t } = useTranslation();
  return (
    <FadeIn kind="step">
      <Stack gap="sm">
        <Text live>{t("admin.tls.removeExplained")}</Text>
        <Button
          label={t("admin.tls.removeConfirm")}
          variant="danger"
          busy={busy}
          onPress={() => void onRemove().then(onCancel)}
          testID="tls-remove-confirm"
        />
        <Button label={t("common.cancel")} variant="secondary" onPress={onCancel} />
      </Stack>
    </FadeIn>
  );
}

/**
 * Removing the certificate turns HTTPS off for everyone, so it takes two
 * presses: the first explains the effect, the second does it.
 */
export function RemoveCertificate({ busy, onRemove }: RemoveCertificateProps): ReactNode {
  const { t } = useTranslation();
  const [confirming, setConfirming] = useState(false);
  if (confirming) {
    return (
      <ConfirmRemoval
        busy={busy}
        onRemove={onRemove}
        onCancel={() => {
          setConfirming(false);
        }}
      />
    );
  }
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
