import { spacing, Text, useTheme } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Pressable, StyleSheet, View } from "react-native";

import type { TimelineItem } from "../../messaging/timeline";
import { ActionButton } from "./ActionButton";
import { AttachmentList } from "./AttachmentList";
import { MessageBody } from "./MessageBody";
import { PreviewCard } from "./PreviewCard";
import { Reactions } from "./Reactions";

/** What the people in a conversation are called, by `@name@instance`. */
export type NameOf = (user: string) => string;

/** Props of {@link MessageItem}. */
export interface MessageItemProps {
  item: TimelineItem;
  /** The message it replies to, when this device has it. */
  replied: TimelineItem | undefined;
  /** `firstUnread` marks where new messages begin. */
  context: { self: string; nameOf: NameOf; selected: boolean; firstUnread?: boolean };
  on: {
    select: () => void;
    reply: () => void;
    edit: () => void;
    delete: () => void;
    react: (emoji: string, added: boolean) => void;
  };
}

const quickReactions = ["👍", "❤️", "😂", "🎉"];

function ReplyPreview({
  item,
  replied,
  nameOf,
}: {
  item: TimelineItem;
  replied: TimelineItem | undefined;
  nameOf: NameOf;
}): ReactNode {
  const { t } = useTranslation();
  if (item.replyTo === undefined) {
    return null;
  }
  return (
    <Text variant="small" tone="muted">
      {replied === undefined
        ? t("conversation.missingReply")
        : t("conversation.replyTo", {
            name: nameOf(replied.sender),
            text: replied.deleted ? t("conversation.deleted") : replied.text.slice(0, 80),
          })}
    </Text>
  );
}

function Actions({
  item,
  self,
  on,
}: {
  item: TimelineItem;
  self: string;
  on: MessageItemProps["on"];
}): ReactNode {
  const { t } = useTranslation();
  const own = item.sender === self;
  return (
    <View style={styles.actions}>
      <ActionButton label={t("conversation.reply")} onPress={on.reply} />
      {quickReactions.map((emoji) => (
        <ActionButton
          key={emoji}
          label={emoji}
          accessibilityLabel={t("conversation.react", { emoji })}
          onPress={() => {
            on.react(emoji, true);
          }}
        />
      ))}
      {own ? <ActionButton label={t("conversation.edit")} onPress={on.edit} /> : null}
      {own ? (
        <ActionButton label={t("conversation.delete")} onPress={on.delete} testID="message-delete" />
      ) : null}
    </View>
  );
}

/** Who sent the message, and when. */
function Byline({ name, sentAt }: { name: string; sentAt: string }): ReactNode {
  const time = new Date(sentAt).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
  return (
    <Text variant="label">
      {name}{" "}
      <Text variant="small" tone="muted">
        {time}
      </Text>
    </Text>
  );
}

/** The line above the first unread message. */
function NewMessages(): ReactNode {
  const { t } = useTranslation();
  const { colors } = useTheme();
  return (
    <View style={[styles.divider, { borderColor: colors.danger }]} testID="new-messages">
      <Text variant="small" tone="danger">
        {t("conversation.newMessages")}
      </Text>
    </View>
  );
}

/** One message: who sent it, what it replies to, its text, reactions and actions. */
export function MessageItem({ item, replied, context, on }: MessageItemProps): ReactNode {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const name = context.nameOf(item.sender);
  return (
    <View style={styles.item}>
      {context.firstUnread === true ? <NewMessages /> : null}
      <Pressable
        onPress={item.deleted ? undefined : on.select}
        accessibilityLabel={t("conversation.messageActions", { name })}
        accessibilityState={{ expanded: context.selected }}
        style={({ pressed }) => [
          styles.press,
          { backgroundColor: pressed || context.selected ? colors.surface : "transparent" },
        ]}
        testID={`message-${item.id}`}
      >
        <Byline name={name} sentAt={item.sentAt} />
        <ReplyPreview item={item} replied={replied} nameOf={context.nameOf} />
        {item.deleted ? (
          <Text tone="muted">{t("conversation.deleted")}</Text>
        ) : (
          <>
            {item.text === "" ? null : <MessageBody text={item.text} />}
            <PreviewCard preview={item.preview} text={item.text} />
            <AttachmentList files={item.attachments} />
          </>
        )}
        {item.edited && !item.deleted ? (
          <Text variant="small" tone="muted">
            {t("conversation.edited")}
          </Text>
        ) : null}
      </Pressable>
      <Reactions reactions={item.reactions} self={context.self} nameOf={context.nameOf} onToggle={on.react} />
      {context.selected && !item.deleted ? <Actions item={item} self={context.self} on={on} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  item: { gap: spacing.xs },
  divider: { borderTopWidth: StyleSheet.hairlineWidth, paddingTop: spacing.xs, alignItems: "flex-end" },
  press: { padding: spacing.sm, borderRadius: 8, gap: spacing.xs },
  actions: { flexDirection: "row", flexWrap: "wrap", gap: spacing.sm, paddingHorizontal: spacing.sm },
});
