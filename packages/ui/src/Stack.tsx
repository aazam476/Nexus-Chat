import type { ReactNode } from "react";
import { View } from "react-native";

import { spacing } from "./tokens";

/** Props of {@link Stack}. */
export interface StackProps {
  children: ReactNode;
  /** Space between children. */
  gap?: keyof typeof spacing;
}

/** Lays its children out top to bottom with even spacing. */
export function Stack({ children, gap = "lg" }: StackProps): ReactNode {
  return <View style={{ gap: spacing[gap] }}>{children}</View>;
}
