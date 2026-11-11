import { Button, Heading, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { answersMatch, pickPositions } from "../../messaging/phraseCheck";

/** Props of {@link PhraseCheck}. */
export interface PhraseCheckProps {
  phrase: string;
  onConfirmed: () => Promise<void>;
  onShowAgain: () => void;
}

function WordFields({
  positions,
  answers,
  onChange,
}: {
  positions: number[];
  answers: string[];
  onChange: (index: number, text: string) => void;
}): ReactNode {
  const { t } = useTranslation();
  return positions.map((position, index) => (
    <TextField
      key={position}
      label={t("recovery.checkWord", { number: position + 1 })}
      value={answers[index] ?? ""}
      onChangeText={(text) => {
        onChange(index, text);
      }}
      autoCapitalize="none"
      autoComplete="off"
      testID={`recovery-word-${String(index)}`}
    />
  ));
}

/** Asks for a few words of the phrase before the device forgets it. */
export function PhraseCheck({ phrase, onConfirmed, onShowAgain }: PhraseCheckProps): ReactNode {
  const { t } = useTranslation();
  const [positions] = useState(() => pickPositions(phrase.split(" ").length));
  const [answers, setAnswers] = useState(() => positions.map(() => ""));
  const [mismatch, setMismatch] = useState(false);
  const [busy, setBusy] = useState(false);
  const confirm = (): void => {
    if (!answersMatch(phrase, positions, answers)) {
      setMismatch(true);
      return;
    }
    setBusy(true);
    void onConfirmed().finally(() => {
      setBusy(false);
    });
  };
  return (
    <Stack gap="md">
      <Heading>{t("recovery.checkTitle")}</Heading>
      <Text tone="muted">{t("recovery.checkIntro")}</Text>
      <WordFields
        positions={positions}
        answers={answers}
        onChange={(index, text) => {
          setMismatch(false);
          setAnswers((current) => current.map((answer, at) => (at === index ? text : answer)));
        }}
      />
      {mismatch ? (
        <Text tone="danger" live>
          {t("recovery.mismatch")}
        </Text>
      ) : null}
      <Button label={t("recovery.confirm")} onPress={confirm} busy={busy} testID="recovery-confirm" />
      <Button label={t("recovery.showAgain")} variant="secondary" onPress={onShowAgain} />
    </Stack>
  );
}
