import type { ReactNode } from "react";
import { StyleSheet, View } from "react-native";

import { Text } from "./Text";
import { useTheme } from "./theme";
import { radius, spacing } from "./tokens";

/** Props of {@link Banner}. */
export interface BannerProps {
  /** Starts with the kind of message ("Warning: …"), so the meaning never depends on color. */
  title: string;
  children?: ReactNode;
  testID?: string;
}

/**
 * A persistent warning, such as the HTTPS warning (spec §3.3). It appears
 * with the page rather than animating in: it is important, and motion would
 * only delay it.
 */
export function Banner({ title, children, testID }: BannerProps): ReactNode {
  const { colors } = useTheme();
  return (
    <View
      role="region"
      aria-label={title}
      testID={testID}
      style={[styles.banner, { backgroundColor: colors.warningSurface, borderColor: colors.warningBorder }]}
    >
      <Text variant="label">{title}</Text>
      <View style={styles.body}>{children}</View>
    </View>
  );
}

const styles = StyleSheet.create({
  banner: { borderWidth: 2, borderRadius: radius.card, padding: spacing.lg, gap: spacing.sm },
  body: { gap: spacing.xs },
});
