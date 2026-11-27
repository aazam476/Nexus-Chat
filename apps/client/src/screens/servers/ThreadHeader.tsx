import { Button, Heading, Link, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../components/HttpsWarning";
import type { ThreadFollow } from "../../servers/useThreadFollow";

/** The way back to the channel, the thread's title, and following it once it has replies. */
export function ThreadHeader({ follow }: { follow: ThreadFollow }): ReactNode {
  const { t } = useTranslation();
  return (
    <Stack gap="xs">
      <HttpsWarning />
      <Link
        label={t("threads.back")}
        onPress={() => {
          router.back();
        }}
        testID="thread-back"
      />
      <Heading>{t("threads.title")}</Heading>
      {follow.following === undefined ? null : (
        <Stack gap="xs" direction="row">
          <Text tone="muted">{follow.following ? t("threads.following") : t("threads.notFollowing")}</Text>
          <Button
            label={follow.following ? t("threads.unfollow") : t("threads.follow")}
            variant="secondary"
            onPress={follow.toggle}
            busy={follow.busy}
            testID="thread-follow"
          />
        </Stack>
      )}
    </Stack>
  );
}
