import { Banner, Heading, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../components/HttpsWarning";

/** Props of {@link SetupIntro}. */
export interface SetupIntroProps {
  host: string;
  secure: boolean;
}

/**
 * The wizard's title and warnings. Over plain HTTP the setup token and the
 * admin password travel unencrypted, which the wizard says plainly (spec
 * §3.5).
 */
export function SetupIntro({ host, secure }: SetupIntroProps): ReactNode {
  const { t } = useTranslation();
  return (
    <>
      <HttpsWarning />
      {!secure && <Banner title={t("setup.httpWarning")} testID="setup-http-warning" />}
      <Stack gap="sm">
        <Heading>{t("setup.title", { instance: host })}</Heading>
        <Text tone="muted">{t("setup.intro")}</Text>
      </Stack>
    </>
  );
}
