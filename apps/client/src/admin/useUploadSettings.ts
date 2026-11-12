/** The attachment limit in the Admin panel (ADR 0014). */
import { getUploadSettings, updateUploadSettings } from "@nexus-chat/api-client";
import { useEffect, useState } from "react";

import { messageFor } from "../auth/errors";
import { useConnection } from "../connection/ConnectionContext";
import { useChange, type Change } from "./useChange";

const megabyte = 1024 * 1024;

/** State and actions of the uploads section. */
export interface UploadFlow extends Omit<Change, "run"> {
  /** The saved limit in bytes, once loaded. */
  limit: number | undefined;
  /** Saves a limit typed in MB. */
  save: (megabytes: string) => Promise<void>;
}

/** Whole MB from 1 to 4096, or undefined. */
function parseMegabytes(text: string): number | undefined {
  const value = Number(text.trim());
  return Number.isInteger(value) && value >= 1 && value <= 4096 ? value : undefined;
}

/** Loads the attachment limit and saves a new one. */
export function useUploadSettings(): UploadFlow {
  const { client } = useConnection();
  const [limit, setLimit] = useState<number>();
  const change = useChange();
  useEffect(() => {
    void getUploadSettings({ client }).then((result) => {
      setLimit(result.data?.attachmentMaxBytes);
    });
  }, [client]);
  const save = async (megabytes: string): Promise<void> => {
    await change.run(async () => {
      const parsed = parseMegabytes(megabytes);
      if (parsed === undefined) {
        return { error: { key: "admin.uploads.invalid" } };
      }
      const result = await updateUploadSettings({ client, body: { attachmentMaxBytes: parsed * megabyte } });
      if (result.data === undefined) {
        return { error: messageFor(result.error, {}) };
      }
      setLimit(result.data.attachmentMaxBytes);
      return { notice: { key: "admin.uploads.saved" } };
    });
  };
  const { busy, error, fields, notice } = change;
  return { busy, error, fields, notice, limit, save };
}
