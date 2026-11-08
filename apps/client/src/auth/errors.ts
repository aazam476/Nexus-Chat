/**
 * Turning API errors into messages for the user. Only codes the screens
 * expect get their own message; anything else is reported generically and
 * the details stay in the server's log.
 */
import type { Error as ApiError } from "@nexus-chat/api-client";

/** A translation key with its values. */
export interface Message {
  key: string;
  values?: Record<string, string | number>;
}

/** Field name to message, for validation errors. */
export type FieldMessages = Partial<Record<string, string>>;

/** The message for an API error, using `known` for codes the screen expects. */
export function messageFor(error: unknown, known: Record<string, string>): Message {
  const code = codeOf(error);
  const key = code === undefined ? undefined : known[code];
  return { key: key ?? "common.unexpectedError" };
}

/** The error's code, if it is an API error. */
export function codeOf(error: unknown): string | undefined {
  if (typeof error === "object" && error !== null && "code" in error && typeof error.code === "string") {
    return error.code;
  }
  return undefined;
}

/** The server's message for each invalid field of a validation error. */
export function fieldMessages(error: unknown): FieldMessages {
  const fields = (error as Partial<ApiError> | undefined)?.fields ?? [];
  return Object.fromEntries(fields.map((field) => [field.field, field.message]));
}
