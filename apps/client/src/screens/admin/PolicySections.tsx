import {
  getModerationSettings,
  getRegistrationSettings,
  updateModerationSettings,
  updateRegistrationSettings,
  type RegistrationSettings,
} from "@nexus-chat/api-client";
import { Heading, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { OptionPicker } from "../../components/OptionPicker";
import { useConnection } from "../../connection/ConnectionContext";
import { useLoaded } from "../../hooks/useLoaded";

type Mode = RegistrationSettings["mode"];

/** Who may sign up: anyone, those with an invite, or nobody. */
export function RegistrationSection(): ReactNode {
  const { t } = useTranslation();
  const { client } = useConnection();
  const settings = useLoaded(
    async (api) => (await getRegistrationSettings({ client: api })).data,
    "registration",
  );
  const mode = settings.value?.mode;
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("admin.registration.title")}</Heading>
      {mode === undefined ? null : (
        <OptionPicker
          label={t("admin.registration.who")}
          options={(["open", "invite_only", "closed"] as const).map((value) => ({
            value,
            label: t(`admin.registration.${value}`),
          }))}
          value={mode}
          onChange={(next: Mode) => {
            if (client !== undefined) {
              void updateRegistrationSettings({ client, body: { mode: next } }).then(settings.reload);
            }
          }}
          testID="registration-mode"
        />
      )}
    </Stack>
  );
}

/** Whether every server's reports also come to the admins. */
export function ReportsSettingSection(): ReactNode {
  const { t } = useTranslation();
  const { client } = useConnection();
  const settings = useLoaded(
    async (api) => (await getModerationSettings({ client: api })).data,
    "moderation",
  );
  const value = settings.value?.reportsToAdmins;
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("admin.reports.title")}</Heading>
      <Text tone="muted">{t("admin.reports.intro")}</Text>
      {value === undefined ? null : (
        <OptionPicker
          label={t("admin.reports.which")}
          options={[
            { value: false, label: t("admin.reports.passedOn") },
            { value: true, label: t("admin.reports.every") },
          ]}
          value={value}
          onChange={(reportsToAdmins) => {
            if (client !== undefined) {
              void updateModerationSettings({ client, body: { reportsToAdmins } }).then(settings.reload);
            }
          }}
          testID="reports-to-admins"
        />
      )}
    </Stack>
  );
}
