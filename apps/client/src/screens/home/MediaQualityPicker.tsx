import { Button, Heading, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { mediaQualities, useMediaQuality } from "../../settings/mediaQuality";

/**
 * How photos are sent by default (ADR 0014). The chosen option is marked
 * with a check and the primary style, so it does not depend on color alone.
 */
export function MediaQualityPicker(): ReactNode {
  const { t } = useTranslation();
  const { quality, choose } = useMediaQuality();
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("mediaQuality.title")}</Heading>
      <Text tone="muted">{t("mediaQuality.intro")}</Text>
      {mediaQualities.map((option) => (
        <Button
          key={option}
          label={option === quality ? `✓ ${t(`mediaQuality.${option}`)}` : t(`mediaQuality.${option}`)}
          variant={option === quality ? "primary" : "secondary"}
          onPress={() => {
            choose(option);
          }}
          testID={`media-quality-${option}`}
        />
      ))}
    </Stack>
  );
}
