import type { ReactNode } from "react";
import { View } from "react-native";

import { spacing } from "./tokens";

/** Props of {@link Stack}. */
export interface StackProps {
  children: ReactNode;
  /** Space between children. */
  gap?: keyof typeof spacing;
  /**
   * "row" lays children side by side and wraps them onto more lines when
   * they do not fit, so large text and narrow screens never clip them.
   */
  direction?: "column" | "row";
}

/** Lays its children out top to bottom (or side by side) with even spacing. */
export function Stack({ children, gap = "lg", direction = "column" }: StackProps): ReactNode {
  const row = direction === "row" ? { flexDirection: "row" as const, flexWrap: "wrap" as const } : {};
  return <View style={{ gap: spacing[gap], ...row }}>{children}</View>;
}
