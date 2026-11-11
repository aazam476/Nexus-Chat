/**
 * Checking that the user wrote the recovery phrase down: they type back a
 * few of its words, picked at random, before the device forgets it.
 */

/** How many words the user types back. */
export const checkedWords = 3;

/** `count` distinct word positions (0-based) of a phrase of `length` words, in order. */
export function pickPositions(length: number, random: () => number = Math.random): number[] {
  const positions = new Set<number>();
  while (positions.size < Math.min(checkedWords, length)) {
    positions.add(Math.floor(random() * length));
  }
  return [...positions].sort((a, b) => a - b);
}

/** Whether `answers` are the phrase's words at `positions`, ignoring case and spaces. */
export function answersMatch(phrase: string, positions: number[], answers: string[]): boolean {
  const words = phrase.trim().split(/\s+/u);
  return positions.every(
    (position, index) => (answers[index] ?? "").trim().toLowerCase() === words[position]?.toLowerCase(),
  );
}
