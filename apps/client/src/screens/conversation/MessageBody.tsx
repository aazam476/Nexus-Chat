import { radius, spacing, type, useTheme, type Colors } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Linking, Platform, Text as NativeText, StyleSheet, View } from "react-native";

import { parseMarkdown, type Block, type Inline } from "../../messaging/markdown";

const monospace = Platform.select({ ios: "Menlo", android: "monospace", default: "monospace" });

/**
 * A spoiler: until pressed, a bar that says "Spoiler" stands in for it, so
 * the hidden text is neither shown, selectable nor read out.
 */
function Spoiler({ children }: { children: ReactNode }): ReactNode {
  const { t } = useTranslation();
  const { colors } = useTheme();
  const [shown, setShown] = useState(false);
  if (shown) {
    return <NativeText>{children}</NativeText>;
  }
  return (
    <NativeText
      role="button"
      accessibilityHint={t("conversation.spoilerHint")}
      onPress={() => {
        setShown(true);
      }}
      style={{ backgroundColor: colors.textMuted, color: colors.background }}
    >
      {t("conversation.spoiler")}
    </NativeText>
  );
}

const marks = {
  bold: { fontWeight: "700" },
  italic: { fontStyle: "italic" },
  strike: { textDecorationLine: "line-through" },
} as const;

function LinkText({ url, children }: { url: string; children: ReactNode }): ReactNode {
  const { colors } = useTheme();
  return (
    <NativeText
      role="link"
      style={[styles.link, { color: colors.primary }]}
      onPress={() => void Linking.openURL(url)}
    >
      {children}
    </NativeText>
  );
}

function mentionText(node: Extract<Inline, { type: "mention" }>): string {
  return "id" in node ? `@${node.id}` : `@${node.kind}`;
}

function renderNode(node: Inline, key: number, colors: Colors): ReactNode {
  switch (node.type) {
    case "text":
      return node.text;
    case "break":
      return "\n";
    case "bold":
    case "italic":
    case "strike":
      return (
        <NativeText key={key} style={marks[node.type]}>
          {renderInline(node.children, colors)}
        </NativeText>
      );
    case "spoiler":
      return <Spoiler key={key}>{renderInline(node.children, colors)}</Spoiler>;
    case "code":
      return (
        <NativeText key={key} style={[styles.mono, { backgroundColor: colors.surface }]}>
          {node.text}
        </NativeText>
      );
    case "link":
      return (
        <LinkText key={key} url={node.url}>
          {renderInline(node.children, colors)}
        </LinkText>
      );
    case "mention":
      return (
        <NativeText key={key} style={[styles.mention, { color: colors.primary }]}>
          {mentionText(node)}
        </NativeText>
      );
  }
}

function renderInline(nodes: Inline[], colors: Colors): ReactNode[] {
  return nodes.map((node, index) => renderNode(node, index, colors));
}

function renderBlock(block: Block, index: number, colors: Colors): ReactNode {
  if (block.type === "code") {
    return (
      <View key={index} style={[styles.codeBlock, { backgroundColor: colors.surface }]}>
        <NativeText style={[type.small, styles.mono, { color: colors.text }]} selectable>
          {block.text}
        </NativeText>
      </View>
    );
  }
  const text = (
    <NativeText style={[type.body, { color: colors.text }]} selectable>
      {renderInline(block.children, colors)}
    </NativeText>
  );
  if (block.type === "quote") {
    return (
      <View key={index} style={[styles.quote, { borderColor: colors.border }]}>
        {text}
      </View>
    );
  }
  return <View key={index}>{text}</View>;
}

/** Props of {@link MessageBody}. */
export interface MessageBodyProps {
  text: string;
}

/** A message's Markdown, drawn with the theme. */
export function MessageBody({ text }: MessageBodyProps): ReactNode {
  const { colors } = useTheme();
  return (
    <View style={styles.body}>
      {parseMarkdown(text).map((block, index) => renderBlock(block, index, colors))}
    </View>
  );
}

const styles = StyleSheet.create({
  body: { gap: spacing.xs },
  mono: { fontFamily: monospace },
  link: { textDecorationLine: "underline" },
  mention: { fontWeight: "600" },
  codeBlock: { padding: spacing.sm, borderRadius: radius.control },
  quote: { borderLeftWidth: 3, paddingLeft: spacing.sm },
});
