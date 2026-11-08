import type { ReactNode } from "react";
import { ActivityIndicator, Pressable, StyleSheet, type ViewStyle } from "react-native";
import Animated, { type CSSTransitionProperties } from "react-native-reanimated";

import {
  easeOut,
  pressDuration,
  pressScale,
  reducedMotionPressOpacity,
  usePrefersReducedMotion,
} from "./motion";
import { useTheme } from "./theme";
import type { Colors } from "./tokens";
import { minimumTouchTarget, radius, spacing, type } from "./tokens";
import { useInteraction } from "./useInteraction";

/** Props of {@link Button}. */
export interface ButtonProps {
  /** The visible text, which is also the accessible name. */
  label: string;
  onPress: () => void;
  /** Primary for the screen's main action; one per screen. */
  variant?: "primary" | "secondary" | "danger";
  disabled?: boolean;
  /** Shows a spinner and ignores presses while an action runs. */
  busy?: boolean;
  testID?: string;
}

/**
 * A button. It shrinks slightly while pressed, which confirms the press
 * before it completes; with reduced motion it dims instead.
 */
export function Button({
  label,
  onPress,
  variant = "primary",
  disabled = false,
  busy = false,
  testID,
}: ButtonProps): ReactNode {
  const { colors } = useTheme();
  const { pressed, focused, handlers } = useInteraction();
  const palette = paletteFor(variant, colors);
  const inactive = disabled || busy;
  return (
    <Pressable
      role="button"
      accessibilityLabel={label}
      accessibilityState={{ disabled: inactive, busy }}
      disabled={inactive}
      onPress={onPress}
      {...handlers}
      pressRetentionOffset={16}
      testID={testID}
    >
      <ButtonFace
        label={label}
        busy={busy}
        palette={palette}
        pressed={pressed}
        focused={focused}
        inactive={inactive}
      />
    </Pressable>
  );
}

interface Palette {
  background: string;
  text: string;
  border: string;
}

interface ButtonFaceProps {
  label: string;
  busy: boolean;
  palette: Palette;
  pressed: boolean;
  focused: boolean;
  inactive: boolean;
}

/** What the button looks like: its box, label or spinner, and states. */
function ButtonFace({ label, busy, palette, pressed, focused, inactive }: ButtonFaceProps): ReactNode {
  const { colors } = useTheme();
  const reducedMotion = usePrefersReducedMotion();
  const borderColor = focused ? colors.focus : palette.border;
  const pressedStyle = reducedMotion ? styles.pressedReduced : styles.pressed;
  return (
    <Animated.View
      style={[
        styles.box,
        pressTransition,
        { backgroundColor: palette.background, borderColor },
        focused && styles.focused,
        inactive && styles.inactive,
        pressed && pressedStyle,
      ]}
    >
      {busy ? (
        <ActivityIndicator color={palette.text} accessibilityElementsHidden />
      ) : (
        <Animated.Text style={[type.label, { color: palette.text }]}>{label}</Animated.Text>
      )}
    </Animated.View>
  );
}

function paletteFor(
  variant: NonNullable<ButtonProps["variant"]>,
  colors: Colors,
): { background: string; text: string; border: string } {
  switch (variant) {
    case "secondary":
      return { background: colors.background, text: colors.text, border: colors.border };
    case "danger":
      return { background: colors.danger, text: colors.onDanger, border: colors.danger };
    default:
      return { background: colors.primary, text: colors.onPrimary, border: colors.primary };
  }
}

// Reanimated runs this transition on the UI thread; it is kept out of
// StyleSheet.create, whose types only know React Native's own properties.
const pressTransition: CSSTransitionProperties<ViewStyle> = {
  transitionProperty: ["transform", "opacity"],
  transitionDuration: pressDuration,
  transitionTimingFunction: easeOut,
};

const styles = StyleSheet.create({
  box: {
    minHeight: minimumTouchTarget,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.sm,
    borderRadius: radius.control,
    borderWidth: 2,
    alignItems: "center",
    justifyContent: "center",
    transform: [{ scale: 1 }],
    opacity: 1,
  },
  focused: { borderWidth: 3 },
  inactive: { opacity: 0.55 },
  pressed: { transform: [{ scale: pressScale }] },
  pressedReduced: { opacity: reducedMotionPressOpacity },
});
