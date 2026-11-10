import type { ReactNode } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { useTheme } from "./theme";
import { minimumTouchTarget, radius, spacing, type } from "./tokens";
import { useInteraction } from "./useInteraction";

/** Props of {@link ListRow}. */
export interface ListRowProps {
  title: string;
  /** A second line, e.g. the last message; cut to one line. */
  subtitle?: string;
  /** Shown before the text, e.g. an avatar. */
  leading?: ReactNode;
  /** Shown after the text, e.g. a time and a badge. */
  trailing?: ReactNode;
  /** Draws the title bold, for unread conversations; never the only signal. */
  emphasized?: boolean;
  /** Without it the row is not pressable. */
  onPress?: () => void;
  /** What a screen reader says for the whole row. */
  accessibilityLabel?: string;
  testID?: string;
}

interface RowFaceProps extends Pick<
  ListRowProps,
  "title" | "subtitle" | "leading" | "trailing" | "emphasized"
> {
  pressed: boolean;
  focused: boolean;
}

/** What a row looks like, pressed or not. */
function RowFace({
  title,
  subtitle,
  leading,
  trailing,
  emphasized = false,
  pressed,
  focused,
}: RowFaceProps): ReactNode {
  const { colors } = useTheme();
  return (
    <View
      style={[
        styles.row,
        pressed && { backgroundColor: colors.surface },
        focused && { borderColor: colors.focus },
      ]}
    >
      {leading}
      <View style={styles.text}>
        <Text numberOfLines={1} style={[type.body, { color: colors.text }, emphasized && styles.emphasized]}>
          {title}
        </Text>
        {subtitle === undefined ? null : (
          <Text numberOfLines={1} style={[type.small, { color: colors.textMuted }]}>
            {subtitle}
          </Text>
        )}
      </View>
      {trailing === undefined ? null : <View style={styles.trailing}>{trailing}</View>}
    </View>
  );
}

/**
 * One row of a list. Rows are pressed many times a day, so the press shows
 * a background highlight at once, with no animation; text cuts to one line
 * rather than overflowing at large font sizes.
 */
export function ListRow({ onPress, accessibilityLabel, testID, ...face }: ListRowProps): ReactNode {
  const { pressed, focused, handlers } = useInteraction();
  if (onPress === undefined) {
    return (
      <View testID={testID}>
        <RowFace {...face} pressed={false} focused={false} />
      </View>
    );
  }
  return (
    <Pressable
      role="button"
      accessibilityLabel={accessibilityLabel ?? face.title}
      onPress={onPress}
      {...handlers}
      testID={testID}
    >
      <RowFace {...face} pressed={pressed} focused={focused} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: {
    minHeight: minimumTouchTarget + spacing.md,
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.md,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: radius.control,
    borderWidth: 2,
    borderColor: "transparent",
  },
  text: { flex: 1, minWidth: 0, gap: 2 },
  emphasized: { fontWeight: "700" },
  trailing: { alignItems: "flex-end", gap: spacing.xs },
});
