import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { OptionPicker } from "../../../components/OptionPicker";

/** Which reports a list shows. */
export type ReportStatus = "open" | "resolved";

/** Choosing open or resolved reports. */
export function StatusPicker({
  value,
  onChange,
}: {
  value: ReportStatus;
  onChange: (status: ReportStatus) => void;
}): ReactNode {
  const { t } = useTranslation();
  return (
    <OptionPicker
      label={t("moderation.reports.show")}
      options={(["open", "resolved"] as const).map((status) => ({
        value: status,
        label: t(`moderation.reports.status.${status}`),
      }))}
      value={value}
      onChange={onChange}
      testID="reports-status"
    />
  );
}
