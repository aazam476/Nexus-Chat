/**
 * The instance picker's logic: check the address and that a Nexus Chat
 * instance answers there.
 */
import { useState } from "react";

import type { Message } from "../auth/errors";
import { useConnection } from "../connection/ConnectionContext";
import { normalizeAddress } from "../instance/address";

/** State and action of the instance picker. */
export interface ConnectForm {
  address: string;
  setAddress: (address: string) => void;
  busy: boolean;
  error: Message | undefined;
  /** Connects; true once the instance answered. */
  submit: () => Promise<boolean>;
}

/** Runs the instance picker. */
export function useConnectForm(): ConnectForm {
  const connection = useConnection();
  const [address, setAddress] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<Message | undefined>(undefined);
  const submit = async (): Promise<boolean> => {
    const normalized = normalizeAddress(address);
    if (normalized === undefined) {
      setError({ key: "connect.invalid" });
      return false;
    }
    setBusy(true);
    setError(undefined);
    const connected = await connection.chooseInstance(normalized);
    setBusy(false);
    if (!connected) {
      setError({ key: "connect.unreachable" });
    }
    return connected;
  };
  return { address, setAddress, busy, error, submit };
}
