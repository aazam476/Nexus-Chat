import { radius, spacing, Text } from "@nexus-chat/ui";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Image, Pressable, StyleSheet, View } from "react-native";

import { toBase64 } from "../../messaging/engine/bytes";
import { fileSize } from "../../messaging/fileSize";
import type { AttachmentReference } from "../../messaging/payload";
import { useConversationAttachments, type ConversationAttachments } from "../../messaging/useAttachments";
import { saveReceivedFile } from "../../platform/files";
import { ActionButton } from "./ActionButton";

/** The widest an image preview is drawn, in points. */
const previewWidth = 240;

/** Downloads, decrypts and hands a file to the system; reports failure. */
function useOpen(
  attachments: ConversationAttachments | undefined,
  file: AttachmentReference,
): { open: () => void; failed: boolean } {
  const [failed, setFailed] = useState(false);
  const open = (): void => {
    setFailed(false);
    if (attachments === undefined) {
      return;
    }
    attachments
      .download(file)
      .then((bytes) => saveReceivedFile(file.name, file.mimeType, bytes))
      .catch(() => {
        setFailed(true);
      });
  };
  return { open, failed };
}

/** An image's decrypted thumbnail as a data address, once fetched. */
function useThumbnail(
  attachments: ConversationAttachments | undefined,
  file: AttachmentReference,
): string | undefined {
  const [address, setAddress] = useState<string>();
  const latestRef = useRef({ attachments, thumbnail: file.thumbnail });
  useEffect(() => {
    latestRef.current = { attachments, thumbnail: file.thumbnail };
  });
  // A message's reference never changes, so its ID is all that matters.
  const thumbnailId = file.thumbnail?.id;
  useEffect(() => {
    const { attachments: source, thumbnail } = latestRef.current;
    if (source === undefined || thumbnail === undefined || thumbnailId === undefined) {
      return;
    }
    let current = true;
    source
      .download(thumbnail)
      .then((bytes) => {
        if (current) {
          setAddress(`data:${thumbnail.mimeType};base64,${toBase64(bytes)}`);
        }
      })
      .catch(() => undefined);
    return () => {
      current = false;
    };
  }, [thumbnailId]);
  return address;
}

/** The decrypted thumbnail, pressed to open the full file. */
function ImagePreview({
  file,
  address,
  open,
}: {
  file: AttachmentReference;
  address: string;
  open: () => void;
}): ReactNode {
  const { t } = useTranslation();
  const thumbnail = file.thumbnail;
  const width = Math.min(previewWidth, thumbnail?.width ?? previewWidth);
  const height = (width * (thumbnail?.height ?? 1)) / (thumbnail?.width ?? 1);
  return (
    <Pressable
      role="button"
      accessibilityLabel={t("conversation.openAttachment", { name: file.name })}
      onPress={open}
      testID={`attachment-${file.id}`}
    >
      <Image
        source={{ uri: address }}
        accessibilityLabel={t("conversation.imageDescription", { name: file.name })}
        style={[styles.image, { width, height }]}
      />
    </Pressable>
  );
}

/** One attachment: an image preview, or a file to open. */
function AttachmentView({ file }: { file: AttachmentReference }): ReactNode {
  const { t } = useTranslation();
  const attachments = useConversationAttachments();
  const { open, failed } = useOpen(attachments, file);
  const preview = useThumbnail(attachments, file);
  return (
    <View style={styles.item}>
      {preview === undefined ? (
        <ActionButton
          label={t("conversation.attachmentChip", { name: file.name, size: fileSize(file.size, t) })}
          accessibilityLabel={t("conversation.openAttachment", { name: file.name })}
          onPress={open}
          testID={`attachment-${file.id}`}
        />
      ) : (
        <ImagePreview file={file} address={preview} open={open} />
      )}
      {failed ? (
        <Text tone="danger" live variant="small">
          {t("conversation.attachmentUnavailable")}
        </Text>
      ) : null}
    </View>
  );
}

/** A message's attachments. */
export function AttachmentList({ files }: { files: AttachmentReference[] }): ReactNode {
  if (files.length === 0) {
    return null;
  }
  return (
    <View style={styles.list}>
      {files.map((file) => (
        <AttachmentView key={file.id} file={file} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  list: { gap: spacing.sm },
  item: { gap: spacing.xs, alignItems: "flex-start" },
  image: { borderRadius: radius.card },
});
