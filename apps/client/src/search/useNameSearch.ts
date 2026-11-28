/**
 * Searching names on the server: the user's servers, the channels they can
 * see, and the people they share a server with (spec §4.6). Names are not
 * end-to-end encrypted, so the server can search them; message content
 * never goes to it.
 */
import { searchNames, type NameSearchResults } from "@nexus-chat/api-client";
import { useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";

/** How long typing must pause before asking the server, in ms. */
const pause = 300;

/** The fewest characters the server searches for. */
const fewestNameCharacters = 2;

/**
 * What the server found for `query`: undefined while asking, or when the
 * query is too short, or when the server could not answer.
 */
export function useNameSearch(query: string): NameSearchResults | undefined {
  const { client } = useConnection();
  const wanted = query.trim();
  const [answer, setAnswer] = useState<{ query: string; found: NameSearchResults }>();
  useEffect(() => {
    if (client === undefined || !longEnough(wanted)) {
      return;
    }
    let current = true;
    const timer = setTimeout(() => {
      void searchNames({ client, query: { q: wanted } }).then((result) => {
        if (current && result.data !== undefined) {
          setAnswer({ query: wanted, found: result.data });
        }
      });
    }, pause);
    return () => {
      current = false;
      clearTimeout(timer);
    };
  }, [client, wanted]);
  return answer?.query === wanted ? answer.found : undefined;
}

/** Whether `query` is long enough to send; counted in code points, as the server counts. */
export function longEnough(query: string): boolean {
  return Array.from(query.trim()).length >= fewestNameCharacters;
}
