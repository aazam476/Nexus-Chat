/**
 * Choosing a status (spec §4.1). The new presence comes back over the
 * gateway, so every device of the user updates together.
 */
import { setStatus, type UserStatus } from "@nexus-chat/api-client";
import { useState } from "react";

import { useConnection } from "../connection/ConnectionContext";

/** State and action of the status picker. */
export interface StatusChoice {
  busy: boolean;
  choose: (status: UserStatus) => Promise<void>;
}

/** Saves the status the user picks. */
export function useStatusChoice(): StatusChoice {
  const connection = useConnection();
  const [busy, setBusy] = useState(false);
  const choose = async (status: UserStatus): Promise<void> => {
    setBusy(true);
    try {
      const result = await setStatus({ client: connection.client, body: { status } });
      if (result.data !== undefined) {
        connection.setUser(result.data);
      }
    } finally {
      setBusy(false);
    }
  };
  return { busy, choose };
}
