import { Button, Stack, Text } from "@nexus-chat/ui";
import { useState, type ReactNode } from "react";

/** Props of {@link ConfirmAction}. */
export interface ConfirmActionProps {
  /** The button that starts it, such as "Delete channel". */
  label: string;
  /** What screen readers say for that button, when the label leans on what is around it. */
  accessibilityLabel?: string;
  /** What is asked before it happens. */
  question: string;
  confirmLabel: string;
  cancelLabel: string;
  onConfirm: () => void;
  busy?: boolean;
  /** The first button's test ID; the confirming one's is `${testID}-confirm`. */
  testID: string;
}

/** The question, with buttons to go ahead or keep things as they are. */
function Question(props: ConfirmActionProps & { onCancel: () => void }): ReactNode {
  const { question, confirmLabel, cancelLabel, onConfirm, busy = false, testID, onCancel } = props;
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
      <Button label={cancelLabel} variant="secondary" onPress={onCancel} />
    </Stack>
  );
}

/**
 * A destructive action that is asked once more in place, rather than in a
 * dialog that covers what it is about.
 */
export function ConfirmAction(props: ConfirmActionProps): ReactNode {
  const { label, accessibilityLabel, testID } = props;
  const [asking, setAsking] = useState(false);
  if (!asking) {
    return (
      <Button
        label={label}
        {...(accessibilityLabel === undefined ? {} : { accessibilityLabel })}
        variant="danger"
        onPress={() => {
          setAsking(true);
        }}
        testID={testID}
      />
    );
  }
  return (
    <Question
      {...props}
      onCancel={() => {
        setAsking(false);
      }}
    />
  );
}
