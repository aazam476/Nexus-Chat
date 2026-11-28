import { Button, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import type { BackfillState } from "../../search/useBackfill";

/** A note about the backfill, with a button. */
function WithButton({
  note,
  label,
  onPress,
  testID,
}: {
  note: string;
  label: string;
  onPress: () => void;
  testID: string;
}): ReactNode {
  return (
    <Stack gap="xs">
      <Text tone="muted" live>
        {note}
      </Text>
      <Button label={label} variant="secondary" onPress={onPress} testID={testID} />
    </Stack>
  );
}

/**
 * Below results searched in one place: the recent window it covers, and
 * searching older messages there too, with how far that got.
 */
export function OlderMessages({
  state,
  start,
}: {
  state: BackfillState;
  start: (now: boolean) => void;
}): ReactNode {
  const { t } = useTranslation();
  if (state.kind === "running" || state.kind === "done") {
    return (
      <Text tone="muted" live>
        {state.kind === "running"
          ? t("search.older.running")
          : t("search.older.done", { count: state.indexed })}
      </Text>
    );
  }
  const waiting = state.kind === "waiting";
  const notes = {
    waiting: "search.older.waiting",
    failed: "search.older.failed",
    idle: "search.older.intro",
  } as const;
  return (
    <WithButton
      note={t(notes[state.kind], waiting ? { count: state.indexed } : {})}
      label={waiting ? t("search.older.now") : t("search.older.start")}
      onPress={() => {
        start(waiting);
      }}
      testID={waiting ? "search-older-now" : "search-older"}
    />
  );
}
