/**
 * Unicode isolation for text people chose (names, instance names), placed
 * inside a translated sentence. Without it, a right-to-left name ending in
 * punctuation or digits can reorder the sentence around it (Unicode
 * Bidirectional Algorithm, UAX #9, isolates FSI and PDI). The marks are
 * invisible and screen readers skip them.
 */
const firstStrongIsolate = "⁨";
const popDirectionalIsolate = "⁩";

/** `text` wrapped so its direction cannot leak into the sentence around it. */
export function isolate(text: string): string {
  return `${firstStrongIsolate}${text}${popDirectionalIsolate}`;
}
