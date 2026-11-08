/**
 * Pressed and focused state for a control, so components can show press
 * feedback and a focus ring the same way on every platform.
 */
import { useState } from "react";

/** State and handlers to spread onto a `Pressable` or `TextInput`. */
export interface Interaction {
  pressed: boolean;
  focused: boolean;
  handlers: {
    onPressIn: () => void;
    onPressOut: () => void;
    onFocus: () => void;
    onBlur: () => void;
  };
}

/** Tracks whether a control is pressed and whether it has keyboard focus. */
export function useInteraction(): Interaction {
  const [pressed, setPressed] = useState(false);
  const [focused, setFocused] = useState(false);
  return {
    pressed,
    focused,
    handlers: {
      onPressIn: () => {
        setPressed(true);
      },
      onPressOut: () => {
        setPressed(false);
      },
      onFocus: () => {
        setFocused(true);
      },
      onBlur: () => {
        setFocused(false);
      },
    },
  };
}
