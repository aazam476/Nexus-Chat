/**
 * Searching as the user types: the query runs a moment after they stop
 * typing, so each key does not search the whole index again, and an
 * answer to an older query never replaces a newer one's.
 */
import { useEffect, useState } from "react";

import { messengerOf, useMessaging } from "../messaging/MessagingContext";
import type { SearchFilters, SearchHit } from "./query";

/** How long typing must pause before a search runs, in ms. */
const pause = 250;

/**
 * The hits for `query` and `filters`; undefined while searching or before
 * encryption is ready. A change of `revision` (the index grew) searches again.
 */
export function useSearch(query: string, filters: SearchFilters, revision = ""): SearchHit[] | undefined {
  const messenger = messengerOf(useMessaging());
  const key = JSON.stringify([query, filters, revision]);
  // The hits with the request they answer; a newer request shows none until its own come.
  const [answer, setAnswer] = useState<{ key: string; hits: SearchHit[] }>();
  useEffect(() => {
    if (messenger === undefined) {
      return;
    }
    let current = true;
    const [asked, wanted] = JSON.parse(key) as [string, SearchFilters, string];
    const timer = setTimeout(() => {
      void messenger.search(asked, wanted).then((hits) => {
        if (current) {
          setAnswer({ key, hits });
        }
      });
    }, pause);
    return () => {
      current = false;
      clearTimeout(timer);
    };
  }, [messenger, key]);
  return answer?.key === key ? answer.hits : undefined;
}
