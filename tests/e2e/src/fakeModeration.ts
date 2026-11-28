/**
 * The moderation half of the fake instance: a server's bans, audit log and
 * reports, and the Admin panel's accounts, servers, numbers and settings,
 * all built from the dataset so the worst-case names and reasons show.
 * Reports carry no real evidence, so they show as not verified.
 */
import type { Dataset } from "@nexus-chat/fixtures";

import type { Answer } from "./fakeMessaging.js";
import { serverId } from "./fakeServers.js";

const createdAt = "2026-11-01T09:00:00Z";

/** The longest message of the dataset, as a reason or comment. */
const longest = (dataset: Dataset): string =>
  dataset.messaging.messages.reduce((a, b) => (b.length > a.length ? b : a), "");

function bans(dataset: Dataset): unknown[] {
  return dataset.messaging.contacts.map((contact) => ({
    user: contact,
    reason: longest(dataset),
    createdAt,
  }));
}

function auditLog(dataset: Dataset): unknown[] {
  const [first] = dataset.messaging.contacts;
  const actions = [
    "member.ban",
    "role.create",
    "channel.update",
    "member.kick",
    "report.resolve",
    "invite.create",
  ];
  return actions.map((action, index) => ({
    id: 100 - index,
    actorId: dataset.user.id,
    action,
    targetKind: action.split(".")[0],
    targetId: first?.id,
    details: {
      name: dataset.servers.textChannels[0]?.name,
      reason: longest(dataset),
      username: first?.username,
    },
    createdAt,
  }));
}

function reports(dataset: Dataset): unknown[] {
  return [
    {
      id: "0195f0e0-0000-7000-8000-00000000a501",
      serverId,
      messageId: "0195f0e0-0000-7000-8000-00000000a502",
      evidence: "AA==",
      receipt: "AA==",
      receiptSignature: "AA==",
      comment: longest(dataset),
      status: "open",
      escalated: false,
      resolution: "",
      createdAt,
    },
  ];
}

function adminUsers(dataset: Dataset): unknown[] {
  return [dataset.user, ...dataset.messaging.contacts].map((user, index) => ({
    id: user.id,
    username: user.username,
    displayName: user.displayName,
    email: `${user.username}@example.com`,
    isAdmin: index === 0,
    createdAt,
    ...(index === 1
      ? { suspendedAt: createdAt, suspensionReason: longest(dataset) }
      : { suspensionReason: "" }),
  }));
}

/** The Admin panel's numbers and settings. */
function adminSettings(): Record<string, Answer> {
  const stats = {
    users: 1_234_567,
    suspendedUsers: 1234,
    activeUsers: 98_765,
    servers: 4321,
    conversations: 54_321,
    recentMessages: 9_876_543,
    openReports: 12,
    attachmentBytes: 5 * 2 ** 40,
  };
  const federation = {
    mode: "allowlist",
    allowed: ["friends.example", "a-very-long-instance-name-for-testing-wrapping.example.org"],
    blocked: ["spam.example"],
  };
  return {
    "GET /api/v1/admin/stats": [200, stats],
    "GET /api/v1/admin/registration": [200, { mode: "invite_only" }],
    "GET /api/v1/admin/moderation": [200, { reportsToAdmins: false }],
    "GET /api/v1/admin/federation": [200, federation],
  };
}

/** The answer to `method path`, or undefined when it is not about moderation. */
export function answerModeration(dataset: Dataset, method: string, path: string): Answer | undefined {
  const { user, servers } = dataset;
  const listed = {
    id: serverId,
    name: servers.name,
    ownerId: user.id,
    ownerUsername: user.username,
    members: 123_456,
    createdAt,
  };
  const table: Record<string, Answer> = {
    [`GET /api/v1/servers/${serverId}/bans`]: [200, bans(dataset)],
    [`GET /api/v1/servers/${serverId}/audit-log`]: [200, auditLog(dataset)],
    [`GET /api/v1/servers/${serverId}/reports`]: [200, reports(dataset)],
    "GET /api/v1/admin/reports": [200, reports(dataset)],
    "GET /api/v1/admin/users": [200, adminUsers(dataset)],
    "GET /api/v1/admin/servers": [200, [listed]],
    ...adminSettings(),
  };
  return table[`${method} ${path}`];
}
