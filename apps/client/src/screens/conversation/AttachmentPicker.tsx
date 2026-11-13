import { spacing, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StyleSheet, View } from "react-native";

import type { PickedFile } from "../../messaging/attachmentTransfer";
import { fileSize } from "../../messaging/fileSize";
import { pickDocument, pickMedia } from "../../platform/files";
import { compressByDefault, useMediaQuality } from "../../settings/mediaQuality";
import { ActionButton } from "./ActionButton";
import type { Composer } from "./useComposer";

/** Props of {@link AttachmentPicker}. */
export interface AttachmentPickerProps {
  composer: Composer;
  disabled: boolean;
}

/** One file waiting to be sent: removing it, and choosing a photo's quality. */
function PendingFile({
  file,
  index,
  composer,
}: {
  file: PickedFile;
  index: number;
  composer: Composer;
}): ReactNode {
  const { t } = useTranslation();
  return (
    <View style={styles.pending}>
      <ActionButton
        label={t("conversation.attachmentChip", { name: file.name, size: fileSize(file.bytes.length, t) })}
        accessibilityLabel={t("conversation.removeAttachment", { name: file.name })}
        selected
        onPress={() => {
          composer.detach(index);
        }}
        testID={`pending-${String(index)}`}
      />
      {file.compressed === undefined ? null : (
        <ActionButton
          label={t("conversation.sendOriginal")}
          selected={file.sendOriginal === true}
          onPress={() => {
            composer.setOriginal(index, file.sendOriginal !== true);
          }}
          testID={`original-${String(index)}`}
        />
      )}
      {file.mimeType.startsWith("video/") ? (
        <Text variant="small" tone="muted">
          {t("conversation.videoOriginal")}
        </Text>
      ) : null}
    </View>
  );
}

/** Buttons that add files to the next message, and the files added so far. */
export function AttachmentPicker({ composer, disabled }: AttachmentPickerProps): ReactNode {
  const { t } = useTranslation();
  const { quality } = useMediaQuality();
  if (composer.mode.kind === "edit") {
    return null;
  }
  const add = (pick: () => Promise<PickedFile | undefined>) => () => {
    if (!disabled) {
      void pick().then(async (file) => {
        if (file !== undefined) {
          composer.attach({ ...file, sendOriginal: !(await compressByDefault(quality)) });
        }
      });
    }
  };
  return (
    <View style={styles.row}>
      <ActionButton label={t("conversation.attachMedia")} onPress={add(pickMedia)} testID="attach-media" />
      <ActionButton label={t("conversation.attachFile")} onPress={add(pickDocument)} testID="attach-file" />
      {composer.files.map((file, index) => (
        <PendingFile key={`${String(index)}-${file.name}`} file={file} index={index} composer={composer} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm },
  pending: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: spacing.xs },
});
