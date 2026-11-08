/**
 * Creating an account (spec §4.1). The server checks every rule; the form
 * shows its messages next to the fields they are about.
 */
import { register, type RegisterRequest } from "@nexus-chat/api-client";
import { useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { deviceName } from "./deviceName";
import { codeOf, fieldMessages, messageFor, type FieldMessages, type Message } from "./errors";

/** What the user typed. */
export interface Registration extends Record<string, string> {
  username: string;
  displayName: string;
  email: string;
  password: string;
  inviteCode: string;
}

/** State and action of the sign-up form. */
export interface RegisterFlow {
  busy: boolean;
  error: Message | undefined;
  fields: FieldMessages;
  submit: (registration: Registration) => Promise<void>;
}

const registrationErrors = {
  registration_closed: "register.closed",
  invite_required: "register.inviteRequired",
  invalid_invite: "register.invalidInvite",
  rate_limited: "login.rateLimited",
};

/** Runs registration against the current instance. */
export function useRegister(): RegisterFlow {
  const connection = useConnection();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<Message | undefined>(undefined);
  const [fields, setFields] = useState<FieldMessages>({});
  const submit = async (registration: Registration): Promise<void> => {
    setBusy(true);
    setError(undefined);
    setFields({});
    try {
      const body = registrationBody(registration);
      const result = await register({ client: connection.client, body });
      if (result.data !== undefined) {
        await connection.signedIn(result.data.token, result.data.user);
      } else if (codeOf(result.error) === "validation_failed") {
        setFields(fieldMessages(result.error));
      } else {
        setError(messageFor(result.error, registrationErrors));
      }
    } finally {
      setBusy(false);
    }
  };
  return { busy, error, fields, submit };
}

/** The request for what the user typed, trimmed, without an empty invite code. */
export function registrationBody(registration: Registration): RegisterRequest {
  const invite = registration.inviteCode.trim();
  return {
    username: registration.username.trim(),
    displayName: registration.displayName.trim(),
    email: registration.email.trim(),
    password: registration.password,
    deviceName: deviceName(),
    ...(invite === "" ? {} : { inviteCode: invite }),
  };
}
