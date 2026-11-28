/**
 * How text becomes search terms, the same way for messages and queries
 * (spec §4.6): case and accents are folded away (so "Café" finds "cafe"),
 * words are runs of letters and digits in any script, and scripts written
 * without spaces (Chinese, Japanese, Korean) are split into overlapping
 * pairs of characters, so a word inside a sentence can still be found.
 */

/** Words: letters (with their combining marks) and digits. */
const wordPattern = /[\p{L}\p{M}\p{N}]+/gu;
/** Characters of scripts that do not separate words with spaces. */
const unspaced = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/u;
/** The longest term kept; longer runs (hashes, long links) are cut. */
const maximumTermLength = 40;

/** Lowercase, with accents and other combining marks removed. */
export function fold(text: string): string {
  return text.normalize("NFKD").replace(/\p{M}/gu, "").toLowerCase();
}

/** Splits a run of unspaced characters into overlapping pairs; a single character stays whole. */
function pairs(run: string): string[] {
  const characters = Array.from(run);
  if (characters.length < 2) {
    return characters;
  }
  return characters.slice(0, -1).map((character, index) => character + (characters[index + 1] ?? ""));
}

/** Splits one word into terms: itself, or pairs where it is written without spaces. */
function termsOfWord(word: string): string[] {
  if (!unspaced.test(word)) {
    return [Array.from(word).slice(0, maximumTermLength).join("")];
  }
  const terms: string[] = [];
  let run = "";
  let other = "";
  for (const character of word) {
    if (unspaced.test(character)) {
      if (other !== "") {
        terms.push(other);
        other = "";
      }
      run += character;
    } else {
      terms.push(...pairs(run));
      run = "";
      other += character;
    }
  }
  return [...terms, ...pairs(run), ...(other === "" ? [] : [other])];
}

/** The search terms of `text`, in order, repeats kept. */
export function terms(text: string): string[] {
  return (fold(text).match(wordPattern) ?? []).flatMap(termsOfWord);
}
