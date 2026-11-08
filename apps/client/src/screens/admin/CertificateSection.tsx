import { Heading, Stack, Text } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useTlsSettings } from "../../admin/useTlsSettings";
import { ErrorMessage } from "../../components/ErrorMessage";
import { NoticeMessage } from "../../components/NoticeMessage";
import { CertificateDetails } from "./CertificateDetails";
import { InstallCertificateForm } from "./InstallCertificateForm";
import { RemoveCertificate } from "./RemoveCertificate";

/** The HTTPS certificate: its state first, then replacing or removing it (spec §3.3). */
export function CertificateSection(): ReactNode {
  const { t } = useTranslation();
  const tls = useTlsSettings();
  const [now] = useState(() => new Date());
  return (
    <Stack>
      <Heading level={2}>{t("admin.tls.title")}</Heading>
      {tls.status === undefined ? (
        <Text tone="muted">{t("common.loading")}</Text>
      ) : (
        <CertificateDetails status={tls.status} now={now} />
      )}
      <Text tone="muted">{t("admin.tls.proxyNote")}</Text>
      <InstallCertificateForm busy={tls.busy} onInstall={tls.install} />
      {tls.status?.active === true && <RemoveCertificate busy={tls.busy} onRemove={tls.remove} />}
      <ErrorMessage message={tls.error} />
      <NoticeMessage message={tls.notice} testID="tls-notice" />
    </Stack>
  );
}
