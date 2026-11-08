import { Banner, Link, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../connection/ConnectionContext";

/** The self-hosting guide's HTTPS section (spec §3.3). */
export const httpsGuideUrl = "https://github.com/aazam476/nexus-chat/blob/main/docs/self-hosting.md#https";

const features = ["calls", "passkeys", "encryption", "push", "mobile", "federation"] as const;

/**
 * The warning shown while the instance is reached over plain HTTP (spec
 * §3.3), listing what does not work until HTTPS is set up. Localhost
 * counts as secure, as browsers allow these features there.
 */
export function HttpsWarning(): ReactNode {
  const { t } = useTranslation();
  const { instance } = useConnection();
  if (instance === undefined || instance.info.secureConnection) {
    return null;
  }
  return (
    <Banner title={t("https.title")} testID="https-warning">
      <Text>{t("https.intro")}</Text>
      {features.map((feature) => (
        <Text key={feature}>{`• ${t(`https.${feature}`)}`}</Text>
      ))}
      <Link label={t("https.learnMore")} href={httpsGuideUrl} />
    </Banner>
  );
}
