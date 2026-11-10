import type { ReactNode } from "react";
import { StyleSheet, Text, View } from "react-native";

import { useTheme } from "./theme";
import { spacing } from "./tokens";

/** Props of {@link Badge}. */
export interface BadgeProps {
  count: number;
  /** What the count means, read by screen readers, e.g. "3 unread messages". */
  label: string;
}

/** The most a badge shows; beyond it reads "99+", so huge counts never stretch a row. */
export const badgeLimit = 99;

/**
 * An unread or mention count. It shows nothing at zero, and the number is
 * never the only signal: the label says what it counts (spec §4.12).
 */
export function Badge({ count, label }: BadgeProps): ReactNode {
  const { colors } = useTheme();
  if (count <= 0) {
    return null;
  }
  return (
    <View style={[styles.badge, { backgroundColor: colors.primary }]} accessible accessibilityLabel={label}>
      <Text style={[styles.count, { color: colors.onPrimary }]} maxFontSizeMultiplier={1.5}>
        {count > badgeLimit ? `${String(badgeLimit)}+` : String(count)}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  badge: {
    minWidth: 22,
    borderRadius: 11,
    paddingHorizontal: spacing.xs + 2,
    alignItems: "center",
    justifyContent: "center",
  },
  count: { fontSize: 12, lineHeight: 22, fontWeight: "700", fontVariant: ["tabular-nums"] },
});
