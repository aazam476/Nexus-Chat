import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../../src/components/HttpsWarning";
import { MemberAvatar } from "../../../../src/screens/servers/admin/MemberAvatar";
import { isolate } from "../../../../src/i18n/bidi";
import { MemberRoles } from "../../../../src/screens/servers/admin/MemberRoles";
import { useServerRunner } from "../../../../src/screens/servers/useServerRunner";
import { useServer } from "../../../../src/servers/useServers";

/** One member: who they are, since when, and their roles. */
export default function Member(): ReactNode {
  const { t } = useTranslation();
  const { id, userId } = useLocalSearchParams<{ id: string; userId: string }>();
  const { server } = useServer(id);
  const runner = useServerRunner();
  const member = server?.members.find((candidate) => candidate.user.id === userId);
  return (
    <Screen>
      <HttpsWarning />
      <Link
        label={t("servers.back")}
        onPress={() => {
          router.back();
        }}
        testID="member-back"
      />
      {member === undefined ? null : (
        <Stack gap="sm">
          <MemberAvatar user={member.user} size={96} />
          <Heading>{isolate(member.user.displayName)}</Heading>
          <Text tone="muted">
            {t("servers.memberAdmin.joined", { date: new Date(member.joinedAt).toLocaleDateString() })}
          </Text>
        </Stack>
      )}
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
      {server && member ? <MemberRoles details={server.details} member={member} runner={runner} /> : null}
    </Screen>
  );
}
