import { Button, spacing, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StyleSheet, View } from "react-native";

import { payloadLimits, type Payload } from "../../messaging/payload";
import { ActionButton } from "./ActionButton";
import type { NameOf } from "./MessageItem";
import type { Composer } from "./useComposer";

/** Props of {@link ComposerBar}. */
export interface ComposerBarProps {
  composer: Composer;
  nameOf: NameOf;
  send: (payload: Payload) => Promise<unknown>;
}

function ModeLine({ composer, nameOf }: Omit<ComposerBarProps, "send">): ReactNode {
  const { t } = useTranslation();
  const { mode } = composer;
  if (mode.kind === "new") {
    return null;
  }
  return (
    <View style={styles.mode}>
      <Text variant="small" tone="muted">
        {mode.kind === "edit"
          ? t("conversation.editing")
          : t("conversation.replyingTo", { name: nameOf(mode.item.sender) })}
      </Text>
      <ActionButton label={t("conversation.cancel")} onPress={composer.cancel} />
    </View>
  );
}

/** Sends the composer's payload, once at a time. */
function useSubmit(
  composer: Composer,
  send: ComposerBarProps["send"],
): { submit: () => void; busy: boolean; failed: boolean } {
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const submit = (): void => {
    const payload = composer.payload();
    if (payload === undefined || busy) {
      return;
    }
    setBusy(true);
    setFailed(false);
    send(payload)
      .then(composer.sent, () => {
        setFailed(true);
      })
      .finally(() => {
        setBusy(false);
      });
  };
  return { submit, busy, failed };
}

/** The message field, its send button and what the next send replies to or edits. */
export function ComposerBar({ composer, nameOf, send }: ComposerBarProps): ReactNode {
  const { t } = useTranslation();
  const { submit, busy, failed } = useSubmit(composer, send);
  const error = composer.tooLong
    ? t("conversation.tooLong", { limit: payloadLimits.textLength })
    : failed
      ? t("conversation.sendFailed")
      : undefined;
  return (
    <View style={styles.bar}>
      <ModeLine composer={composer} nameOf={nameOf} />
      <TextField
        label={t("conversation.composer")}
        hint={t("conversation.composerHint")}
        value={composer.draft}
        onChangeText={composer.setDraft}
        {...(error === undefined ? {} : { error })}
        lines={2}
        testID="composer"
      />
      <Button
        label={t("conversation.send")}
        onPress={submit}
        busy={busy}
        disabled={composer.payload() === undefined}
        testID="composer-send"
      />
    </View>
  );
}

const styles = StyleSheet.create({
  bar: { gap: spacing.sm },
  mode: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: spacing.sm },
});
