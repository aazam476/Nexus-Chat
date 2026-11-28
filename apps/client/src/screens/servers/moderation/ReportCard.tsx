import type { Report } from "@nexus-chat/api-client";
import { spacing, Stack, Text, useTheme } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StyleSheet, View } from "react-native";

import type { ReportVerdict } from "../../../messaging/engine/reports";
import { parseMarkdown, plainText } from "../../../messaging/markdown";
import { isolate } from "../../../i18n/bidi";

/** A verified message's words as one plain paragraph. */
function wordsOf(verdict: Extract<ReportVerdict, { verified: true }>): string {
  const payload = verdict.payload;
  if (payload.kind !== "text") {
    return "";
  }
  return parseMarkdown(payload.text)
    .map((block) => (block.type === "code" ? block.text : plainText(block.children)))
    .join(" ");
}

/** What the device found out about the report. */
function Verdict({ verdict }: { verdict: ReportVerdict | undefined }): ReactNode {
  const { t } = useTranslation();
  if (verdict === undefined) {
    return <Text tone="muted">{t("moderation.reports.checking")}</Text>;
  }
  if (!verdict.verified) {
    return (
      <Text tone="danger" testID="report-unverified">
        {t(`moderation.reports.problem.${verdict.problem}`)}
      </Text>
    );
  }
  return (
    <>
      <Text variant="small" tone="muted" testID="report-verified">
        {t("moderation.reports.verified", {
          sender: isolate(verdict.sender),
          time: new Date(verdict.sentAt).toLocaleString(),
        })}
      </Text>
      <Text>{wordsOf(verdict)}</Text>
    </>
  );
}

/** One report: the message as verified, the reporter's comment, and what can be done about it. */
export function ReportCard({
  report,
  verdict,
  children,
}: {
  report: Report;
  verdict: ReportVerdict | undefined;
  children?: ReactNode;
}): ReactNode {
  const { t } = useTranslation();
  const { colors } = useTheme();
  return (
    <View style={[styles.card, { borderColor: colors.border }]} testID={`report-${report.id}`}>
      <Stack gap="sm">
        <Verdict verdict={verdict} />
        {report.comment === "" ? null : (
          <Text>{t("moderation.reports.comment", { comment: isolate(report.comment) })}</Text>
        )}
        <Text variant="small" tone="muted">
          {t("moderation.reports.filed", { time: new Date(report.createdAt).toLocaleString() })}
        </Text>
        {report.resolution === "" ? null : (
          <Text variant="small">
            {t("moderation.reports.resolution", { resolution: isolate(report.resolution) })}
          </Text>
        )}
        {children}
      </Stack>
    </View>
  );
}

const styles = StyleSheet.create({
  card: { padding: spacing.md, borderRadius: 8, borderWidth: StyleSheet.hairlineWidth },
});
