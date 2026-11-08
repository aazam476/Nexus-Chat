import type { ReactNode } from "react";
import { StyleSheet, TextInput, View, type TextInputProps } from "react-native";

import { Text } from "./Text";
import { useTheme } from "./theme";
import { minimumTouchTarget, radius, spacing, type } from "./tokens";
import { useInteraction } from "./useInteraction";

/** Props of {@link TextField}. */
export interface TextFieldProps {
  /** The visible label, which is also the field's accessible name. */
  label: string;
  value: string;
  onChangeText: (text: string) => void;
  /** Help shown under the field. */
  hint?: string;
  /** An error shown under the field and announced to screen readers. */
  error?: string;
  secure?: boolean;
  autoComplete?: TextInputProps["autoComplete"];
  keyboardType?: TextInputProps["keyboardType"];
  autoCapitalize?: TextInputProps["autoCapitalize"];
  onSubmitEditing?: () => void;
  /** Visible lines for multi-line text, such as a pasted certificate; one line when unset. */
  lines?: number;
  testID?: string;
}

/**
 * A labelled text input. The label is always visible (placeholders vanish
 * as you type), and hints and errors are linked to the field for screen
 * readers.
 */
export function TextField(props: TextFieldProps): ReactNode {
  const { colors } = useTheme();
  const { focused, handlers } = useInteraction();
  const id = props.testID ?? props.label;
  const described = [
    props.hint === undefined ? "" : `${id}-hint`,
    props.error === undefined ? "" : `${id}-error`,
  ]
    .join(" ")
    .trim();
  const borderColor = props.error !== undefined ? colors.danger : focused ? colors.focus : colors.border;
  return (
    <View style={styles.field}>
      <Text variant="label" nativeID={`${id}-label`}>
        {props.label}
      </Text>
      <FieldInput
        {...props}
        id={id}
        described={described}
        borderColor={borderColor}
        focused={focused}
        handlers={handlers}
      />
      <FieldMessages id={id} hint={props.hint} error={props.error} />
    </View>
  );
}

interface FieldInputProps extends TextFieldProps {
  id: string;
  described: string;
  borderColor: string;
  focused: boolean;
  handlers: { onFocus: () => void; onBlur: () => void };
}

/** The input itself, wired to its label, hint and error. */
function FieldInput(props: FieldInputProps): ReactNode {
  const { colors } = useTheme();
  return (
    <TextInput
      aria-labelledby={`${props.id}-label`}
      accessibilityLabel={props.label}
      aria-describedby={props.described === "" ? undefined : props.described}
      aria-invalid={props.error !== undefined}
      value={props.value}
      onChangeText={props.onChangeText}
      secureTextEntry={props.secure}
      autoComplete={props.autoComplete}
      keyboardType={props.keyboardType}
      autoCapitalize={props.autoCapitalize ?? "none"}
      autoCorrect={false}
      onSubmitEditing={props.onSubmitEditing}
      multiline={props.lines !== undefined}
      numberOfLines={props.lines}
      onFocus={props.handlers.onFocus}
      onBlur={props.handlers.onBlur}
      testID={props.testID}
      style={[
        styles.input,
        type.body,
        { color: colors.text, backgroundColor: colors.surface, borderColor: props.borderColor },
        props.focused && styles.focused,
        props.lines !== undefined && {
          minHeight: props.lines * type.body.lineHeight,
          textAlignVertical: "top" as const,
        },
      ]}
    />
  );
}

function FieldMessages({
  id,
  hint,
  error,
}: {
  id: string;
  hint: string | undefined;
  error: string | undefined;
}): ReactNode {
  return (
    <>
      {hint !== undefined && (
        <Text variant="small" tone="muted" nativeID={`${id}-hint`}>
          {hint}
        </Text>
      )}
      {error !== undefined && (
        <Text variant="small" tone="danger" nativeID={`${id}-error`} live>
          {error}
        </Text>
      )}
    </>
  );
}

const styles = StyleSheet.create({
  field: { gap: spacing.xs },
  input: {
    minHeight: minimumTouchTarget,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: radius.control,
    borderWidth: 2,
  },
  focused: { borderWidth: 3 },
});
