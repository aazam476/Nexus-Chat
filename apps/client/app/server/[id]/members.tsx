import type { ServerDetails, ServerMember } from "@nexus-chat/api-client";
import { Heading, Link, ListRow, Screen, Stack } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { MemberAvatar } from "../../../src/screens/servers/admin/MemberAvatar";
import { isolate } from "../../../src/i18n/bidi";
import { useServer } from "../../../src/servers/useServers";

/** What a member's row says under their name: owner, or their roles. */
function rolesLine(details: ServerDetails, member: ServerMember, t: (key: string) => string): string {
  if (member.user.id === details.server.ownerId) {
    return t("servers.memberAdmin.owner");
  }
  const names = details.roles
    .filter((role) => member.roleIds.includes(role.id))
    .map((role) => isolate(role.name));
  return names.length === 0 ? t("servers.memberAdmin.noRoles") : names.join(", ");
}

/** The server's members; each opens their roles. */
export default function Members(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { server } = useServer(id);
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="members-back"
        />
        <Heading>{t("servers.memberAdmin.title")}</Heading>
      </Stack>
      <Stack gap="xs">
        {server?.members.map((member) => (
          <ListRow
            key={member.user.id}
            title={isolate(member.user.displayName)}
            subtitle={rolesLine(server.details, member, t)}
            leading={<MemberAvatar user={member.user} />}
            onPress={() => {
              router.push({
                pathname: "/server/[id]/member/[userId]",
                params: { id, userId: member.user.id },
              });
            }}
            testID={`member-${member.user.id}`}
          />
        ))}
      </Stack>
    </Screen>
  );
}
