import { renderHook, waitFor } from "@testing-library/react-native";
import { act, type ReactNode } from "react";

import { ConnectionProvider, useConnection } from "../connection/ConnectionContext";
import { useConnectionState } from "../connection/useConnectionState";
import { createMemoryStorage, storageKeys, type SecureStorage } from "../storage/storage";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { ada, exampleInstance } from "../testing/renderWithApp";

const setupDone = { setupRequired: false, secureConnection: true, domainFromEnvironment: false };

function instanceServer(): FakeServer {
  const server = fakeServer();
  server.answer("GET /api/v1/instance", { status: 200, body: exampleInstance });
  server.answer("GET /api/v1/setup", { status: 200, body: setupDone });
  globalThis.fetch = server.fetch;
  return server;
}

async function connectionWith(storage: SecureStorage) {
  const hook = await renderHook(() => useConnectionState(storage));
  await waitFor(() => {
    expect(hook.result.current.loading).toBe(false);
  });
  return hook;
}

describe("the connection", () => {
  it("starts with nothing on a fresh install", async () => {
    instanceServer();
    const { result } = await connectionWith(createMemoryStorage());

    expect(result.current.address).toBeUndefined();
    expect(result.current.client).toBeUndefined();
  });

  it("reopens the saved instance and session", async () => {
    const server = instanceServer();
    server.answer("GET /api/v1/users/me", { status: 200, body: ada });
    const storage = createMemoryStorage();
    await storage.set(storageKeys.instance, "https://chat.example.com");
    await storage.set(storageKeys.session, "nxs_saved");

    const { result } = await connectionWith(storage);

    expect(result.current.user).toEqual(ada);
    expect(result.current.sessionToken).toBe("nxs_saved");
    expect(result.current.instance?.info.name).toBe("Example");
    expect(server.requests.find((request) => request.path === "/api/v1/users/me")?.authorization).toBe(
      "Bearer nxs_saved",
    );
  });

  it("forgets a session the server no longer accepts", async () => {
    const server = instanceServer();
    server.answer("GET /api/v1/users/me", { status: 401, body: { code: "unauthorized", message: "x" } });
    const storage = createMemoryStorage();
    await storage.set(storageKeys.instance, "https://chat.example.com");
    await storage.set(storageKeys.session, "nxs_revoked");

    const { result } = await connectionWith(storage);

    expect(result.current.user).toBeUndefined();
    expect(await storage.get(storageKeys.session)).toBeUndefined();
  });

  it("treats an instance that does not answer as missing", async () => {
    globalThis.fetch = () => Promise.reject(new Error("offline"));
    const storage = createMemoryStorage();
    await storage.set(storageKeys.instance, "https://gone.example.com");

    const { result } = await connectionWith(storage);

    expect(result.current.instance).toBeUndefined();
    expect(await result.current.chooseInstance("https://gone.example.com")).toBe(false);
  });

  it("chooses an instance, signs in and out, and forgets it", async () => {
    const server = instanceServer();
    server.answer("POST /api/v1/auth/logout", { status: 204 });
    const storage = createMemoryStorage();
    const { result } = await connectionWith(storage);

    await act(async () => {
      expect(await result.current.chooseInstance("https://chat.example.com")).toBe(true);
    });
    expect(await storage.get(storageKeys.instance)).toBe("https://chat.example.com");
    await act(() => result.current.signedIn("nxs_new", ada));
    expect(result.current.user).toEqual(ada);
    expect(await storage.get(storageKeys.session)).toBe("nxs_new");
    act(() => {
      result.current.setUser({ ...ada, status: "dnd" });
    });
    expect(result.current.user?.status).toBe("dnd");

    await act(() => result.current.refreshInstance());
    await act(() => result.current.signOut());
    expect(server.requests.at(-1)).toMatchObject({
      path: "/api/v1/auth/logout",
      authorization: "Bearer nxs_new",
    });
    expect(result.current.user).toBeUndefined();

    await act(() => result.current.forgetInstance());
    expect(result.current.address).toBeUndefined();
    expect(await storage.get(storageKeys.instance)).toBeUndefined();
    await act(() => result.current.refreshInstance()); // no instance: nothing to reload
    expect(result.current.instance).toBeUndefined();
  });

  it("logs out locally even when the server cannot be reached", async () => {
    instanceServer();
    const storage = createMemoryStorage();
    const { result } = await connectionWith(storage);
    await act(async () => {
      await result.current.chooseInstance("https://chat.example.com");
      await result.current.signedIn("nxs_x", ada);
    });
    globalThis.fetch = () => Promise.reject(new Error("offline"));

    await act(() => result.current.signOut());

    expect(result.current.user).toBeUndefined();
  });
});

describe("ConnectionProvider", () => {
  it("shares the connection, and useConnection needs it", async () => {
    instanceServer();
    const storage = createMemoryStorage();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <ConnectionProvider storage={storage}>{children}</ConnectionProvider>
    );
    const { result } = await renderHook(() => useConnection(), { wrapper });
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await expect(renderHook(() => useConnection())).rejects.toThrow(
      "useConnection needs a ConnectionProvider",
    );
  });
});
