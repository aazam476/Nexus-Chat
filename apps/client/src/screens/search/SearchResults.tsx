import { Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Pressable, StyleSheet, View } from "react-native";

import { plainText, parseMarkdown } from "../../messaging/markdown";
import type { SearchHit } from "../../search/query";
import type { Place } from "../../search/places";
import { spacing, useTheme } from "@nexus-chat/ui";

/** A message's text as one line of plain words, for a result. */
function excerpt(text: string): string {
  return parseMarkdown(text)
    .map((block) => (block.type === "code" ? block.text : plainText(block.children)))
    .join(" ")
    .replace(/\s+/g, " ")
    .slice(0, 200);
}

/** One result: where it is, who sent it and when, and its words; pressing it opens the conversation. */
function Result({
  hit,
  place,
  nameOf,
}: {
  hit: SearchHit;
  place: Place;
  nameOf: (user: string) => string;
}): ReactNode {
  const { colors } = useTheme();
  const time = new Date(hit.sentAt).toLocaleString([], { dateStyle: "medium", timeStyle: "short" });
  return (
    <Pressable
      role="link"
      onPress={() => {
        router.push(place.href);
      }}
      style={({ pressed }) => [
        styles.result,
        { backgroundColor: pressed ? colors.surface : "transparent", borderColor: colors.border },
      ]}
      testID={`search-hit-${hit.id}`}
    >
      <Text variant="small" tone="muted">{`${place.name} · ${time}`}</Text>
      <Text variant="label">{nameOf(hit.sender)}</Text>
      <Text>{excerpt(hit.text)}</Text>
    </Pressable>
  );
}

/** Props of {@link SearchResults}. */
export interface SearchResultsProps {
  hits: SearchHit[] | undefined;
  placeOf: (id: string) => Place;
  /** A sender's name, from `@name@instance`. */
  nameOf: (sender: string) => string;
  /** Whether anything was asked yet: no words and no filters shows a hint instead of everything. */
  asked: boolean;
}

/** The results, newest first, or why there are none. */
export function SearchResults({ hits, placeOf, nameOf, asked }: SearchResultsProps): ReactNode {
  const { t } = useTranslation();
  if (!asked) {
    return <Text tone="muted">{t("search.hint")}</Text>;
  }
  if (hits === undefined) {
    return <Text tone="muted">{t("search.searching")}</Text>;
  }
  if (hits.length === 0) {
    return (
      <Text tone="muted" live>
        {t("search.none")}
      </Text>
    );
  }
  return (
    <View style={styles.list}>
      <Text variant="small" tone="muted" live>
        {t("search.count", { count: hits.length })}
      </Text>
      {hits.map((hit) => (
        <Result key={hit.id} hit={hit} place={placeOf(hit.conversationId)} nameOf={nameOf} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  list: { gap: spacing.sm },
  result: { padding: spacing.sm, gap: spacing.xs, borderRadius: 8, borderWidth: StyleSheet.hairlineWidth },
});
