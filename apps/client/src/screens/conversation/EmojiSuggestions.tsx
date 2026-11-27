import { spacing } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StyleSheet, View } from "react-native";

import { emojiQuery, insertEmoji, suggestEmoji } from "../../servers/emojiTokens";
import { useOfferedEmoji } from "../../servers/ServerEmoji";
import { ActionButton } from "./ActionButton";
import { CustomEmoji } from "./CustomEmoji";
import type { Composer } from "./useComposer";

/**
 * While the draft ends in `:` and part of a name, the server's emoji it
 * could be; choosing one writes it into the draft. Shown at once, without
 * animation, since it changes with every key.
 */
export function EmojiSuggestions({ composer }: { composer: Composer }): ReactNode {
  const { t } = useTranslation();
  const offered = useOfferedEmoji();
  const query = emojiQuery(composer.draft);
  const matches = query === undefined ? [] : suggestEmoji(query, offered);
  if (matches.length === 0) {
    return null;
  }
  return (
    <View style={styles.row} testID="emoji-suggestions">
      {matches.map((emoji) => (
        <ActionButton
          key={emoji.id}
          icon={<CustomEmoji name={emoji.name} id={emoji.id} />}
          label={`:${emoji.name}:`}
          accessibilityLabel={t("conversation.emojiSuggestion", { name: `:${emoji.name}:` })}
          onPress={() => {
            composer.setDraft(insertEmoji(composer.draft, emoji));
          }}
          testID={`emoji-suggestion-${emoji.name}`}
        />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm },
});
