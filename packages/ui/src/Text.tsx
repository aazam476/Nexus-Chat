import type { ReactNode } from "react";
import { Text as NativeText, type TextProps as NativeTextProps } from "react-native";

import { useTheme } from "./theme";
import { type } from "./tokens";

/** Props of {@link Text}. */
export interface TextProps {
  children: ReactNode;
  /** Size and weight. */
  variant?: "body" | "small" | "label";
  /** Color role. Never the only way meaning is shown (spec §4.12). */
  tone?: "default" | "muted" | "danger";
  /** Ties the text to a field it describes, for screen readers. */
  nativeID?: string;
  /** Announce changes to screen readers, for errors that appear later. */
  live?: boolean;
  /** Passed through for tests. */
  testID?: NativeTextProps["testID"];
}

/**
 * Body text. It always follows the user's font size setting, up to and past
 * 200 %, so layouts must let it wrap (spec §4.12).
 */
export function Text({
  children,
  variant = "body",
  tone = "default",
  nativeID,
  live = false,
  testID,
}: TextProps): ReactNode {
  const { colors } = useTheme();
  const color = tone === "muted" ? colors.textMuted : tone === "danger" ? colors.danger : colors.text;
  return (
    <NativeText
      nativeID={nativeID}
      testID={testID}
      accessibilityLiveRegion={live ? "polite" : "none"}
      style={[type[variant], { color }]}
    >
      {children}
    </NativeText>
  );
}
