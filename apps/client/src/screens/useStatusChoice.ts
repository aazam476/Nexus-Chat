/**
 * Choosing a status and a custom status (spec §4.1). The server replaces
 * both together, so every change sends both: picking "Idle" keeps the
 * custom status, and editing the custom status keeps the status. The new
 * presence comes back over the gateway, so every device updates together.
 */
import { setStatus, type CustomStatus, type UserStatus } from "@nexus-chat/api-client";
import { useState } from "react";

import { codeOf, fieldMessages, messageFor, type FieldMessages, type Message } from "../auth/errors";
import { useConnection } from "../connection/ConnectionContext";

/** State and actions of the status controls. */
export interface StatusChoice {
  busy: boolean;
  error: Message | undefined;
  fields: FieldMessages;
  choose: (status: UserStatus) => Promise<void>;
  /** Sets the custom status, or clears it when `custom` is undefined. */
  setCustom: (custom: CustomStatus | undefined) => Promise<boolean>;
}

/** Saves the status the user picks. */
export function useStatusChoice(): StatusChoice {
  const connection = useConnection();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<Message | undefined>(undefined);
  const [fields, setFields] = useState<FieldMessages>({});
  const save = async (status: UserStatus, customStatus: CustomStatus | undefined): Promise<boolean> => {
    setBusy(true);
    setError(undefined);
    setFields({});
    try {
      const body = customStatus === undefined ? { status } : { status, customStatus };
      const result = await setStatus({ client: connection.client, body });
      if (result.data !== undefined) {
        connection.setUser(result.data);
        return true;
      }
      if (codeOf(result.error) === "validation_failed") {
        setFields(fieldMessages(result.error));
      } else {
        setError(messageFor(result.error, {}));
      }
      return false;
    } finally {
      setBusy(false);
    }
  };
  const user = connection.user;
  return {
    busy,
    error,
    fields,
    choose: async (status) => {
      await save(status, user?.customStatus);
    },
    setCustom: (custom) => save(user?.status ?? "online", custom),
  };
}
