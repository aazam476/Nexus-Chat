/** A moderator's device checking reports (ADR 0010): nothing about one is shown as true until it checks out. */
import type { Report } from "@nexus-chat/api-client";
import { useEffect, useState } from "react";

import { fromBase64 } from "../../../messaging/engine/bytes";
import type { ReportVerdict } from "../../../messaging/engine/reports";
import { messengerOf, useMessaging } from "../../../messaging/MessagingContext";

/** Each report's verdict by its ID, as they come in. */
export function useVerdicts(reports: readonly Report[] | undefined): ReadonlyMap<string, ReportVerdict> {
  const messenger = messengerOf(useMessaging());
  const [verdicts, setVerdicts] = useState<ReadonlyMap<string, ReportVerdict>>(() => new Map());
  useEffect(() => {
    if (messenger === undefined || reports === undefined) {
      return;
    }
    let current = true;
    void Promise.all(
      reports.map(async (report) => {
        const verdict = await messenger.checkReport({
          messageId: report.messageId,
          evidence: fromBase64(report.evidence),
          receipt: fromBase64(report.receipt),
          receiptSignature: fromBase64(report.receiptSignature),
        });
        return [report.id, verdict] as const;
      }),
    ).then((checked) => {
      if (current) {
        setVerdicts(new Map(checked));
      }
    });
    return () => {
      current = false;
    };
  }, [messenger, reports]);
  return verdicts;
}
