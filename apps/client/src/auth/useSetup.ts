/**
 * The first-run setup wizard (spec §3.5): the one-time token from the
 * server's logs, the first admin account, and the instance's basics. The
 * other settings the spec lists are in the Admin panel and arrive with
 * their features.
 */
import { completeSetup, type RegistrationMode, type SetupRequest } from "@nexus-chat/api-client";
import { useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { codeOf, fieldMessages, messageFor, type FieldMessages, type Message } from "./errors";

/** What the admin typed. */
export interface SetupForm extends Record<string, string> {
  token: string;
  username: string;
  displayName: string;
  email: string;
  password: string;
  instanceName: string;
  domain: string;
  registrationMode: RegistrationMode;
}

/** State and action of the wizard. */
export interface SetupFlow {
  busy: boolean;
  done: boolean;
  error: Message | undefined;
  fields: FieldMessages;
  submit: (form: SetupForm) => Promise<void>;
}

const setupErrors = {
  invalid_setup_token: "setup.invalidToken",
  setup_already_complete: "setup.alreadyComplete",
  rate_limited: "login.rateLimited",
};

/** Runs setup against the current instance. */
export function useSetup(): SetupFlow {
  const connection = useConnection();
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<Message | undefined>(undefined);
  const [fields, setFields] = useState<FieldMessages>({});
  const submit = async (form: SetupForm): Promise<void> => {
    setBusy(true);
    setError(undefined);
    setFields({});
    try {
      const body = setupBody(form);
      const result = await completeSetup({ client: connection.client, body });
      if (result.data !== undefined) {
        setDone(true);
        await connection.refreshInstance();
      } else if (codeOf(result.error) === "validation_failed") {
        setFields(fieldMessages(result.error));
      } else {
        setError(messageFor(result.error, setupErrors));
      }
    } finally {
      setBusy(false);
    }
  };
  return { busy, done, error, fields, submit };
}

/** The request for `form`, trimmed, without an empty domain. */
export function setupBody(form: SetupForm): SetupRequest {
  const domain = form.domain.trim();
  return {
    setupToken: form.token.trim(),
    admin: {
      username: form.username.trim(),
      displayName: form.displayName.trim(),
      email: form.email.trim(),
      password: form.password,
    },
    instance: {
      name: form.instanceName.trim(),
      registrationMode: form.registrationMode,
      ...(domain === "" ? {} : { domain }),
    },
  };
}
