/** Turning what the user asked for on the search screen into index filters. */
import type { SearchRequest } from "../screens/search/SearchForm";
import type { SearchFilters } from "./query";

const days = { week: 7, month: 30, year: 365 } as const;

/** The sender as the index names users: `@name@instance`, the user's own instance when none is given. */
export function senderOf(from: string, domain: string): string | undefined {
  const name = from.trim().replace(/^@/, "").toLowerCase();
  if (name === "") {
    return undefined;
  }
  return name.includes("@") ? `@${name}` : `@${name}@${domain}`;
}

/** The index filters for a request; `scope` is the conversation it was opened from. */
export function filtersOf(
  request: SearchRequest,
  context: { scope: string | undefined; domain: string; now: Date },
): SearchFilters {
  const from = senderOf(request.from, context.domain);
  return {
    ...(from === undefined ? {} : { from }),
    ...(request.here && context.scope !== undefined ? { in: [context.scope] } : {}),
    ...(request.attachments ? { hasAttachments: true } : {}),
    ...(request.period === "any"
      ? {}
      : { after: new Date(context.now.getTime() - days[request.period] * 86_400_000).toISOString() }),
  };
}
