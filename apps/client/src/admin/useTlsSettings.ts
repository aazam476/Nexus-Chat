/**
 * The HTTPS certificate in the Admin panel (spec §3.3): what is installed,
 * and installing or removing one. The server validates every upload, so a
 * rejected one changes nothing and its reason is shown as the server gave it.
 */
import {
  getTlsStatus,
  installTlsCertificate,
  removeTlsCertificate,
  type TlsStatus,
} from "@nexus-chat/api-client";
import { useEffect, useState } from "react";

import { messageFor, type Message } from "../auth/errors";
import { useConnection } from "../connection/ConnectionContext";
import { useChange, type Change } from "./useChange";

/** State and actions of the certificate section. */
export interface TlsSettings extends Omit<Change, "run" | "fields"> {
  status: TlsStatus | undefined;
  /** Resolves true when the server accepted the certificate. */
  install: (certificate: string, privateKey: string) => Promise<boolean>;
  remove: () => Promise<void>;
}

/** Loads the certificate status and offers the changes. */
export function useTlsSettings(): TlsSettings {
  const { client } = useConnection();
  const [status, setStatus] = useState<TlsStatus | undefined>(undefined);
  const [loadError, setLoadError] = useState<Message | undefined>(undefined);
  const change = useChange();
  useEffect(() => {
    void getTlsStatus({ client }).then((result) => {
      setStatus(result.data);
      setLoadError(result.data === undefined ? messageFor(result.error, {}) : undefined);
    });
  }, [client]);
  const install = async (certificate: string, privateKey: string): Promise<boolean> => {
    const outcome = await change.run(async () => {
      const result = await installTlsCertificate({ client, body: { certificate, privateKey } });
      setStatus((current) => result.data ?? current);
      return result.data === undefined
        ? { error: rejection(result.error) }
        : { notice: { key: "admin.tls.installed" } };
    });
    return outcome.error === undefined;
  };
  const remove = async (): Promise<void> => {
    await change.run(async () => {
      const result = await removeTlsCertificate({ client });
      if (result.error !== undefined) {
        return { error: messageFor(result.error, {}) };
      }
      setStatus({ active: false });
      return { notice: { key: "admin.tls.removed" } };
    });
  };
  return {
    status,
    busy: change.busy,
    error: change.error ?? loadError,
    notice: change.notice,
    install,
    remove,
  };
}

/** Why the server refused a certificate, in its own words. */
function rejection(error: unknown): Message {
  const message = messageFor(error, { certificate_rejected: "admin.tls.rejected" });
  if (message.key === "admin.tls.rejected") {
    return { key: message.key, values: { reason: (error as { message: string }).message } };
  }
  return message;
}
