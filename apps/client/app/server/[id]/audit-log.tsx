import { listServerAuditLog, type AuditEntry, type ServerMember } from "@nexus-chat/api-client";
import { Button, Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { OptionPicker } from "../../../src/components/OptionPicker";
import { useConnection } from "../../../src/connection/ConnectionContext";
import { useLoaded } from "../../../src/hooks/useLoaded";
import { isolate } from "../../../src/i18n/bidi";
import { auditLine, auditReason } from "../../../src/screens/servers/moderation/auditText";
import { useServer } from "../../../src/servers/useServers";

const kinds = ["", "server", "channel", "role", "member", "invite", "emoji", "message", "report"] as const;
const page = 50;

/** One entry: the line, its reason, and when. */
function Entry({ entry, members }: { entry: AuditEntry; members: readonly ServerMember[] }): ReactNode {
  const { t } = useTranslation();
  const reason = auditReason(entry);
  return (
    <Stack gap="xs">
      <Text>{auditLine(entry, members, t)}</Text>
      {reason === undefined ? null : (
        <Text variant="small">{t("moderation.bans.reason", { reason: isolate(reason) })}</Text>
      )}
      <Text variant="small" tone="muted">
        {new Date(entry.createdAt).toLocaleString()}
      </Text>
    </Stack>
  );
}

/** The entries of one kind, newest first, a page at a time. */
function useEntries(
  serverId: string,
  kind: string,
): { entries: AuditEntry[] | undefined; more: (() => void) | undefined } {
  const { client } = useConnection();
  const [older, setOlder] = useState<{ kind: string; entries: AuditEntry[] }>({ kind, entries: [] });
  const first = useLoaded(async (api) => {
    const query = kind === "" ? { limit: page } : { limit: page, action: kind };
    return (await listServerAuditLog({ client: api, path: { serverId }, query })).data;
  }, `${serverId}|${kind}`);
  const extra = older.kind === kind ? older.entries : [];
  const entries = first.value === undefined ? undefined : [...first.value, ...extra];
  const last = entries?.at(-1);
  const full = entries !== undefined && entries.length % page === 0 && entries.length > 0;
  const more =
    client === undefined || last === undefined || !full
      ? undefined
      : () => {
          const query =
            kind === "" ? { limit: page, before: last.id } : { limit: page, before: last.id, action: kind };
          void listServerAuditLog({ client, path: { serverId }, query }).then((answer) => {
            setOlder({ kind, entries: [...extra, ...(answer.data ?? [])] });
          });
        };
  return { entries, more };
}

/** The screen's title and what it shows. */
function AuditHeader(): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      <Link
        label={t("servers.back")}
        onPress={() => {
          router.back();
        }}
        testID="audit-back"
      />
      <Heading>{t("moderation.audit.title")}</Heading>
      <Text tone="muted">{t("moderation.audit.intro")}</Text>
    </Stack>
  );
}

/** The server's audit log (VIEW_AUDIT_LOG): every moderation and configuration change of the last 90 days. */
export default function AuditLog(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { server } = useServer(id);
  const [kind, setKind] = useState<string>("");
  const { entries, more } = useEntries(id, kind);
  const members = server?.members ?? [];
  return (
    <Screen>
      <HttpsWarning />
      <AuditHeader />
      <OptionPicker
        label={t("moderation.audit.show")}
        options={kinds.map((value) => ({
          value,
          label: t(`moderation.audit.kinds.${value === "" ? "all" : value}`),
        }))}
        value={kind}
        onChange={setKind}
        testID="audit-kind"
      />
      {entries === undefined ? <Text tone="muted">{t("common.loading")}</Text> : null}
      {entries?.length === 0 ? <Text tone="muted">{t("moderation.audit.none")}</Text> : null}
      {entries?.map((entry) => (
        <Entry key={entry.id} entry={entry} members={members} />
      ))}
      {more === undefined ? null : (
        <Button label={t("moderation.audit.older")} variant="secondary" onPress={more} testID="audit-older" />
      )}
    </Screen>
  );
}
