import type { TlsStatus } from "@nexus-chat/api-client";
import { Banner, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { expiryWarning } from "../../admin/expiry";

/** Props of {@link CertificateDetails}. */
export interface CertificateDetailsProps {
  status: TlsStatus;
  now: Date;
}

/** What the installed certificate covers and until when, with a warning as expiry nears. */
export function CertificateDetails({ status, now }: CertificateDetailsProps): ReactNode {
  const { t, i18n } = useTranslation();
  if (!status.active) {
    return <Text testID="tls-state">{t("admin.tls.inactive")}</Text>;
  }
  const expires =
    status.expiresAt === undefined
      ? ""
      : new Intl.DateTimeFormat(i18n.language, { dateStyle: "long", timeZone: "UTC" }).format(
          new Date(status.expiresAt),
        );
  const warning = expiryWarning(status.expiresAt, now);
  const names = (status.names ?? []).join(", ");
  const issuer = status.issuer ?? "";
  return (
    <Stack gap="sm">
      <Text testID="tls-state">{t("admin.tls.active")}</Text>
      {warning !== undefined && (
        <Banner
          title={t(warning.urgent ? "admin.tls.expiresVerySoon" : "admin.tls.expiresSoon", {
            count: Math.max(warning.daysLeft, 0),
          })}
          testID="tls-expiry-warning"
        >
          <Text>{t("admin.tls.replace")}</Text>
        </Banner>
      )}
      <Text tone="muted">{t("admin.tls.names", { names })}</Text>
      <Text tone="muted">{t("admin.tls.issuer", { issuer })}</Text>
      <Text tone="muted">{t("admin.tls.expires", { date: expires })}</Text>
    </Stack>
  );
}
