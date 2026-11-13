/** Who the open conversation's mentions name, for the messages that show them. */
import { createContext, use } from "react";

/** Names mentioned users, and says which one is the signed-in user. */
export interface MentionNames {
  nameOfId: (userId: string) => string | undefined;
  selfId: string;
}

/** The open conversation's mention names; undefined outside a conversation. */
export const MentionNamesContext = createContext<MentionNames | undefined>(undefined);

/** The open conversation's mention names, if any. */
export function useMentionNames(): MentionNames | undefined {
  return use(MentionNamesContext);
}
