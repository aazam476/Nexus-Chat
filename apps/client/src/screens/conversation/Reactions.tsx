import { spacing } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StyleSheet, View } from "react-native";

import type { TimelineItem } from "../../messaging/timeline";
import { ActionButton } from "./ActionButton";
import type { NameOf } from "./MessageItem";

/** Props of {@link Reactions}. */
export interface ReactionsProps {
  reactions: TimelineItem["reactions"];
  /** The signed-in user, `@name@instance`. */
  self: string;
  nameOf: NameOf;
  onToggle: (emoji: string, added: boolean) => void;
}

/** Each emoji with its count; pressing one adds or takes back the user's reaction. */
export function Reactions({ reactions, self, nameOf, onToggle }: ReactionsProps): ReactNode {
  const { t } = useTranslation();
  if (reactions.length === 0) {
    return null;
  }
  return (
    <View style={styles.row}>
      {reactions.map(({ emoji, users }) => {
        const mine = users.includes(self);
        return (
          <ActionButton
            key={emoji}
            label={t("conversation.reaction", { emoji, count: users.length })}
            accessibilityLabel={t("conversation.reactionLabel", {
              emoji,
              count: users.length,
              users: users.map(nameOf).join(", "),
            })}
            selected={mine}
            onPress={() => {
              onToggle(emoji, !mine);
            }}
          />
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", flexWrap: "wrap", gap: spacing.xs, paddingHorizontal: spacing.sm },
});
