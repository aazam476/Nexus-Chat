/**
 * The messaging half of the fake instance: just enough delivery service
 * for the app to set up encryption for real in the browser, open its
 * conversations as their only device, and send. It keeps what the app
 * stores (identity, backup, messages) for the length of one page.
 */
import type { Dataset } from "@nexus-chat/fixtures";

/** A status and JSON body, as the fake answers. */
export type Answer = [number, unknown];

/** The DM the conversation screen shows. */
export const dmId = "0195f0e0-0000-7000-8000-00000000d001";
const groupId = "0195f0e0-0000-7000-8000-00000000d002";
const createdAt = "2026-11-08T11:00:00Z";

/** The DM with the first contact, and the group with all of them. */
function conversations(dataset: Dataset): unknown[] {
  const { user, messaging } = dataset;
  const me = { id: user.id, username: user.username, displayName: user.displayName };
  const base = { mlsGroupId: "AAAAAAAAAAAAAAAAAAAAAA==", createdAt };
  return [
    { ...base, id: dmId, kind: "dm", members: [me, ...messaging.contacts.slice(0, 1)] },
    { ...base, id: groupId, kind: "group", name: messaging.groupName, members: [me, ...messaging.contacts] },
  ];
}

/** What one page's app has stored, and the answers built from it. */
export class FakeMessaging {
  private identity: unknown;
  private readonly blobs = new Map<string, string>();
  private readonly wrapped = new Map<string, string>();
  private readonly sent = new Map<string, number>();
  private nextId = 1;

  constructor(private readonly dataset: Dataset) {}

  /** The answer to `method path`, or undefined when it is not messaging. */
  answer(method: string, path: string, body: unknown): Answer | undefined {
    return this.keys(method, path, body) ?? this.delivery(method, path);
  }

  private keys(method: string, path: string, body: unknown): Answer | undefined {
    const { user } = this.dataset;
    const blob = /^\/api\/v1\/key-backup\/blobs\/([^/]+)$/.exec(path)?.[1];
    const wrapped = /^\/api\/v1\/key-backup\/wrapped-keys\/([^/]+)$/.exec(path)?.[1];
    const data = (body ?? {}) as { data?: string; wrappedKey?: string };
    switch (`${method} ${blob === undefined ? path : "blob"}`) {
      case `GET /api/v1/users/${user.id}/identity`:
        return this.identity === undefined
          ? [404, { code: "not_found", message: "none" }]
          : [200, this.identity];
      case "PUT /api/v1/identity":
        this.identity = body;
        return [204, null];
      case "GET /api/v1/key-backup":
        return [200, this.backup()];
      case "PUT blob":
        this.blobs.set(blob ?? "", data.data ?? "");
        return [204, null];
      case "GET blob":
        return this.blobs.has(blob ?? "")
          ? [200, { data: this.blobs.get(blob ?? "") }]
          : [404, { code: "not_found", message: "none" }];
      case "GET /api/v1/key-packages":
      case "POST /api/v1/key-packages":
        return [
          200,
          { oneTime: 50, hasLastResort: true, refill: method === "GET" && this.identity === undefined },
        ];
    }
    if (wrapped !== undefined && method === "PUT") {
      this.wrapped.set(wrapped, data.wrappedKey ?? "");
      return [204, null];
    }
    return undefined;
  }

  private backup(): unknown {
    return {
      salt: "AAAAAAAAAAAAAAAAAAAAAA==",
      wrappedKeys: [...this.wrapped].map(([method, wrappedKey]) => ({ method, wrappedKey })),
      blobs: [...this.blobs.keys()].map((name) => ({ name, size: 1, updatedAt: createdAt })),
    };
  }

  private delivery(method: string, path: string): Answer | undefined {
    const conversation = /^\/api\/v1\/conversations\/([^/]+)(\/.*)?$/.exec(path);
    const id = conversation?.[1] ?? "";
    switch (`${method} ${conversation === null ? path : (conversation[2] ?? "")}`) {
      case "GET /api/v1/conversations":
        return [200, conversations(this.dataset)];
      case "GET /api/v1/read-state":
        return [
          200,
          [{ conversationId: groupId, lastReadId: 0, unreadCount: this.dataset.messaging.groupUnread }],
        ];
      case "GET /api/v1/friends":
        return [200, []];
      case "GET ":
        return [200, conversations(this.dataset).find((item) => (item as { id: string }).id === id)];
      case "GET /mls":
        return [200, { groupId: "AAAAAAAAAAAAAAAAAAAAAA==", epoch: this.sent.has(id) ? 1 : 0 }];
      case "POST /mls/commits":
        this.sent.set(id, 0);
        return [200, { epoch: 1 }];
      case "POST /mls/messages":
        return [201, { id: this.nextId++ }];
      case "GET /mls/messages":
      case "GET /archive":
        return [200, []];
      case "PUT /read":
        return [204, null];
    }
    return /^\/api\/v1\/users\/[^/]+\/key-packages\/claim$/.test(path) ? [200, []] : undefined;
  }
}
