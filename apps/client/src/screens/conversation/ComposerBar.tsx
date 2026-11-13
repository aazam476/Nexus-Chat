import { Button, spacing, Text, TextField } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StyleSheet, View } from "react-native";

import { useConnection } from "../../connection/ConnectionContext";
import { firstLink } from "../../messaging/linkPreview";
import { payloadLimits, type Payload } from "../../messaging/payload";
import { previewSource } from "../../platform/previewSource";
import type { ConversationAttachments } from "../../messaging/useAttachments";
import type { Mentionable } from "../../messaging/mentions";
import { ActionButton } from "./ActionButton";
import { AttachmentPicker } from "./AttachmentPicker";
import { MentionSuggestions } from "./MentionSuggestions";
import type { NameOf } from "./MessageItem";
import type { Composer } from "./useComposer";
import { useSubmit } from "./useSubmit";

/** Props of {@link ComposerBar}. */
export interface ComposerBarProps {
  composer: Composer;
  nameOf: NameOf;
  send: (payload: Payload) => Promise<unknown>;
  attachments: ConversationAttachments;
  /** Who can be mentioned, and who is writing. */
  mention: { people: Mentionable[]; selfId: string };
}

function ModeLine({ composer, nameOf }: Pick<ComposerBarProps, "composer" | "nameOf">): ReactNode {
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

/** The message field, its send button and what the next send replies to or edits. */
export function ComposerBar({ composer, nameOf, send, attachments, mention }: ComposerBarProps): ReactNode {
  const { t } = useTranslation();
  const { client, instance } = useConnection();
  const previews = previewSource(client, instance?.info.linkPreviewProxy ?? false);
  const submitting = useSubmit(composer, send, attachments, previews);
  const { submit, busy } = submitting;
  const error = composer.tooLong
    ? t("conversation.tooLong", { limit: payloadLimits.textLength })
    : submitting.error;
  return (
    <View style={styles.bar}>
      <ModeLine composer={composer} nameOf={nameOf} />
      <MentionSuggestions composer={composer} people={mention.people} selfId={mention.selfId} />
      <TextField
        label={t("conversation.composer")}
        hint={t("conversation.composerHint")}
        value={composer.draft}
        onChangeText={composer.setDraft}
        {...(error === undefined ? {} : { error })}
        lines={2}
        testID="composer"
      />
      {previews?.viaInstance === true && firstLink(composer.draft) !== undefined ? (
        <Text variant="small" tone="muted" testID="preview-disclosure">
          {t("conversation.previewDisclosure")}
        </Text>
      ) : null}
      <AttachmentPicker composer={composer} disabled={busy} />
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
