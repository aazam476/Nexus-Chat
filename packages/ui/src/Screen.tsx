import type { ReactNode } from "react";
import { ScrollView, StyleSheet, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import { useTheme } from "./theme";
import { formWidth, spacing } from "./tokens";

/** Props of {@link Screen}. */
export interface ScreenProps {
  children: ReactNode;
}

/**
 * A full screen with one centered column. It scrolls, so nothing is cut off
 * at 200 % text size or on a small phone, and stays clear of notches.
 */
export function Screen({ children }: ScreenProps): ReactNode {
  const { colors } = useTheme();
  return (
    <SafeAreaView style={[styles.fill, { backgroundColor: colors.background }]}>
      <ScrollView contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
        <View role="main" style={styles.column}>
          {children}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  content: { flexGrow: 1, justifyContent: "center", padding: spacing.xl },
  column: { width: "100%", maxWidth: formWidth, alignSelf: "center", gap: spacing.xl },
});
