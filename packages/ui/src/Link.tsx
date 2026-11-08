import type { ReactNode } from "react";
import { Linking, Pressable, StyleSheet, type TextStyle } from "react-native";
import Animated, { type CSSTransitionProperties } from "react-native-reanimated";

import { easeOut, linkPressedOpacity, pressDuration } from "./motion";
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
 * relying on color (spec §4.12). A pressed link dims, like a pressed
 * button shrinks, so every press is confirmed.
 */
export function Link({ label, href, onPress, testID }: LinkProps): ReactNode {
  const { colors } = useTheme();
  const { pressed, focused, handlers } = useInteraction();
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
      {...handlers}
      hitSlop={8}
      testID={testID}
      style={[styles.target, focused && { borderColor: colors.focus }]}
    >
      <Animated.Text
        style={[
          type.body,
          styles.text,
          pressTransition,
          { color: colors.primary },
          pressed && styles.pressed,
        ]}
      >
        {label}
      </Animated.Text>
    </Pressable>
  );
}

// Reanimated runs this on the UI thread (see Button).
const pressTransition: CSSTransitionProperties<TextStyle> = {
  transitionProperty: "opacity",
  transitionDuration: pressDuration,
  transitionTimingFunction: easeOut,
};

const styles = StyleSheet.create({
  pressed: { opacity: linkPressedOpacity },
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
