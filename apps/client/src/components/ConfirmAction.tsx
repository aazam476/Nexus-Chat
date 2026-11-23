import { Button, Stack, Text } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";

/** Props of {@link ConfirmAction}. */
export interface ConfirmActionProps {
  /** The button that starts it, such as "Delete channel". */
  label: string;
  /** What is asked before it happens. */
  question: string;
  confirmLabel: string;
  cancelLabel: string;
  onConfirm: () => void;
  busy?: boolean;
  /** The first button's test ID; the confirming one's is `${testID}-confirm`. */
  testID: string;
}

/**
 * A destructive action that is asked once more in place, rather than in a
 * dialog that covers what it is about.
 */
export function ConfirmAction(props: ConfirmActionProps): ReactNode {
  const { label, question, confirmLabel, cancelLabel, onConfirm, busy = false, testID } = props;
  const [asking, setAsking] = useState(false);
  if (!asking) {
    return (
      <Button
        label={label}
        variant="danger"
        onPress={() => {
          setAsking(true);
        }}
        testID={testID}
      />
    );
  }
  return (
    <Stack gap="sm">
      <Text live>{question}</Text>
      <Button
        label={confirmLabel}
        variant="danger"
        onPress={onConfirm}
        busy={busy}
        testID={`${testID}-confirm`}
      />
      <Button
        label={cancelLabel}
        variant="secondary"
        onPress={() => {
          setAsking(false);
        }}
      />
    </Stack>
  );
}
