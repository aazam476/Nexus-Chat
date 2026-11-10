import type { ReactNode } from "react";
import { KeyboardAvoidingView, Platform, StyleSheet, View } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import { useTheme } from "./theme";
import { spacing } from "./tokens";

/** Props of {@link ConversationLayout}. */
export interface ConversationLayoutProps {
  /** A bar at the top: the conversation's name and actions. */
  header: ReactNode;
  /** The message list, which scrolls by itself. */
  children: ReactNode;
  /** The composer, kept above the keyboard. */
  footer: ReactNode;
}

/** The widest a conversation's column grows; longer lines are hard to read. */
export const conversationWidth = 760;

/**
 * A full-height screen for a conversation: a header, a list that fills the
 * space, and a footer that stays above the on-screen keyboard. Unlike
 * Screen it does not center its content.
 */
export function ConversationLayout({ header, children, footer }: ConversationLayoutProps): ReactNode {
  const { colors } = useTheme();
  return (
    <SafeAreaView style={[styles.fill, { backgroundColor: colors.background }]}>
      <KeyboardAvoidingView style={styles.fill} behavior={Platform.OS === "ios" ? "padding" : undefined}>
        <View role="main" style={styles.column}>
          <View style={[styles.header, { borderColor: colors.border }]}>{header}</View>
          <View style={styles.fill}>{children}</View>
          <View style={[styles.footer, { borderColor: colors.border }]}>{footer}</View>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  column: { flex: 1, width: "100%", maxWidth: conversationWidth, alignSelf: "center" },
  header: {
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  footer: { padding: spacing.md, borderTopWidth: StyleSheet.hairlineWidth },
});
