import { listAdminReports, resolveAdminReport } from "@nexus-chat/api-client";
import { Text } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useLoaded } from "../src/hooks/useLoaded";
import { AdminPage } from "../src/screens/admin/AdminPage";
import { ReportActions } from "../src/screens/servers/moderation/ReportActions";
import { ReportCard } from "../src/screens/servers/moderation/ReportCard";
import { StatusPicker, type ReportStatus } from "../src/screens/servers/moderation/StatusPicker";
import { useVerdicts } from "../src/screens/servers/moderation/useVerdicts";
import { useServerRunner } from "../src/screens/servers/useServerRunner";
import { resultOf } from "../src/servers/serverActions";

/** The reports for the instance admins, each checked on this device. */
export default function AdminReports(): ReactNode {
  const { t } = useTranslation();
  const [status, setStatus] = useState<ReportStatus>("open");
  const runner = useServerRunner();
  const reports = useLoaded(
    async (client) => (await listAdminReports({ client, query: { status } })).data,
    status,
  );
  const verdicts = useVerdicts(reports.value);
  return (
    <AdminPage title={t("admin.sections.reports")} intro={t("admin.reports.intro")}>
      <StatusPicker value={status} onChange={setStatus} />
      {reports.value?.length === 0 ? <Text tone="muted">{t("moderation.reports.none")}</Text> : null}
      {reports.value?.map((report) => (
        <ReportCard key={report.id} report={report} verdict={verdicts.get(report.id)}>
          <ReportActions
            report={report}
            runner={runner}
            resolve={(resolution) => async (client) =>
              resultOf(
                await resolveAdminReport({
                  client,
                  path: { reportId: report.id },
                  body: { resolution: resolution.trim() },
                }),
              )
            }
            done={reports.reload}
          />
        </ReportCard>
      ))}
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
    </AdminPage>
  );
}
