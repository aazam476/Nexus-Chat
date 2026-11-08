import type { ReactNode } from "react";
import Animated from "react-native-reanimated";

import { appearDuration, easeOut, stepDuration } from "./motion";

/** Props of {@link FadeIn}. */
export interface FadeInProps {
  children: ReactNode;
  /** "message" for errors and confirmations, "step" for a new part of a form. */
  kind?: "message" | "step";
  testID?: string;
}

/**
 * Content that fades in when it appears, so it does not pop into place.
 * Opacity only: nothing moves, so it stays the same with reduced motion
 * and never delays reading or pressing what it contains.
 */
export function FadeIn({ children, kind = "message", testID }: FadeInProps): ReactNode {
  return (
    <Animated.View
      testID={testID}
      style={{
        animationName: { from: { opacity: 0 }, to: { opacity: 1 } },
        animationDuration: kind === "step" ? stepDuration : appearDuration,
        animationTimingFunction: easeOut,
      }}
    >
      {children}
    </Animated.View>
  );
}
