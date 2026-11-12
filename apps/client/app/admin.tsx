import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { useConnection } from "../src/connection/ConnectionContext";
import { CertificateSection } from "../src/screens/admin/CertificateSection";
import { SmtpSection } from "../src/screens/admin/SmtpSection";
import { UploadSection } from "../src/screens/admin/UploadSection";

/**
 * The Admin panel. The HTTPS warning leads every admin page while the
 * instance is on plain HTTP (spec §3.3). Only instance admins get here;
 * the server checks that on every request as well.
 */
export default function Admin(): ReactNode {
  const { t } = useTranslation();
  const connection = useConnection();
  const user = connection.user;
  const allowed = user?.isInstanceAdmin === true;
  useEffect(() => {
    if (!connection.loading && !allowed) {
      router.replace("/");
    }
  }, [connection.loading, allowed]);
  if (!allowed) {
    return null;
  }
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Heading>{t("admin.title")}</Heading>
        <Text tone="muted">{t("admin.intro")}</Text>
        <Link
          label={t("admin.back")}
          onPress={() => {
            router.replace("/home");
          }}
          testID="admin-back"
        />
      </Stack>
      <CertificateSection />
      <SmtpSection />
      <UploadSection />
    </Screen>
  );
}
