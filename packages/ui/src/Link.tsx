import type { ReactNode } from "react";
import { Linking, Pressable, StyleSheet, Text as NativeText } from "react-native";

import { useTheme } from "./theme";
import { minimumTouchTarget, type } from "./tokens";
import { useInteraction } from "./useInteraction";

/** Props of {@link Link}. */
export interface LinkProps {
  label: string;
  /** An address to open, or use `onPress` to navigate inside the app. */
  href?: string;
  onPress?: () => void;
  testID?: string;
}

/**
 * Text that goes somewhere. Links are underlined, so they stand out without
 * relying on color (spec §4.12).
 */
export function Link({ label, href, onPress, testID }: LinkProps): ReactNode {
  const { colors } = useTheme();
  const { focused, handlers } = useInteraction();
  const open = (): void => {
    if (onPress !== undefined) {
      onPress();
    } else if (href !== undefined) {
      void Linking.openURL(href);
    }
  };
  return (
    <Pressable
      role="link"
      accessibilityLabel={label}
      onPress={open}
      onFocus={handlers.onFocus}
      onBlur={handlers.onBlur}
      hitSlop={8}
      testID={testID}
      style={[styles.target, focused && { borderColor: colors.focus }]}
    >
      <NativeText style={[type.body, styles.text, { color: colors.primary }]}>{label}</NativeText>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  // A transparent border that turns into the focus ring, so focusing never
  // shifts the layout.
  target: {
    minHeight: minimumTouchTarget,
    justifyContent: "center",
    borderWidth: 2,
    borderColor: "transparent",
    borderRadius: 4,
  },
  text: { textDecorationLine: "underline" },
});
