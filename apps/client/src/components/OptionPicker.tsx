import { Button, Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";

/** One option: its value and what it is called. */
export interface Option<T> {
  value: T;
  label: string;
}

/** Props of {@link OptionPicker}. */
export interface OptionPickerProps<T> {
  label: string;
  options: readonly Option<T>[];
  value: T;
  onChange: (value: T) => void;
  /** Each option's button is `${testID}-${index}`. */
  testID: string;
}

/** One of a few options, as buttons; the chosen one is marked with a check, not only colored. */
export function OptionPicker<T>({
  label,
  options,
  value,
  onChange,
  testID,
}: OptionPickerProps<T>): ReactNode {
  return (
    <Stack gap="sm">
      <Text variant="label">{label}</Text>
      {options.map((option, index) => (
        <Button
          key={option.label}
          label={option.value === value ? `✓ ${option.label}` : option.label}
          variant={option.value === value ? "primary" : "secondary"}
          onPress={() => {
            onChange(option.value);
          }}
          testID={`${testID}-${String(index)}`}
        />
      ))}
    </Stack>
  );
}
