import type { CustomStatus } from "@nexus-chat/api-client";
import { Button, Heading, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { ErrorMessage } from "../../components/ErrorMessage";
import { useConnection } from "../../connection/ConnectionContext";
import { useStatusChoice, type StatusChoice } from "../useStatusChoice";
import { ExpiryPicker, type ExpiryChoice } from "./ExpiryPicker";

/** The custom status as it would be saved; `now` is when the user saves. */
function customStatusFrom(text: string, minutes: ExpiryChoice, now: Date): CustomStatus {
  return minutes === 0
    ? { text }
    : { text, expiresAt: new Date(now.getTime() + minutes * 60_000).toISOString() };
}

/** The current custom status and when it clears. */
function CurrentCustomStatus({ custom }: { custom: CustomStatus }): ReactNode {
  const { t, i18n } = useTranslation();
  const until =
    custom.expiresAt === undefined
      ? undefined
      : new Intl.DateTimeFormat(i18n.language, { dateStyle: "medium", timeStyle: "short" }).format(
          new Date(custom.expiresAt),
        );
  return (
    <Stack gap="xs">
      <Text testID="custom-status">{custom.text}</Text>
      {until !== undefined && <Text tone="muted">{t("home.customUntil", { time: until })}</Text>}
    </Stack>
  );
}

/** The new custom status and how long it lasts. */
function CustomStatusForm({ choice }: { choice: StatusChoice }): ReactNode {
  const { t } = useTranslation();
  const [text, setText] = useState("");
  const [minutes, setMinutes] = useState<ExpiryChoice>(0);
  const save = async (): Promise<void> => {
    if (await choice.setCustom(customStatusFrom(text, minutes, new Date()))) {
      setText("");
    }
  };
  return (
    <>
      <TextField
        label={t("home.customText")}
        hint={t("home.customHint")}
        value={text}
        onChangeText={setText}
        error={choice.fields["customStatus.text"]}
        testID="custom-text"
      />
      <ExpiryPicker value={minutes} onChange={setMinutes} />
      <Button
        label={t("home.customSave")}
        busy={choice.busy}
        onPress={() => void save()}
        testID="custom-save"
      />
    </>
  );
}

/** Setting, replacing and clearing the custom status (spec §4.1: up to 128 characters, optional expiry). */
export function CustomStatusEditor(): ReactNode {
  const { t } = useTranslation();
  const current = useConnection().user?.customStatus;
  const choice = useStatusChoice();
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("home.customStatus")}</Heading>
      {current === undefined ? (
        <Text tone="muted">{t("home.noCustomStatus")}</Text>
      ) : (
        <CurrentCustomStatus custom={current} />
      )}
      <CustomStatusForm choice={choice} />
      {current !== undefined && (
        <Button
          label={t("home.customClear")}
          variant="secondary"
          onPress={() => void choice.setCustom(undefined)}
          testID="custom-clear"
        />
      )}
      <ErrorMessage message={choice.error} />
    </Stack>
  );
}
