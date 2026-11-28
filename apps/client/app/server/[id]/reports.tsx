import { listServerReports } from "@nexus-chat/api-client";
import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { useLoaded } from "../../../src/hooks/useLoaded";
import { ReportActions } from "../../../src/screens/servers/moderation/ReportActions";
import { ReportCard } from "../../../src/screens/servers/moderation/ReportCard";
import { StatusPicker, type ReportStatus } from "../../../src/screens/servers/moderation/StatusPicker";
import { useVerdicts } from "../../../src/screens/servers/moderation/useVerdicts";
import { useServerRunner } from "../../../src/screens/servers/useServerRunner";
import { handleReport } from "../../../src/servers/moderationActions";

/** The screen's title and what it is for. */
function ReportsHeader(): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      <Link
        label={t("servers.back")}
        onPress={() => {
          router.back();
        }}
        testID="reports-back"
      />
      <Heading>{t("moderation.reports.title")}</Heading>
      <Text tone="muted">{t("moderation.reports.intro")}</Text>
    </Stack>
  );
}

/** A server's reports (MANAGE_MESSAGES), each checked on this device, to resolve or pass on. */
export default function ServerReports(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const [status, setStatus] = useState<ReportStatus>("open");
  const runner = useServerRunner();
  const reports = useLoaded(
    async (client) => (await listServerReports({ client, path: { serverId: id }, query: { status } })).data,
    `${id}|${status}`,
  );
  const verdicts = useVerdicts(reports.value);
  return (
    <Screen>
      <HttpsWarning />
      <ReportsHeader />
      <StatusPicker value={status} onChange={setStatus} />
      {reports.value?.length === 0 ? <Text tone="muted">{t("moderation.reports.none")}</Text> : null}
      {reports.value?.map((report) => {
        const where = { serverId: id, reportId: report.id };
        return (
          <ReportCard key={report.id} report={report} verdict={verdicts.get(report.id)}>
            <ReportActions
              report={report}
              runner={runner}
              resolve={(resolution) => (client) =>
                handleReport(client, where, { escalate: false, resolution })
              }
              escalate={(client) => handleReport(client, where, { escalate: true })}
              done={reports.reload}
            />
          </ReportCard>
        );
      })}
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
    </Screen>
  );
}
