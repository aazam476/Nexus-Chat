import { spacing } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StyleSheet, View } from "react-native";

import { isolate } from "../../i18n/bidi";
import { insertMention, mentionQuery, suggestMentions, type Mentionable } from "../../messaging/mentions";
import { ActionButton } from "./ActionButton";
import type { Composer } from "./useComposer";

/** Props of {@link MentionSuggestions}. */
export interface MentionSuggestionsProps {
  composer: Composer;
  people: Mentionable[];
  selfId: string;
}

/**
 * While the draft ends in `@` and part of a name, the members it could be;
 * choosing one writes the mention into the draft. Shown at once, without
 * animation, since it changes with every key.
 */
export function MentionSuggestions({ composer, people, selfId }: MentionSuggestionsProps): ReactNode {
  const { t } = useTranslation();
  const query = mentionQuery(composer.draft);
  const matches = query === undefined ? [] : suggestMentions(query, people, selfId);
  if (matches.length === 0) {
    return null;
  }
  return (
    <View style={styles.row} testID="mention-suggestions">
      {matches.map((person) => (
        <ActionButton
          key={person.id}
          label={`@${isolate(person.displayName)}`}
          accessibilityLabel={t("conversation.mentionSuggestion", {
            name: `@${isolate(person.displayName)}`,
          })}
          onPress={() => {
            composer.setDraft(insertMention(composer.draft, person));
          }}
          testID={`mention-${person.username}`}
        />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm },
});
