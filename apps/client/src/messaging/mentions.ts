/**
 * Mentions (spec §4.5): typing `@` and part of a name in the composer
 * suggests the conversation's members, and choosing one writes `<@id>`
 * into the message, which every device shows as the member's name. A
 * message mentions a user when it names them, or says `@everyone` or `@here`.
 */
import { parseMarkdown, type Block, type Inline } from "./markdown";

/** Someone who can be mentioned. */
export interface Mentionable {
  id: string;
  username: string;
  displayName: string;
}

/** The most suggestions shown at once. */
const suggestionLimit = 5;

/** The name being typed after a trailing `@`, or undefined when there is none. */
export function mentionQuery(draft: string): string | undefined {
  return /(?:^|\s)@([\p{L}\p{N}_.-]{0,32})$/u.exec(draft)?.[1];
}

/** Members whose username or display name starts with `query`, the user excluded. */
export function suggestMentions(query: string, people: Mentionable[], selfId: string): Mentionable[] {
  const wanted = query.toLocaleLowerCase();
  return people
    .filter((person) => person.id !== selfId)
    .filter(
      (person) =>
        person.username.toLocaleLowerCase().startsWith(wanted) ||
        person.displayName.toLocaleLowerCase().startsWith(wanted),
    )
    .slice(0, suggestionLimit);
}

/** The draft with its trailing `@query` replaced by a mention of `person`. */
export function insertMention(draft: string, person: Mentionable): string {
  return draft.replace(/@[\p{L}\p{N}_.-]{0,32}$/u, `<@${person.id}> `);
}

function inlineMentions(nodes: Inline[], userId: string): boolean {
  return nodes.some((node) => {
    if (node.type === "mention") {
      return "id" in node ? node.kind === "user" && node.id === userId : true;
    }
    return "children" in node && inlineMentions(node.children, userId);
  });
}

/** Whether a message's text mentions `userId`, or everyone. */
export function mentionsUser(text: string, userId: string): boolean {
  return parseMarkdown(text).some(
    (block: Block) => block.type !== "code" && inlineMentions(block.children, userId),
  );
}
