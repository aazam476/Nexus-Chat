import type { ServerChannel } from "@nexus-chat/api-client";
import { Heading, Link, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../components/HttpsWarning";
import { isolate } from "../../i18n/bidi";

/** The way back, the channel's name and topic, or why it cannot be shown. */
export function ChannelHeader({
  channel,
  loading,
}: {
  channel: ServerChannel | undefined;
  loading: boolean;
}): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="xs">
      <HttpsWarning />
      <Link
        label={t("servers.back")}
        onPress={() => {
          router.back();
        }}
        testID="channel-back"
      />
      {channel === undefined ? (
        <Text tone="muted">{loading ? t("servers.channelLoading") : t("servers.channelGone")}</Text>
      ) : (
        <>
          <Heading>{t("servers.textChannel", { name: isolate(channel.name) })}</Heading>
          {channel.topic === "" ? null : <Text tone="muted">{isolate(channel.topic)}</Text>}
        </>
      )}
    </Stack>
  );
}
