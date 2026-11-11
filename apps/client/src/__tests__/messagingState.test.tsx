import { act, screen, waitFor } from "@testing-library/react-native";
import { Text } from "react-native";
import { useEffect, type ReactNode } from "react";

import { useMessaging, MessagingProvider } from "../messaging/MessagingContext";
import { startMessenger } from "../messaging/messenger";
import type { Messaging } from "../messaging/useMessagingState";
import { createMemoryDatabase } from "../storage/localDatabase";
import { FakeInstance } from "../testing/fakeDelivery";
import { fakeSockets, nth } from "../testing/fakeSocket";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { ada, fakeConnection, renderWithApp } from "../testing/renderWithApp";

jest.setTimeout(60_000);

let latest: Messaging | undefined;
// Trees from earlier tests may still render; only this test's probe reports.
let generation = 0;

function Probe({ of }: { of: number }): ReactNode {
  const messaging = useMessaging();
  useEffect(() => {
    if (of === generation) {
      latest = messaging;
    }
  });
  return <Text testID="status">{messaging.status}</Text>;
}

function serverWithSession(id: string | undefined): FakeServer {
  const server = fakeServer();
  server.answer("GET /api/v1/auth/sessions", {
    status: 200,
    body:
      id === undefined
        ? []
        : [{ id, deviceName: "Phone", createdAt: "", lastSeenAt: "", expiresAt: "", current: true }],
  });
  return server;
}

async function start(
  instance: FakeInstance,
  deviceId: string | undefined,
  database = createMemoryDatabase(),
): Promise<FakeServer> {
  const server = serverWithSession(deviceId);
  const environment = {
    openDatabase: () => Promise.resolve(database),
    createApi: () => instance.api(ada.id, deviceId ?? "none"),
  };
  await renderWithApp(
    <MessagingProvider environment={environment}>
      <Probe of={generation} />
    </MessagingProvider>,
    fakeConnection(server, { user: ada, sessionToken: "token" }),
  );
  return server;
}

const status = (): string => String(screen.getByTestId("status").props.children);

describe("messaging state", () => {
  beforeEach(() => {
    generation += 1;
    latest = undefined;
  });

  it("is off while nobody is logged in", async () => {
    await renderWithApp(
      <MessagingProvider
        environment={{ openDatabase: () => Promise.resolve(createMemoryDatabase()), createApi: jest.fn() }}
      >
        <Probe of={generation} />
      </MessagingProvider>,
    );
    await waitFor(() => {
      expect(status()).toBe("off");
    });
  });

  it("shows a new account's phrase until confirmed, then is ready", async () => {
    const instance = new FakeInstance();
    await start(instance, "ada-laptop");
    await waitFor(() => {
      expect(status()).toBe("recovery-phrase");
    });
    expect(latest?.status === "recovery-phrase" && latest.phrase.split(" ")).toHaveLength(24);
    await act(async () => {
      await latest?.confirmPhrase();
    });
    expect(status()).toBe("ready");
    await act(async () => {
      await latest?.confirmPhrase();
    });
    expect(status()).toBe("ready");
  });

  it("asks a second device to restore, refusing a wrong phrase", async () => {
    const instance = new FakeInstance();
    const laptop = await startMessenger({
      api: instance.api(ada.id, "ada-laptop"),
      database: createMemoryDatabase(),
      user: ada,
      where: { domain: "chat.example.com", deviceId: "ada-laptop" },
      instanceKey: new Uint8Array(32),
    });
    const phrase = laptop.status === "ready" ? (laptop.recoveryPhrase ?? "") : "";

    await start(instance, "ada-phone");
    await waitFor(() => {
      expect(status()).toBe("needs-restore");
    });
    await act(async () => {
      expect(await latest?.restore("not the phrase")).toBe(false);
    });
    await act(async () => {
      expect(await latest?.restore(phrase)).toBe(true);
    });
    await waitFor(() => {
      expect(status()).toBe("ready");
    });
  });

  it("fails when the server does not report this session, and can retry", async () => {
    const server = await start(new FakeInstance(), undefined);
    await waitFor(() => {
      expect(status()).toBe("failed");
    });
    await act(async () => {
      expect(await latest?.restore("x")).toBe(false);
    });
    await act(async () => {
      latest?.retry();
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(server.requests.filter((request) => request.path === "/api/v1/auth/sessions")).toHaveLength(2);
    });
    await waitFor(() => {
      expect(status()).toBe("failed");
    });
  });

  it("applies new messages when the gateway announces them", async () => {
    const sockets = fakeSockets();
    const instance = new FakeInstance();
    await start(instance, "ada-laptop");
    await waitFor(() => {
      expect(status()).toBe("recovery-phrase");
    });
    const grace = await startMessenger({
      api: instance.api("user-grace", "grace-phone"),
      database: createMemoryDatabase(),
      user: { id: "user-grace", username: "grace" },
      where: { domain: "chat.example.com", deviceId: "grace-phone" },
      instanceKey: new Uint8Array(32),
    });
    if (grace.status !== "ready" || latest?.status !== "recovery-phrase") {
      throw new Error("both should be ready");
    }
    const people = [
      { id: ada.id, username: "ada" },
      { id: "user-grace", username: "grace" },
    ];
    const adaMessenger = latest.messenger;
    instance.addConversation("dm");
    await adaMessenger.open("dm", people);
    await grace.messenger.open("dm", people);
    await grace.messenger.send("dm", { kind: "text", text: "hi Ada" });

    await act(async () => {
      await Promise.resolve();
      nth(sockets, 0).receive({ type: "presence_update", seq: 1, data: {} });
      nth(sockets, 0).receive({ type: "mls_messages", seq: 2, data: { conversationId: "dm", lastId: 9 } });
      nth(sockets, 0).receive({ type: "mls_messages", seq: 3, data: { conversationId: 7 } });
      nth(sockets, 0).receive({ type: "mls_messages", seq: 4, data: null });
    });
    await waitFor(async () => {
      expect(await adaMessenger.messages("dm")).toHaveLength(1);
    });
  });
});
