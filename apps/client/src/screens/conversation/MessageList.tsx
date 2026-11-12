import { spacing, Text } from "@nexus-chat/ui";
import { useMemo, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { FlatList, StyleSheet, View } from "react-native";

import type { Payload } from "../../messaging/payload";
import type { TimelineItem } from "../../messaging/timeline";
import { ActionButton } from "./ActionButton";
import { MessageItem, type MessageItemProps, type NameOf } from "./MessageItem";
import type { Composer } from "./useComposer";

/** Props of {@link MessageList}. */
export interface MessageListProps {
  timeline: TimelineItem[] | undefined;
  people: { self: string; nameOf: NameOf };
  composer: Composer;
  send: (payload: Payload) => Promise<unknown>;
  /** Where new messages begin, if any are unread. */
  firstUnreadId?: string | undefined;
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

/** Draws one message, given what the list knows. */
function renderer(state: {
  byId: ReadonlyMap<string, TimelineItem>;
  props: MessageListProps;
  selected: string | undefined;
  select: (id: string | undefined) => void;
}): (info: { item: TimelineItem }) => ReactNode {
  const { byId, props, selected, select } = state;
  return ({ item }) => (
    <MessageItem
      item={item}
      replied={item.replyTo === undefined ? undefined : byId.get(item.replyTo)}
      context={{
        ...props.people,
        selected: selected === item.id,
        firstUnread: props.firstUnreadId === item.id,
      }}
      on={handlersFor(item, {
        selected: selected === item.id,
        select,
        composer: props.composer,
        send: props.send,
      })}
    />
  );
}

/**
 * The messages, newest at the bottom. The list is inverted, so it opens at
 * the latest message and stays there as new ones arrive; only what is on
 * screen is drawn, however long the history. "Jump to first unread" takes
 * the user to where new messages begin.
 */
export function MessageList(props: MessageListProps): ReactNode {
  const { timeline, firstUnreadId } = props;
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string>();
  const listRef = useRef<FlatList<TimelineItem>>(null);
  const byId = useMemo(() => new Map((timeline ?? []).map((item) => [item.id, item])), [timeline]);
  const newestFirst = useMemo(() => [...(timeline ?? [])].reverse(), [timeline]);
  if (timeline === undefined || timeline.length === 0) {
    return <Empty text={t(timeline === undefined ? "conversation.loading" : "conversation.empty")} />;
  }
  const unreadIndex = newestFirst.findIndex((item) => item.id === firstUnreadId);
  return (
    <View style={styles.fill}>
      {unreadIndex < 0 ? null : (
        <ActionButton
          label={t("conversation.jumpToUnread")}
          onPress={() => {
            listRef.current?.scrollToIndex({ index: unreadIndex, viewPosition: 1 });
          }}
          testID="jump-to-unread"
        />
      )}
      <FlatList
        ref={listRef}
        inverted
        data={newestFirst}
        keyExtractor={(item) => item.id}
        contentContainerStyle={styles.list}
        onScrollToIndexFailed={(info) => {
          listRef.current?.scrollToOffset({ offset: info.averageItemLength * info.index });
        }}
        renderItem={renderer({ byId, props, selected, select: setSelected })}
      />
    </View>
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
  fill: { flex: 1 },
  list: { padding: spacing.md, gap: spacing.sm },
  empty: { flex: 1, padding: spacing.lg, justifyContent: "center" },
});
