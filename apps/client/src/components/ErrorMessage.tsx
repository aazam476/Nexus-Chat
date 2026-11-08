import { Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";

import type { Message } from "../auth/errors";
import { useMessageText } from "../i18n/i18n";

/** Props of {@link ErrorMessage}. */
export interface ErrorMessageProps {
  message: Message | undefined;
}

/** A form's error, announced to screen readers when it appears. */
export function ErrorMessage({ message }: ErrorMessageProps): ReactNode {
  const text = useMessageText(message);
  if (text === undefined) {
    return null;
  }
  return (
    <Text tone="danger" live testID="form-error">
      {text}
    </Text>
  );
}
