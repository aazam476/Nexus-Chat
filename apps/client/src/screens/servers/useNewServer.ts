/** The New server screen's fields, and creating the server. */
import { router } from "expo-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { makeServer, serverErrorText } from "../../servers/serverActions";

/** What the New server screen shows and does. */
export interface NewServer {
  name: string;
  setName: (name: string) => void;
  description: string;
  setDescription: (description: string) => void;
  /** Creates the server and opens it, or explains why it could not. */
  create: () => void;
  busy: boolean;
  error: string | undefined;
}

/** Keeps the New server screen's state. */
export function useNewServer(): NewServer {
  const { t } = useTranslation();
  const { client } = useConnection();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const create = (): void => {
    if (client === undefined) {
      return;
    }
    setBusy(true);
    void makeServer(client, name, description).then((result) => {
      setBusy(false);
      if (result.ok) {
        router.replace({ pathname: "/server/[id]", params: { id: result.id } });
      } else {
        setError(serverErrorText(result.code, t));
      }
    });
  };
  return { name, setName, description, setDescription, create, busy, error };
}
