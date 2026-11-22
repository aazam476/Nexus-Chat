import type { ServerInvite } from "@nexus-chat/api-client";
import { Button, Stack, Text } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Share } from "react-native";

import { OptionPicker } from "../../components/OptionPicker";
import { useConnection } from "../../connection/ConnectionContext";
import { makeInvite, type InviteLimits } from "../../servers/inviteActions";
import { inviteLink, serverErrorText } from "../../servers/serverActions";

const hour = 3600;
const expiries = [
  { value: hour, label: "servers.invites.expiryHour" },
  { value: 24 * hour, label: "servers.invites.expiryDay" },
  { value: 7 * 24 * hour, label: "servers.invites.expiryWeek" },
  { value: undefined, label: "servers.invites.expiryNever" },
] as const;
const uses = [
  { value: undefined, label: "servers.invites.usesAny" },
  { value: 1, label: "servers.invites.usesOne" },
  { value: 10, label: "servers.invites.usesTen" },
  { value: 100, label: "servers.invites.usesHundred" },
] as const;

/** The link just made, to copy or share. */
function MadeLink({ link, serverName }: { link: string; serverName: string }): ReactNode {
  const { t } = useTranslation();
  const share = (): void => {
    // Browsers without the share sheet refuse; the link stays on screen to copy.
    Share.share({ message: t("servers.invites.shareMessage", { name: serverName, link }) }).catch(
      () => undefined,
    );
  };
  return (
    <Stack gap="sm">
      <Text variant="label">{t("servers.invites.link")}</Text>
      <Text selectable testID="invite-link">
        {link}
      </Text>
      <Button label={t("servers.invites.share")} variant="secondary" onPress={share} testID="invite-share" />
    </Stack>
  );
}

/** Props of {@link LimitPickers}. */
interface LimitPickersProps {
  limits: InviteLimits;
  setLimits: (change: (current: InviteLimits) => InviteLimits) => void;
}

/** How long the invite lasts, and how many people may use it. */
function LimitPickers({ limits, setLimits }: LimitPickersProps): ReactNode {
  const { t } = useTranslation();
  return (
    <>
      <OptionPicker
        label={t("servers.invites.expiry")}
        options={expiries.map((option) => ({ ...option, label: t(option.label) }))}
        value={limits.expiresInSeconds}
        onChange={(expiresInSeconds) => {
          setLimits((current) => ({ ...current, expiresInSeconds }));
        }}
        testID="invite-expiry"
      />
      <OptionPicker
        label={t("servers.invites.uses")}
        options={uses.map((option) => ({ ...option, label: t(option.label) }))}
        value={limits.maxUses}
        onChange={(maxUses) => {
          setLimits((current) => ({ ...current, maxUses }));
        }}
        testID="invite-uses"
      />
    </>
  );
}

/** Props of {@link NewInvite}. */
export interface NewInviteProps {
  serverId: string;
  serverName: string;
  /** Called with each invite made, so a list of invites can show it. */
  made: (invite: ServerInvite) => void;
}

/** Making an invite link: its limits, the button, and the link. */
export function NewInvite({ serverId, serverName, made }: NewInviteProps): ReactNode {
  const { t } = useTranslation();
  const { client, address } = useConnection();
  const [limits, setLimits] = useState<InviteLimits>({ expiresInSeconds: 7 * 24 * hour, maxUses: undefined });
  const [link, setLink] = useState<string>();
  const [error, setError] = useState<string>();
  const create = (): void => {
    if (client !== undefined) {
      void makeInvite(client, serverId, limits).then((result) => {
        setError(result.ok ? undefined : serverErrorText(result.code, t));
        if (result.ok) {
          setLink(inviteLink(address ?? "", result.invite.code));
          made(result.invite);
        }
      });
    }
  };
  return (
    <Stack gap="md">
      <LimitPickers limits={limits} setLimits={setLimits} />
      {error === undefined ? null : (
        <Text tone="danger" live>
          {error}
        </Text>
      )}
      <Button label={t("servers.invites.create")} onPress={create} testID="invite-create" />
      {link === undefined ? null : <MadeLink link={link} serverName={serverName} />}
    </Stack>
  );
}
