import { minimumTouchTarget, spacing, type, useTheme } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { Pressable, Text, StyleSheet } from "react-native";

/** Props of {@link ActionButton}. */
export interface ActionButtonProps {
  label: string;
  /** What a screen reader says, when the label alone (an emoji) is not enough. */
  accessibilityLabel?: string;
  /** Shown as chosen, for a reaction the user made. */
  selected?: boolean;
  onPress: () => void;
  testID?: string;
}

/**
 * A small text button for actions on a message. These are pressed many
 * times a day, so a press shows at once as a background tint, with no
 * animation; the touch target stays full size around the small label.
 */
export function ActionButton({
  label,
  accessibilityLabel,
  selected = false,
  onPress,
  testID,
}: ActionButtonProps): ReactNode {
  const { colors } = useTheme();
  return (
    <Pressable
      role="button"
      accessibilityLabel={accessibilityLabel ?? label}
      accessibilityState={{ selected }}
      onPress={onPress}
      testID={testID}
      style={({ pressed }) => [
        styles.target,
        {
          borderColor: selected ? colors.primary : colors.border,
          backgroundColor: pressed ? colors.surface : "transparent",
        },
      ]}
    >
      <Text style={[type.small, { color: selected ? colors.primary : colors.text }]}>{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  target: {
    minHeight: minimumTouchTarget,
    minWidth: minimumTouchTarget,
    paddingHorizontal: spacing.sm,
    borderRadius: minimumTouchTarget / 2,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: "center",
    justifyContent: "center",
  },
});
