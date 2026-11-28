import type { ServerDetails, ServerMember } from "@nexus-chat/api-client";
import { Button, Heading, Stack, Text, TextField } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { ConfirmAction } from "../../../components/ConfirmAction";
import { OptionPicker } from "../../../components/OptionPicker";
import { useConnection } from "../../../connection/ConnectionContext";
import { isolate } from "../../../i18n/bidi";
import { topPosition } from "../../../servers/hierarchy";
import { ban, kick, timeOut, type MemberRef } from "../../../servers/moderationActions";
import type { ServerRunner } from "../useServerRunner";

/** Props of {@link MemberModeration}. */
export interface MemberModerationProps {
  details: ServerDetails;
  member: ServerMember;
  runner: ServerRunner;
  /** After a kick or ban, when the member is gone. */
  onRemoved: () => void;
}

/** How long a timeout lasts, in seconds. */
const timeouts = [60, 3600, 86_400, 604_800] as const;
/** How much of a banned member's messages to delete, in seconds. */
const purges = [0, 3600, 86_400, 604_800] as const;

/** Whether the user outranks the member, as the server requires for every action here. */
function outranks(details: ServerDetails, userId: string, member: ServerMember): boolean {
  if (member.user.id === userId || member.user.id === details.server.ownerId) {
    return false;
  }
  const theirs = details.roles
    .filter((role) => member.roleIds.includes(role.id))
    .reduce((top, role) => Math.max(top, role.position), 0);
  return topPosition(details, userId) > theirs;
}

interface Acting {
  member: ServerMember;
  runner: ServerRunner;
  where: MemberRef;
}

/** When the member's timeout ends, and a button to end it now. */
function TimeoutState({ member, runner, where }: Acting): ReactNode {
  const { t } = useTranslation();
  const until = member.timedOutUntil;
  if (until === undefined) {
    return null;
  }
  return (
    <>
      <Text testID="member-timed-out">
        {t("moderation.timedOutUntil", { time: new Date(until).toLocaleString() })}
      </Text>
      <Button
        label={t("moderation.endTimeout")}
        variant="secondary"
        disabled={runner.busy}
        onPress={() => {
          runner.run((client) => timeOut(client, where, 0));
        }}
        testID="member-timeout-end"
      />
    </>
  );
}

/** Timing the member out for a while. */
function Timeout(props: Acting): ReactNode {
  const { t } = useTranslation();
  const [seconds, setSeconds] = useState<number>(timeouts[1]);
  const { runner, where } = props;
  return (
    <Stack gap="sm">
      <TimeoutState {...props} />
      <OptionPicker
        label={t("moderation.timeoutFor")}
        options={timeouts.map((value) => ({ value, label: t(`moderation.length.${String(value)}`) }))}
        value={seconds}
        onChange={setSeconds}
        testID="member-timeout-length"
      />
      <Button
        label={t("moderation.timeout")}
        variant="secondary"
        disabled={runner.busy}
        onPress={() => {
          runner.run((client) => timeOut(client, where, seconds));
        }}
        testID="member-timeout"
      />
    </Stack>
  );
}

interface Removing extends Acting {
  reason: string;
  onRemoved: () => void;
}

/** Kicking the member. */
function Kick({ member, runner, where, reason, onRemoved }: Removing): ReactNode {
  const { t } = useTranslation();
  return (
    <ConfirmAction
      label={t("moderation.kick")}
      question={t("moderation.kickConfirm", { name: isolate(member.user.displayName) })}
      confirmLabel={t("moderation.kickYes")}
      cancelLabel={t("moderation.cancel")}
      busy={runner.busy}
      onConfirm={() => {
        runner.run((client) => kick(client, where, reason), onRemoved);
      }}
      testID="member-kick"
    />
  );
}

/** Banning the member, deleting their recent messages if asked. */
function Ban({ member, runner, where, reason, onRemoved }: Removing): ReactNode {
  const { t } = useTranslation();
  const [purge, setPurge] = useState<number>(purges[0]);
  return (
    <>
      <OptionPicker
        label={t("moderation.deleteMessages")}
        options={purges.map((value) => ({ value, label: t(`moderation.purge.${String(value)}`) }))}
        value={purge}
        onChange={setPurge}
        testID="member-purge"
      />
      <ConfirmAction
        label={t("moderation.ban")}
        question={t("moderation.banConfirm", { name: isolate(member.user.displayName) })}
        confirmLabel={t("moderation.banYes")}
        cancelLabel={t("moderation.cancel")}
        busy={runner.busy}
        onConfirm={() => {
          runner.run((client) => ban(client, where, { reason, deleteSeconds: purge }), onRemoved);
        }}
        testID="member-ban"
      />
    </>
  );
}

/** Kicking or banning, with a reason for the audit log. */
function Removal(props: MemberModerationProps & { where: MemberRef }): ReactNode {
  const { t } = useTranslation();
  const [reason, setReason] = useState("");
  const may = (permission: "KICK_MEMBERS" | "BAN_MEMBERS"): boolean =>
    props.details.permissions.includes(permission);
  const removing = { ...props, reason };
  return (
    <Stack gap="sm">
      <TextField
        label={t("moderation.reason")}
        hint={t("moderation.reasonHint")}
        value={reason}
        onChangeText={setReason}
        testID="member-reason"
      />
      {may("KICK_MEMBERS") ? <Kick {...removing} /> : null}
      {may("BAN_MEMBERS") ? <Ban {...removing} /> : null}
    </Stack>
  );
}

/** What the user may do about a member: time them out, kick or ban them, as far as they may. */
export function MemberModeration(props: MemberModerationProps): ReactNode {
  const { t } = useTranslation();
  const { user } = useConnection();
  const { details, member, runner } = props;
  const permissions = details.permissions;
  const any = ["KICK_MEMBERS", "BAN_MEMBERS", "TIMEOUT_MEMBERS"].some((name) =>
    permissions.includes(name as never),
  );
  if (!any || !outranks(details, user?.id ?? "", member)) {
    return null;
  }
  const where = { serverId: details.server.id, userId: member.user.id };
  return (
    <Stack gap="md">
      <Heading level={2}>{t("moderation.title")}</Heading>
      {permissions.includes("TIMEOUT_MEMBERS") ? (
        <Timeout member={member} runner={runner} where={where} />
      ) : null}
      <Removal {...props} where={where} />
    </Stack>
  );
}
