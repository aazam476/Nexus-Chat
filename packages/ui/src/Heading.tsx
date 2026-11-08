import type { ReactNode } from "react";
import { Text as NativeText } from "react-native";

import { useTheme } from "./theme";
import { type } from "./tokens";

/** Props of {@link Heading}. */
export interface HeadingProps {
  children: ReactNode;
  /** 1 for the screen's title, 2 for sections. */
  level?: 1 | 2;
}

/** A heading, announced as one by screen readers so they can jump between sections. */
export function Heading({ children, level = 1 }: HeadingProps): ReactNode {
  const { colors } = useTheme();
  return (
    <NativeText
      role="heading"
      aria-level={level}
      style={[level === 1 ? type.heading1 : type.heading2, { color: colors.text }]}
    >
      {children}
    </NativeText>
  );
}
