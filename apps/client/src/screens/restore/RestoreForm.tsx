import { Button, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** Props of {@link RestoreForm}. */
export interface RestoreFormProps {
  /** Resolves false when the phrase does not open the backup. */
  restore: (phrase: string) => Promise<boolean>;
}

/** The recovery phrase field and its button. */
export function RestoreForm({ restore }: RestoreFormProps): ReactNode {
  const { t } = useTranslation();
  const [phrase, setPhrase] = useState("");
  const [wrong, setWrong] = useState(false);
  const [busy, setBusy] = useState(false);
  const submit = (): void => {
    setBusy(true);
    void restore(phrase).then((restored) => {
      setBusy(false);
      setWrong(!restored);
    });
  };
  return (
    <>
      <TextField
        label={t("restore.phrase")}
        hint={t("restore.phraseHint")}
        value={phrase}
        onChangeText={(text) => {
          setWrong(false);
          setPhrase(text);
        }}
        {...(wrong ? { error: t("restore.wrong") } : {})}
        autoCapitalize="none"
        autoComplete="off"
        lines={3}
        testID="restore-phrase"
      />
      <Button label={t("restore.submit")} onPress={submit} busy={busy} testID="restore-submit" />
    </>
  );
}
