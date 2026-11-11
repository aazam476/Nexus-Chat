import { Text, spacing } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { StyleSheet, View } from "react-native";

/** Props of {@link PhraseWords}. */
export interface PhraseWordsProps {
  phrase: string;
}

/**
 * The recovery phrase as numbered words, wrapping into as many columns as
 * fit, so the order is clear at any width and any text size.
 */
export function PhraseWords({ phrase }: PhraseWordsProps): ReactNode {
  const { t } = useTranslation();
  const words = phrase.split(" ").map((word, index) => ({ word, number: index + 1 }));
  return (
    <View style={styles.grid} role="list" testID="recovery-phrase">
      {words.map(({ word, number }) => (
        <View key={number} style={styles.word} role="listitem">
          <Text>{t("recovery.word", { number, word })}</Text>
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  grid: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm },
  word: { minWidth: 140, flexGrow: 1, flexBasis: "30%" },
});
