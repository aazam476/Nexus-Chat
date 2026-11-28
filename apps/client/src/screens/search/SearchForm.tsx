import { Stack, TextField } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { OptionPicker } from "../../components/OptionPicker";

/** How far back results may go. */
export type Period = "any" | "week" | "month" | "year";

/** What the user asked for. */
export interface SearchRequest {
  query: string;
  /** A username, or `name@instance`, or empty for anyone. */
  from: string;
  attachments: boolean;
  period: Period;
  /** Whether to search only the conversation the search was opened from. */
  here: boolean;
}

/** Props of {@link SearchForm}. */
export interface SearchFormProps {
  request: SearchRequest;
  change: (request: SearchRequest) => void;
  /** Whether it was opened from a conversation or channel, which "here" means. */
  scoped: boolean;
}

type Set = (patch: Partial<SearchRequest>) => void;

/** Whether to search only where search was opened from. */
function WherePicker({ request, set }: { request: SearchRequest; set: Set }): ReactNode {
  const { t } = useTranslation();
  return (
    <OptionPicker
      label={t("search.where")}
      options={[
        { value: true, label: t("search.here") },
        { value: false, label: t("search.everywhere") },
      ]}
      value={request.here}
      onChange={(here) => {
        set({ here });
      }}
      testID="search-where"
    />
  );
}

/** Whether only messages with attachments count, and how recent they must be. */
function Filters({ request, set }: { request: SearchRequest; set: Set }): ReactNode {
  const { t } = useTranslation();
  const periods = (["any", "week", "month", "year"] as const).map((value) => ({
    value,
    label: t(`search.period.${value}`),
  }));
  return (
    <>
      <OptionPicker
        label={t("search.attachments")}
        options={[
          { value: false, label: t("search.anyMessage") },
          { value: true, label: t("search.withAttachments") },
        ]}
        value={request.attachments}
        onChange={(attachments) => {
          set({ attachments });
        }}
        testID="search-attachments"
      />
      <OptionPicker
        label={t("search.when")}
        options={periods}
        value={request.period}
        onChange={(period) => {
          set({ period });
        }}
        testID="search-when"
      />
    </>
  );
}

/** The query and its filters: from whom, with attachments, how recent, and where. */
export function SearchForm({ request, change, scoped }: SearchFormProps): ReactNode {
  const { t } = useTranslation();
  const set = (patch: Partial<SearchRequest>): void => {
    change({ ...request, ...patch });
  };
  return (
    <Stack gap="md">
      <TextField
        label={t("search.query")}
        value={request.query}
        onChangeText={(query) => {
          set({ query });
        }}
        testID="search-query"
      />
      <TextField
        label={t("search.from")}
        hint={t("search.fromHint")}
        value={request.from}
        onChangeText={(from) => {
          set({ from });
        }}
        testID="search-from"
      />
      {scoped ? <WherePicker request={request} set={set} /> : null}
      <Filters request={request} set={set} />
    </Stack>
  );
}
