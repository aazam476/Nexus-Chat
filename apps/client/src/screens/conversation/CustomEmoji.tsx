import { useState, type ReactNode } from "react";
import { Image, Text as NativeText } from "react-native";

import { useConnection } from "../../connection/ConnectionContext";

/** How large an emoji is drawn: in a line of text, or alone ("jumbo"). */
export const emojiSizes = { inline: 22, alone: 48 } as const;

/** Props of {@link CustomEmoji}. */
export interface CustomEmojiProps {
  name: string;
  id: string;
  size?: number;
}

/**
 * A server's custom emoji (spec §4.5): its public picture, which screen
 * readers announce as `:name:`. Where the picture cannot be shown (no
 * instance yet, or the emoji was deleted), the name stands in for it.
 */
export function CustomEmoji({ name, id, size = emojiSizes.inline }: CustomEmojiProps): ReactNode {
  const { address } = useConnection();
  const [failed, setFailed] = useState(false);
  const label = `:${name}:`;
  if (address === undefined || failed) {
    return <NativeText>{label}</NativeText>;
  }
  return (
    <Image
      source={{ uri: `${address}/api/v1/emoji/${id}` }}
      accessibilityLabel={label}
      accessible
      role="img"
      resizeMode="contain"
      style={{ width: size, height: size }}
      onError={() => {
        setFailed(true);
      }}
      testID={`emoji-${id}`}
    />
  );
}
