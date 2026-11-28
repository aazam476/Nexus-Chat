/**
 * Searching the on-device index (spec §4.6): every word of the query must
 * match the start of a word in the message, so "rep" finds "report", and
 * the filters (from someone, in conversations or channels, with
 * attachments, between dates) narrow the results. Only the chunks the
 * filters allow are read. Results come newest first.
 */
import type { LocalDatabase } from "../storage/localDatabase";
import { chunkKeys, readChunk, type Chunk, type IndexedDoc } from "./searchIndex";
import { terms } from "./tokens";

/** What to search for besides the words. */
export interface SearchFilters {
  /** `@name@instance`: only messages this user sent. */
  from?: string;
  /** Only these conversations or channels. */
  in?: readonly string[];
  /** Only messages with attachments. */
  hasAttachments?: boolean;
  /** Only messages sent at or after this time (ISO 8601). */
  after?: string;
  /** Only messages sent before this time (ISO 8601). */
  before?: string;
}

/** One message that matched. */
export interface SearchHit extends IndexedDoc {
  id: string;
}

/** The most results returned. */
export const resultLimit = 100;

/** Whether a chunk can hold matches: its conversation and month are allowed. */
function chunkAllowed(key: string, filters: SearchFilters): boolean {
  const [conversationId = "", month = ""] = key.split("|");
  if (filters.in !== undefined && !filters.in.includes(conversationId)) {
    return false;
  }
  if (filters.after !== undefined && month < filters.after.slice(0, 7)) {
    return false;
  }
  return filters.before === undefined || month <= filters.before.slice(0, 7);
}

function docAllowed(doc: IndexedDoc, filters: SearchFilters): boolean {
  return (
    (filters.from === undefined || doc.sender === filters.from) &&
    (filters.hasAttachments !== true || doc.hasAttachments) &&
    (filters.after === undefined || doc.sentAt >= filters.after) &&
    (filters.before === undefined || doc.sentAt < filters.before)
  );
}

/** The messages of a chunk with a word starting with `prefix`. */
function withPrefix(chunk: Chunk, prefix: string): Set<string> {
  const ids = new Set<string>();
  for (const [term, termIds] of chunk.terms) {
    if (term.startsWith(prefix)) {
      termIds.forEach((id) => ids.add(id));
    }
  }
  return ids;
}

/** The messages of a chunk that match every query term (all of them, without terms). */
function matching(chunk: Chunk, wanted: readonly string[]): string[] {
  let found: Set<string> | undefined;
  for (const prefix of wanted) {
    const ids = withPrefix(chunk, prefix);
    found = found === undefined ? ids : new Set([...found].filter((id) => ids.has(id)));
    if (found.size === 0) {
      return [];
    }
  }
  return [...(found ?? chunk.docs.keys())];
}

/** Searches the index; a query without words lists what the filters allow. */
export async function searchMessages(
  database: LocalDatabase,
  query: string,
  filters: SearchFilters = {},
): Promise<SearchHit[]> {
  const wanted = [...new Set(terms(query))];
  const hits: SearchHit[] = [];
  for (const key of await chunkKeys(database)) {
    if (!chunkAllowed(key, filters)) {
      continue;
    }
    const chunk = await readChunk(database, key);
    for (const id of matching(chunk, wanted)) {
      const doc = chunk.docs.get(id);
      if (doc !== undefined && docAllowed(doc, filters)) {
        hits.push({ id, ...doc });
      }
    }
  }
  hits.sort(
    (first, second) => second.sentAt.localeCompare(first.sentAt) || first.id.localeCompare(second.id),
  );
  return hits.slice(0, resultLimit);
}
