/**
 * Form state for screens with several text fields: values by name, and
 * the props that wire one field to its value, server error and test ID.
 */
import { useState } from "react";

import type { FieldMessages } from "../auth/errors";

/** Props a TextField needs for one form field. */
export interface FieldBinding {
  value: string;
  onChangeText: (value: string) => void;
  error: string | undefined;
  testID: string;
}

/** A form's values and how to bind its fields. */
export interface Form<Values> {
  values: Values;
  setValue: (name: keyof Values, value: string) => void;
  /** Binds field `name`; `errorPath` is the field's name in server errors when it differs. */
  bind: (name: keyof Values & string, errorPath?: string) => FieldBinding;
}

/**
 * Keeps a form's values. Generic because every form has its own fields;
 * the type keeps a misspelt field name from compiling.
 */
export function useForm<Values extends Record<string, string>>(
  initial: Values,
  errors: FieldMessages,
  prefix: string,
): Form<Values> {
  const [values, setValues] = useState(initial);
  const setValue = (name: keyof Values, value: string): void => {
    setValues((current) => ({ ...current, [name]: value }));
  };
  const bind = (name: keyof Values & string, errorPath: string = name): FieldBinding => ({
    value: values[name] ?? "",
    onChangeText: (value) => {
      setValue(name, value);
    },
    error: errors[errorPath],
    testID: `${prefix}-${name}`,
  });
  return { values, setValue, bind };
}
