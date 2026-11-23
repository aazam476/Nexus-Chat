import type { Server } from "@nexus-chat/api-client";
import { ListRow, Stack, Text } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { isolate } from "../../i18n/bidi";
import { ServerIcon } from "./ServerIcon";

/** The user's servers, or why there are none to show. */
export function ServerList({ servers }: { servers: Server[] | undefined }): ReactNode {
  const { t } = useTranslation();
  if (servers === undefined) {
    return <Text tone="muted">{t("servers.loading")}</Text>;
  }
  if (servers.length === 0) {
    return <Text tone="muted">{t("servers.empty")}</Text>;
  }
  return (
    <Stack gap="xs">
      {servers.map((server) => (
        <ListRow
          key={server.id}
          title={isolate(server.name)}
          {...(server.description === "" ? {} : { subtitle: isolate(server.description) })}
          leading={<ServerIcon server={server} />}
          onPress={() => {
            router.push({ pathname: "/server/[id]", params: { id: server.id } });
          }}
          testID={`server-${server.id}`}
        />
      ))}
    </Stack>
  );
}
