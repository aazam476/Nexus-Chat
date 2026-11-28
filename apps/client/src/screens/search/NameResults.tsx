import { openDirectMessage, type NameSearchResults } from "@nexus-chat/api-client";
import { spacing, Text, useTheme } from "@nexus-chat/ui";
import { router, type Href } from "expo-router";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Pressable, StyleSheet, View } from "react-native";

import { useConnection } from "../../connection/ConnectionContext";
import { isolate } from "../../i18n/bidi";

/** One found name; pressing it opens what it names. */
function Row({
  title,
  subtitle,
  onPress,
  testID,
}: {
  title: string;
  subtitle: string;
  onPress: () => void;
  testID: string;
}): ReactNode {
  const { colors } = useTheme();
  return (
    <Pressable
      role="link"
      onPress={onPress}
      style={({ pressed }) => [
        styles.row,
        { backgroundColor: pressed ? colors.surface : "transparent", borderColor: colors.border },
      ]}
      testID={testID}
    >
      <Text variant="label">{title}</Text>
      <Text variant="small" tone="muted">
        {subtitle}
      </Text>
    </Pressable>
  );
}

/** Where a found channel opens: a text channel itself, a voice channel's server. */
function channelHref(channel: NameSearchResults["channels"][number]): Href {
  return channel.kind === "text"
    ? {
        pathname: "/server/[id]/channel/[channelId]",
        params: { id: channel.serverId, channelId: channel.id },
      }
    : { pathname: "/server/[id]", params: { id: channel.serverId } };
}

/** Opens a direct message with someone, or says why it could not. */
function useMessagePerson(): { message: (userId: string) => void; failed: boolean } {
  const { client } = useConnection();
  const [failed, setFailed] = useState(false);
  const message = (userId: string): void => {
    setFailed(false);
    void (client && openDirectMessage({ client, body: { userId } }))?.then((result) => {
      if (result.data === undefined) {
        setFailed(true);
      } else {
        router.push({ pathname: "/conversation/[id]", params: { id: result.data.id } });
      }
    });
  };
  return { message, failed };
}

type Found = NameSearchResults;

function ServerRows({ servers }: { servers: Found["servers"] }): ReactNode {
  const { t } = useTranslation();
  return servers.map((server) => (
    <Row
      key={server.id}
      title={isolate(server.name)}
      subtitle={t("search.names.server")}
      onPress={() => {
        router.push({ pathname: "/server/[id]", params: { id: server.id } });
      }}
      testID={`search-server-${server.id}`}
    />
  ));
}

function ChannelRows({ channels }: { channels: Found["channels"] }): ReactNode {
  const { t } = useTranslation();
  return channels.map((channel) => (
    <Row
      key={channel.id}
      title={channel.kind === "text" ? `#${isolate(channel.name)}` : isolate(channel.name)}
      subtitle={t(channel.kind === "text" ? "search.names.textChannel" : "search.names.voiceChannel", {
        server: isolate(channel.serverName),
      })}
      onPress={() => {
        router.push(channelHref(channel));
      }}
      testID={`search-channel-${channel.id}`}
    />
  ));
}

function PeopleRows({ people }: { people: Found["people"] }): ReactNode {
  const { t } = useTranslation();
  const { message, failed } = useMessagePerson();
  return (
    <>
      {people.map((person) => (
        <Row
          key={person.id}
          title={isolate(person.displayName)}
          subtitle={t("search.names.person", { username: isolate(person.username) })}
          onPress={() => {
            message(person.id);
          }}
          testID={`search-person-${person.id}`}
        />
      ))}
      {failed ? (
        <Text tone="danger" live>
          {t("search.names.messageFailed")}
        </Text>
      ) : null}
    </>
  );
}

/** The servers, channels and people whose names match, from the server. */
export function NameResults({ found }: { found: Found }): ReactNode {
  const { t } = useTranslation();
  if (found.servers.length + found.channels.length + found.people.length === 0) {
    return (
      <Text tone="muted" live>
        {t("search.names.none")}
      </Text>
    );
  }
  return (
    <View style={styles.list} testID="search-names">
      <Text variant="label">{t("search.names.title")}</Text>
      <ServerRows servers={found.servers} />
      <ChannelRows channels={found.channels} />
      <PeopleRows people={found.people} />
    </View>
  );
}

const styles = StyleSheet.create({
  list: { gap: spacing.sm },
  row: { padding: spacing.sm, gap: spacing.xs, borderRadius: 8, borderWidth: StyleSheet.hairlineWidth },
});
