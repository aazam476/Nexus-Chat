import { Heading, Link, Screen, Stack, Text } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../../src/components/HttpsWarning";
import { EmojiAdmin } from "../../../src/screens/servers/admin/EmojiAdmin";
import { useServerRunner } from "../../../src/screens/servers/useServerRunner";
import { useServerEmoji } from "../../../src/servers/ServerEmoji";

/** Managing the server's custom emoji: adding and removing them (MANAGE_EMOJI). */
export default function EmojiSettings(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { emoji, reload } = useServerEmoji(id);
  const runner = useServerRunner();
  return (
    <Screen>
      <HttpsWarning />
      <Stack gap="sm">
        <Link
          label={t("servers.back")}
          onPress={() => {
            router.back();
          }}
          testID="emoji-admin-back"
        />
        <Heading>{t("servers.emojiAdmin.title")}</Heading>
        <Text tone="muted">{t("servers.emojiAdmin.intro")}</Text>
      </Stack>
      {runner.error === undefined ? null : (
        <Text tone="danger" live>
          {runner.error}
        </Text>
      )}
      {emoji === undefined ? null : (
        <EmojiAdmin serverId={id} emoji={emoji} runner={runner} reload={reload} />
      )}
    </Screen>
  );
}
