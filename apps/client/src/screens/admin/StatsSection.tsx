import { getInstanceStats } from "@nexus-chat/api-client";
import { Heading, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useLoaded } from "../../hooks/useLoaded";
import { fileSize } from "../../messaging/fileSize";

/** The instance's numbers, as of opening the panel. */
export function StatsSection(): ReactNode {
  const { t } = useTranslation();
  const stats = useLoaded(async (client) => (await getInstanceStats({ client })).data, "stats").value;
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("admin.stats.title")}</Heading>
      {stats === undefined ? (
        <Text tone="muted">{t("common.loading")}</Text>
      ) : (
        <Stack gap="xs">
          <Text testID="stats-users">
            {t("admin.stats.users", { count: stats.users, suspended: stats.suspendedUsers })}
          </Text>
          <Text>{t("admin.stats.active", { count: stats.activeUsers })}</Text>
          <Text>{t("admin.stats.servers", { count: stats.servers })}</Text>
          <Text>{t("admin.stats.conversations", { count: stats.conversations })}</Text>
          <Text>{t("admin.stats.messages", { count: stats.recentMessages })}</Text>
          <Text>{t("admin.stats.reports", { count: stats.openReports })}</Text>
          <Text>{t("admin.stats.storage", { size: fileSize(stats.attachmentBytes, t) })}</Text>
        </Stack>
      )}
    </Stack>
  );
}
