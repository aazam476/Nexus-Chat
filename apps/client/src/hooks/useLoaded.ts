/** Loading something from the instance for a screen, again on request. */
import type { Client } from "@nexus-chat/api-client";
import { useCallback, useEffect, useRef, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";

/** What was loaded (undefined until then, or when `load` gave nothing), and a way to load it again. */
export interface Loaded<T> {
  value: T | undefined;
  reload: () => void;
}

/**
 * Runs `load` once the instance is connected and whenever `key` changes
 * or `reload` is called; an answer to an older load never replaces a newer one.
 */
export function useLoaded<T>(load: (client: Client) => Promise<T | undefined>, key: string): Loaded<T> {
  const { client } = useConnection();
  const [value, setValue] = useState<T>();
  const [version, setVersion] = useState(0);
  const reload = useCallback(() => {
    setVersion((count) => count + 1);
  }, []);
  // The latest `load`, which is a new function each render; `key` says when it asks for something else.
  const loadRef = useRef(load);
  useEffect(() => {
    loadRef.current = load;
  });
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    let current = true;
    void loadRef.current(client).then((loaded) => {
      if (current) {
        setValue(loaded);
      }
    });
    return () => {
      current = false;
    };
  }, [client, key, version]);
  return { value, reload };
}
