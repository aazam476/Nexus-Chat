import type { Report } from "@nexus-chat/api-client";
import { Button, Stack, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { ServerAction, ServerRunner } from "../useServerRunner";

/** Props of {@link ReportActions}. */
export interface ReportActionsProps {
  report: Report;
  runner: ServerRunner;
  resolve: (resolution: string) => ServerAction;
  /** Passing the report on, where that is possible. */
  escalate?: ServerAction;
  done: () => void;
}

/** Resolving an open report with a note, or passing it on. */
export function ReportActions({ report, runner, resolve, escalate, done }: ReportActionsProps): ReactNode {
  const { t } = useTranslation();
  const [note, setNote] = useState("");
  if (report.status !== "open") {
    return null;
  }
  return (
    <Stack gap="sm">
      <TextField
        label={t("moderation.reports.note")}
        hint={t("moderation.reports.noteHint")}
        value={note}
        onChangeText={setNote}
        testID={`report-note-${report.id}`}
      />
      <Button
        label={t("moderation.reports.resolve")}
        disabled={runner.busy}
        onPress={() => {
          runner.run(resolve(note), done);
        }}
        testID={`report-resolve-${report.id}`}
      />
      {escalate === undefined || report.escalated ? null : (
        <Button
          label={t("moderation.reports.escalate")}
          variant="secondary"
          disabled={runner.busy}
          onPress={() => {
            runner.run(escalate, done);
          }}
          testID={`report-escalate-${report.id}`}
        />
      )}
    </Stack>
  );
}
