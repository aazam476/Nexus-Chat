import type { ServerInvite } from "@nexus-chat/api-client";
import { Button, Heading, ListRow, Stack, Text } from "@nexus-chat/ui";
import type { TFunction } from "i18next";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

/** How much of an invite is used, and when it ends. */
function detailOf(invite: ServerInvite, t: TFunction): string {
  const used =
    invite.maxUses === undefined
      ? t("servers.invites.used", { uses: invite.uses })
      : t("servers.invites.usedOf", { uses: invite.uses, max: invite.maxUses });
  const ends =
    invite.expiresAt === undefined
      ? t("servers.invites.neverExpires")
      : t("servers.invites.expires", { time: new Date(invite.expiresAt).toLocaleString() });
  return `${used} · ${ends}`;
}

/** Props of {@link InviteList}. */
export interface InviteListProps {
  invites: ServerInvite[];
  revoke: (code: string) => void;
}

/** The server's invites, each with how it is used and a way to revoke it. */
export function InviteList({ invites, revoke }: InviteListProps): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="sm">
      <Heading level={2}>{t("servers.invites.existing")}</Heading>
      {invites.length === 0 ? <Text tone="muted">{t("servers.invites.none")}</Text> : null}
      {invites.map((invite) => (
        <Stack gap="xs" key={invite.code}>
          <ListRow title={invite.code} subtitle={detailOf(invite, t)} />
          <Button
            label={t("servers.invites.revoke")}
            variant="danger"
            onPress={() => {
              revoke(invite.code);
            }}
            testID={`invite-revoke-${invite.code}`}
          />
        </Stack>
      ))}
    </Stack>
  );
}
