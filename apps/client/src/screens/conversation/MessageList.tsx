import { spacing, Text } from "@nexus-chat/ui";
import { useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { FlatList, StyleSheet, View } from "react-native";

import type { Payload } from "../../messaging/payload";
import type { TimelineItem } from "../../messaging/timeline";
import { MessageItem, type MessageItemProps, type NameOf } from "./MessageItem";
import type { Composer } from "./useComposer";

/** Props of {@link MessageList}. */
export interface MessageListProps {
  timeline: TimelineItem[] | undefined;
  people: { self: string; nameOf: NameOf };
  composer: Composer;
  send: (payload: Payload) => Promise<unknown>;
}

/** What pressing a message and its actions does. */
function handlersFor(
  item: TimelineItem,
  state: {
    selected: boolean;
    select: (id: string | undefined) => void;
    composer: Composer;
    send: (payload: Payload) => Promise<unknown>;
  },
): MessageItemProps["on"] {
  const { select, composer, send } = state;
  const then = (action: () => void) => () => {
    select(undefined);
    action();
  };
  const sendQuietly = (payload: Payload): void => {
    // A failed delete or reaction leaves the message as it was, which shows it.
    send(payload).catch(() => undefined);
  };
  return {
    select: () => {
      select(state.selected ? undefined : item.id);
    },
    reply: then(() => {
      composer.replyTo(item);
    }),
    edit: then(() => {
      composer.edit(item);
    }),
    delete: then(() => {
      sendQuietly({ kind: "delete", target: item.id });
    }),
    react: (emoji, added) => {
      select(undefined);
      sendQuietly({ kind: "reaction", target: item.id, emoji, added });
    },
  };
}

/**
 * The messages, newest at the bottom. The list is inverted, so it opens at
 * the latest message and stays there as new ones arrive; only what is on
 * screen is drawn, however long the history.
 */
export function MessageList({ timeline, people, composer, send }: MessageListProps): ReactNode {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string>();
  const byId = useMemo(() => new Map((timeline ?? []).map((item) => [item.id, item])), [timeline]);
  const newestFirst = useMemo(() => [...(timeline ?? [])].reverse(), [timeline]);
  if (timeline === undefined) {
    return <Empty text={t("conversation.loading")} />;
  }
  if (timeline.length === 0) {
    return <Empty text={t("conversation.empty")} />;
  }
  return (
    <FlatList
      inverted
      data={newestFirst}
      keyExtractor={(item) => item.id}
      contentContainerStyle={styles.list}
      renderItem={({ item }) => (
        <MessageItem
          item={item}
          replied={item.replyTo === undefined ? undefined : byId.get(item.replyTo)}
          context={{ ...people, selected: selected === item.id }}
          on={handlersFor(item, {
            selected: selected === item.id,
            select: setSelected,
            composer,
            send,
          })}
        />
      )}
    />
  );
}

function Empty({ text }: { text: string }): ReactNode {
  return (
    <View style={styles.empty}>
      <Text tone="muted">{text}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  list: { padding: spacing.md, gap: spacing.sm },
  empty: { flex: 1, padding: spacing.lg, justifyContent: "center" },
});
