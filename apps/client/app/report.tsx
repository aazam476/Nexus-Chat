import { Button, Heading, Link, Screen, Stack, Text, TextField } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../src/components/HttpsWarning";
import { useReport, type ReportState } from "../src/screens/report/useReport";

const knownProblems = new Set(["already_reported", "own_message", "message_not_found", "no_evidence"]);

/** What to say about how filing went. */
function Outcome({ state }: { state: ReportState }): ReactNode {
  const { t } = useTranslation();
  if (state.kind === "sent") {
    return (
      <Text live testID="report-sent">
        {t("report.sent")}
      </Text>
    );
  }
  if (state.kind !== "failed") {
    return null;
  }
  return (
    <Text tone="danger" live>
      {knownProblems.has(state.code) ? t(`report.errors.${state.code}`) : t("report.errors.failed")}
    </Text>
  );
}

/** The screen's title and who will see the report. */
function ReportHeader({ conversation }: { conversation: boolean }): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      <Link
        label={t("report.back")}
        onPress={() => {
          router.back();
        }}
        testID="report-back"
      />
      <Heading>{t("report.title")}</Heading>
      <Text tone="muted">{t(conversation ? "report.introConversation" : "report.introChannel")}</Text>
    </Stack>
  );
}

/**
 * Reporting a message (spec §4.7). This device hands over the message and
 * its sender's signature, so moderators can check who wrote it; nothing
 * else of the conversation goes with it.
 */
export default function Report(): ReactNode {
  const { t } = useTranslation();
  const target = useLocalSearchParams<{
    messageId: string;
    serverId?: string;
    channelId?: string;
    conversationId?: string;
  }>();
  const { state, submit } = useReport(target);
  const [comment, setComment] = useState("");
  return (
    <Screen>
      <HttpsWarning />
      <ReportHeader conversation={target.conversationId !== undefined} />
      <TextField
        label={t("report.comment")}
        hint={t("report.commentHint")}
        value={comment}
        onChangeText={setComment}
        testID="report-comment"
      />
      <Button
        label={t("report.send")}
        busy={state.kind === "sending"}
        disabled={state.kind === "sent"}
        onPress={() => {
          submit(comment);
        }}
        testID="report-send"
      />
      <Outcome state={state} />
    </Screen>
  );
}
