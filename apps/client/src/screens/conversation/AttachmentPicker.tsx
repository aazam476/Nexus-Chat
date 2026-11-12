import { spacing } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StyleSheet, View } from "react-native";

import type { PickedFile } from "../../messaging/attachmentTransfer";
import { fileSize } from "../../messaging/fileSize";
import { pickDocument, pickMedia } from "../../platform/files";
import { ActionButton } from "./ActionButton";
import type { Composer } from "./useComposer";

/** Props of {@link AttachmentPicker}. */
export interface AttachmentPickerProps {
  composer: Composer;
  disabled: boolean;
}

/** Buttons that add files to the next message, and the files added so far. */
export function AttachmentPicker({ composer, disabled }: AttachmentPickerProps): ReactNode {
  const { t } = useTranslation();
  if (composer.mode.kind === "edit") {
    return null;
  }
  const add = (pick: () => Promise<PickedFile | undefined>) => () => {
    if (!disabled) {
      void pick().then((file) => {
        if (file !== undefined) {
          composer.attach(file);
        }
      });
    }
  };
  return (
    <View style={styles.row}>
      <ActionButton label={t("conversation.attachMedia")} onPress={add(pickMedia)} testID="attach-media" />
      <ActionButton label={t("conversation.attachFile")} onPress={add(pickDocument)} testID="attach-file" />
      {composer.files.map((file, index) => (
        <ActionButton
          key={`${String(index)}-${file.name}`}
          label={t("conversation.attachmentChip", { name: file.name, size: fileSize(file.bytes.length, t) })}
          accessibilityLabel={t("conversation.removeAttachment", { name: file.name })}
          selected
          onPress={() => {
            composer.detach(index);
          }}
          testID={`pending-${String(index)}`}
        />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm },
});
