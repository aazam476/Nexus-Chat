import {
  getFederationSettings,
  updateFederationSettings,
  type FederationPolicy,
} from "@nexus-chat/api-client";
import { Button, Heading, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { ErrorMessage } from "../../components/ErrorMessage";
import { NoticeMessage } from "../../components/NoticeMessage";
import { OptionPicker } from "../../components/OptionPicker";
import { useConnection } from "../../connection/ConnectionContext";
import { useLoaded } from "../../hooks/useLoaded";

type Mode = FederationPolicy["mode"];

/** Domains as one per line, and back. */
const lines = (domains: readonly string[]): string => domains.join("\n");
const domainsOf = (text: string): string[] => text.split(/[\s,]+/).filter((domain) => domain !== "");

/** A list of domains, one per line. */
function DomainList(props: {
  label: string;
  value: string;
  onChange: (text: string) => void;
  testID: string;
}): ReactNode {
  const { t } = useTranslation();
  return (
    <TextField
      label={props.label}
      hint={t("admin.federation.listHint")}
      value={props.value}
      onChangeText={props.onChange}
      lines={4}
      testID={props.testID}
    />
  );
}

/** Saving the policy, and how that went. */
function useSave(saved: () => void): {
  outcome: "saved" | "invalid" | undefined;
  save: (policy: FederationPolicy) => void;
} {
  const { client } = useConnection();
  const [outcome, setOutcome] = useState<"saved" | "invalid">();
  const save = (policy: FederationPolicy): void => {
    if (client !== undefined) {
      void updateFederationSettings({ client, body: policy }).then((answer) => {
        setOutcome(answer.response?.ok === true ? "saved" : "invalid");
        saved();
      });
    }
  };
  return { outcome, save };
}

/** Whom to talk to. */
function ModePicker({ mode, onChange }: { mode: Mode; onChange: (mode: Mode) => void }): ReactNode {
  const { t } = useTranslation();
  return (
    <OptionPicker
      label={t("admin.federation.mode")}
      options={(["open", "allowlist", "closed"] as const).map((value) => ({
        value,
        label: t(`admin.federation.${value}`),
      }))}
      value={mode}
      onChange={onChange}
      testID="federation-mode"
    />
  );
}

/** The form, once the current policy is known. */
function FederationForm({ policy, saved }: { policy: FederationPolicy; saved: () => void }): ReactNode {
  const { t } = useTranslation();
  const [mode, setMode] = useState<Mode>(policy.mode);
  const [allowed, setAllowed] = useState(() => lines(policy.allowed));
  const [blocked, setBlocked] = useState(() => lines(policy.blocked));
  const { outcome, save } = useSave(saved);
  return (
    <Stack gap="sm">
      <ModePicker mode={mode} onChange={setMode} />
      {mode === "allowlist" ? (
        <DomainList
          label={t("admin.federation.allowed")}
          value={allowed}
          onChange={setAllowed}
          testID="federation-allowed"
        />
      ) : null}
      <DomainList
        label={t("admin.federation.blocked")}
        value={blocked}
        onChange={setBlocked}
        testID="federation-blocked"
      />
      <Button
        label={t("admin.federation.save")}
        variant="secondary"
        onPress={() => {
          save({ mode, allowed: domainsOf(allowed), blocked: domainsOf(blocked) });
        }}
        testID="federation-save"
      />
      <ErrorMessage message={outcome === "invalid" ? { key: "admin.federation.invalid" } : undefined} />
      <NoticeMessage
        message={outcome === "saved" ? { key: "admin.federation.saved" } : undefined}
        testID="federation-notice"
      />
    </Stack>
  );
}

/** Which instances this one will talk to, once federation arrives. */
export function FederationSection(): ReactNode {
  const { t } = useTranslation();
  const policy = useLoaded(async (client) => (await getFederationSettings({ client })).data, "federation");
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("admin.federation.title")}</Heading>
      <Text tone="muted">{t("admin.federation.intro")}</Text>
      {policy.value === undefined ? null : (
        <FederationForm key={JSON.stringify(policy.value)} policy={policy.value} saved={policy.reload} />
      )}
    </Stack>
  );
}
