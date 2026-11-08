/**
 * Email settings in the Admin panel: the mail server the instance sends
 * verification, reset and warning emails through. The password is never
 * sent back by the server; leaving the field empty keeps the saved one.
 */
import {
  getSmtpSettings,
  sendTestEmail,
  updateSmtpSettings,
  type SmtpSecurity,
  type SmtpSettings,
  type UpdateSmtpSettingsRequest,
} from "@nexus-chat/api-client";
import { useEffect, useState } from "react";

import { codeOf, fieldMessages, messageFor, type Message } from "../auth/errors";
import { useConnection } from "../connection/ConnectionContext";
import { useChange, type Change } from "./useChange";

/** What the admin typed. */
export interface SmtpValues extends Record<string, string> {
  host: string;
  port: string;
  username: string;
  password: string;
  from: string;
}

/** State and actions of the email section. */
export interface SmtpFlow extends Omit<Change, "run"> {
  settings: SmtpSettings | undefined;
  save: (values: SmtpValues, security: SmtpSecurity) => Promise<void>;
  test: () => Promise<void>;
}

const testErrors = {
  smtp_not_configured: "admin.smtp.notConfigured",
  mail_delivery_failed: "admin.smtp.deliveryFailed",
  rate_limited: "admin.smtp.testRateLimited",
};

/** Loads the saved settings and offers saving and a test email. */
export function useSmtpSettings(): SmtpFlow {
  const { client } = useConnection();
  const [settings, setSettings] = useState<SmtpSettings | undefined>(undefined);
  const [loadError, setLoadError] = useState<Message | undefined>(undefined);
  const change = useChange();
  useEffect(() => {
    void getSmtpSettings({ client }).then((result) => {
      setSettings(result.data);
      setLoadError(result.data === undefined ? messageFor(result.error, {}) : undefined);
    });
  }, [client]);
  const save = async (values: SmtpValues, security: SmtpSecurity): Promise<void> => {
    await change.run(async () => {
      const result = await updateSmtpSettings({ client, body: requestBody(values, security) });
      if (result.data !== undefined) {
        setSettings(result.data);
        return { notice: { key: "admin.smtp.saved" } };
      }
      if (codeOf(result.error) === "validation_failed") {
        return { fields: fieldMessages(result.error) };
      }
      return { error: messageFor(result.error, {}) };
    });
  };
  const test = async (): Promise<void> => {
    await change.run(async () => {
      const result = await sendTestEmail({ client });
      return result.error === undefined
        ? { notice: { key: "admin.smtp.testSent" } }
        : { error: messageFor(result.error, testErrors) };
    });
  };
  const { busy, fields, notice } = change;
  return { settings, busy, error: change.error ?? loadError, fields, notice, save, test };
}

function requestBody(values: SmtpValues, security: SmtpSecurity): UpdateSmtpSettingsRequest {
  return {
    host: values.host.trim(),
    port: Number.parseInt(values.port, 10) || 0,
    security,
    username: values.username.trim(),
    from: values.from.trim(),
    // An empty field keeps the saved password, which is never shown.
    ...(values.password === "" ? {} : { password: values.password }),
  };
}
