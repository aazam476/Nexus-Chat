/**
 * An instance faked at the network boundary for the visual checks: every
 * API request and the gateway are answered from one dataset, so a screen
 * can be shown with any data without a server (code standards §8.5,
 * worst-case data rule). The app itself runs unchanged.
 */
import type { Dataset } from "@nexus-chat/fixtures";
import type { Page, Route } from "@playwright/test";

import { FakeMessaging } from "./fakeMessaging.js";

/** What the fake instance should be doing. */
export interface FakeInstanceOptions {
  /** Whether the instance still needs its setup wizard. */
  setupRequired?: boolean;
  /**
   * Whether the user is new to encryption, so this browser sets it up and
   * can open conversations; otherwise the account has keys elsewhere and
   * the browser must restore them.
   */
  messaging?: boolean;
}

/** Field errors as the setup endpoint reports them (under `admin.`). */
function setupErrors(dataset: Dataset): Dataset["fieldErrors"] {
  return dataset.fieldErrors.map((error) => ({ ...error, field: `admin.${error.field}` }));
}

/** The Admin panel's settings. */
function adminAnswers(dataset: Dataset): Record<string, [number, unknown]> {
  return {
    "GET /api/v1/admin/tls": [200, dataset.tls],
    "GET /api/v1/admin/smtp": [200, dataset.smtp],
    "GET /api/v1/admin/uploads": [200, { attachmentMaxBytes: 25 << 20 }],
  };
}

function answers(dataset: Dataset, options: FakeInstanceOptions): Record<string, [number, unknown]> {
  const validation = (fields: Dataset["fieldErrors"]): [number, unknown] => [
    422,
    { code: "validation_failed", message: "One or more fields are invalid.", fields },
  ];
  const session = {
    id: "s",
    deviceName: "Browser",
    createdAt: "x",
    lastSeenAt: "x",
    expiresAt: "x",
    current: true,
  };
  const key = "A".repeat(43) + "=";
  const identity = {
    masterPublicKey: key,
    selfSigningPublicKey: key,
    selfSigningSignature: "A".repeat(86) + "==",
  };
  return {
    "GET /api/v1/instance": [200, dataset.instance],
    "GET /api/v1/setup": [
      200,
      {
        setupRequired: options.setupRequired ?? false,
        secureConnection: dataset.instance.secureConnection,
        domainFromEnvironment: false,
      },
    ],
    "POST /api/v1/setup": validation(setupErrors(dataset)),
    "POST /api/v1/auth/register": validation(dataset.fieldErrors),
    "GET /api/v1/auth/oidc/providers": [200, dataset.providers],
    "POST /api/v1/auth/login": [200, { token: "nxs_visual", session, user: dataset.user }],
    "GET /api/v1/users/me": [200, dataset.user],
    // The account already has an identity, so this browser is a new device
    // that must restore it: the screens show that step.
    "GET /api/v1/auth/sessions": [200, [session]],
    [`GET /api/v1/users/${dataset.user.id}/identity`]: [200, identity],
    ...adminAnswers(dataset),
  };
}

/** A request's JSON body; undefined when it has none or it is not JSON. */
function jsonBody(text: string | null): unknown {
  try {
    return text === null ? undefined : (JSON.parse(text) as unknown);
  } catch {
    return undefined;
  }
}

/** Answers the app's requests from `dataset`. Call before the first navigation. */
export async function fakeInstance(
  page: Page,
  dataset: Dataset,
  options: FakeInstanceOptions = {},
): Promise<void> {
  const table = answers(dataset, options);
  const messaging = options.messaging === true ? new FakeMessaging(dataset) : undefined;
  await page.route("**/api/v1/**", async (route: Route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const key = `${request.method()} ${path}`;
    const [status, body] = messaging?.answer(request.method(), path, jsonBody(request.postData())) ??
      table[key] ?? [404, { code: "not_found", message: key }];
    await route.fulfill(
      status === 204 ? { status } : { status, contentType: "application/json", body: JSON.stringify(body) },
    );
  });
  await page.routeWebSocket("**/api/v1/gateway", (socket) => {
    socket.send(JSON.stringify({ type: "hello", data: { heartbeatIntervalMs: 30000, protocolVersion: 1 } }));
    socket.onMessage((message) => {
      const frame = JSON.parse(String(message)) as { type: string };
      if (frame.type === "identify") {
        const presence = {
          userId: dataset.user.id,
          status: dataset.presence,
          customStatus: dataset.user.customStatus,
        };
        socket.send(
          JSON.stringify({
            type: "ready",
            seq: 1,
            data: { gatewaySessionId: "g", user: dataset.user, presence },
          }),
        );
      } else if (frame.type === "heartbeat") {
        socket.send(JSON.stringify({ type: "heartbeat_ack", data: {} }));
      }
    });
  });
}
