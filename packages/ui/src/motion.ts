/**
 * Every animation value in the app, named. Components never write a
 * duration or curve inline (code standards §8.5).
 *
 * The rules behind them: frequent actions get almost no motion, entering
 * and exiting use a strong ease-out, nothing animates from nothing, and
 * reduced motion replaces movement with a gentle fade.
 */
import { cubicBezier, useReducedMotion } from "react-native-reanimated";

/** A strong ease-out: starts fast, so feedback feels immediate. */
export const easeOut = cubicBezier(0.23, 1, 0.32, 1);

/** How far a pressed control shrinks. Small, because it is pressed often. */
export const pressScale = 0.97;

/** How long press feedback takes. */
export const pressDuration = "120ms" as const;

/** With reduced motion, a pressed control dims instead of shrinking. */
export const reducedMotionPressOpacity = 0.8;

/** Whether the system asks for reduced motion. */
export function usePrefersReducedMotion(): boolean {
  return useReducedMotion();
}

/** A pressed link dims; underlined text would look wrong scaled. */
export const linkPressedOpacity = 0.6;

/**
 * How long a busy button waits before showing its spinner, in ms. Most
 * requests finish sooner, and a spinner that flashes for a frame reads as
 * a glitch. Screen readers hear "busy" at once.
 */
export const spinnerDelay = 250;

/** How long a message (an error, "Saved.") takes to fade in. */
export const appearDuration = "150ms" as const;

/** How long a new step of a form (a second factor, a confirmation) takes to fade in. */
export const stepDuration = "200ms" as const;
