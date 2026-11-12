/**
 * The link preview a message carries: the title, description and image
 * the sender's device fetched. Nothing is fetched from the link here; the
 * image comes encrypted, like any attachment. A preview whose link is not
 * in the message's text is not shown, so it cannot disguise where it goes.
 */
import { radius, spacing, Text, useTheme } from "@nexus-chat/ui";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Image, Linking, Pressable, StyleSheet } from "react-native";

import { toBase64 } from "../../messaging/engine/bytes";
import { linksOf } from "../../messaging/linkPreview";
import type { LinkPreview } from "../../messaging/payload";
import { useConversationAttachments } from "../../messaging/useAttachments";

const imageHeight = 120;

/** Keeps what a sender wrote short enough to show. */
function shorten(text: string, limit: number): string {
  return text.length <= limit ? text : `${text.slice(0, limit - 1)}…`;
}

/** The preview's decrypted image as a data address, once fetched. */
function usePreviewImage(preview: LinkPreview): string | undefined {
  const attachments = useConversationAttachments();
  const [address, setAddress] = useState<string>();
  const latestRef = useRef({ attachments, image: preview.image });
  useEffect(() => {
    latestRef.current = { attachments, image: preview.image };
  });
  // A message's preview never changes, so its image's ID is all that matters.
  const imageId = preview.image?.id;
  useEffect(() => {
    const { attachments: source, image } = latestRef.current;
    if (source === undefined || image === undefined || imageId === undefined) {
      return;
    }
    let current = true;
    source
      .download(image)
      .then((bytes) => {
        if (current) {
          setAddress(`data:${image.mimeType};base64,${toBase64(bytes)}`);
        }
      })
      .catch(() => undefined);
    return () => {
      current = false;
    };
  }, [imageId]);
  return address;
}

/** A message's link preview, when the link is in its text. */
export function PreviewCard({
  preview,
  text,
}: {
  preview: LinkPreview | undefined;
  text: string;
}): ReactNode {
  if (preview === undefined || !linksOf(text).includes(preview.url)) {
    return null;
  }
  return <Card preview={preview} />;
}

function Card({ preview }: { preview: LinkPreview }): ReactNode {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const image = usePreviewImage(preview);
  const host = URL.canParse(preview.url) ? new URL(preview.url).host : preview.url;
  const title = shorten(preview.title ?? host, 120);
  return (
    <Pressable
      role="link"
      accessibilityLabel={t("conversation.previewOpen", { title })}
      onPress={() => void Linking.openURL(preview.url)}
      style={[styles.card, { borderColor: colors.border }]}
      testID="link-preview"
    >
      {image === undefined ? null : <Image source={{ uri: image }} accessible={false} style={styles.image} />}
      <Text variant="small" tone="muted">
        {host}
      </Text>
      <Text variant="label">{title}</Text>
      {preview.description === undefined ? null : (
        <Text variant="small">{shorten(preview.description, 300)}</Text>
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: {
    maxWidth: 360,
    alignSelf: "flex-start",
    gap: spacing.xs,
    padding: spacing.sm,
    borderRadius: radius.card,
    borderWidth: StyleSheet.hairlineWidth,
  },
  image: { height: imageHeight, borderRadius: radius.card, resizeMode: "cover" },
});
