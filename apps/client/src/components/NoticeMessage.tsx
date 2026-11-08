import { Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";

import type { Message } from "../auth/errors";
import { useMessageText } from "../i18n/i18n";

/** Props of {@link NoticeMessage}. */
export interface NoticeMessageProps {
  message: Message | undefined;
  testID?: string;
}

/**
 * Confirms that a change worked ("Saved."), announced to screen readers.
 * It appears in place, next to what changed, instead of a toast that
 * covers the page and disappears before it is read.
 */
export function NoticeMessage({ message, testID = "form-notice" }: NoticeMessageProps): ReactNode {
  const text = useMessageText(message);
  if (text === undefined) {
    return null;
  }
  return (
    <Text live testID={testID}>
      {text}
    </Text>
  );
}
