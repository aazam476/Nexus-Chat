import { act, fireEvent, renderHook, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";
import type { ReactNode } from "react";

import ProviderReturn from "../../app/oidc-return";
import { deviceName } from "../auth/deviceName";
import { startProviderSignIn } from "../auth/providerSignIn.web";
import { useProviderReturn } from "../auth/useProviderReturn";
import { usePasskeyLogin } from "../auth/usePasskeyLogin";
import { getAssertion as nativeAssertion, passkeysSupported as nativeSupported } from "../auth/webauthn";
import { getAssertion, passkeysSupported } from "../auth/webauthn.web";
import { ConnectionContext } from "../connection/ConnectionContext";
import type { Connection } from "../connection/useConnectionState";
import { defaultAddress } from "../instance/defaultAddress.web";
import { createMemoryStorage, storageKeys } from "../storage/storage";
import { fakeServer } from "../testing/fakeServer";
import { ada, fakeConnection, renderWithApp } from "../testing/renderWithApp";

const session = {
  id: "0195f0e0-0000-7000-8000-0000000000aa",
  deviceName: "Browser",
  createdAt: "2026-11-08T12:00:00Z",
  lastSeenAt: "2026-11-08T12:00:00Z",
  expiresAt: "2027-02-06T12:00:00Z",
  current: true,
};

function wrapperFor(connection: Connection) {
  return ({ children }: { children: ReactNode }) => (
    <ConnectionContext value={connection}>{children}</ConnectionContext>
  );
}

beforeEach(() => {
  jest.clearAllMocks();
});

describe("returning from a provider on web", () => {
  it("finishes the sign-in with the saved flow and the returned code", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/oidc/complete", {
      status: 200,
      body: { token: "nxs_o", session, user: ada },
    });
    const connection = fakeConnection(server, { storage: createMemoryStorage() });
    await connection.storage.set(storageKeys.providerFlow, "flow-9");

    await renderHook(() => useProviderReturn("#flow=flow-9&code=c9"), { wrapper: wrapperFor(connection) });

    await waitFor(() => {
      expect(connection.signedIn).toHaveBeenCalledWith("nxs_o", ada);
    });
    expect(server.requests.at(-1)?.body).toMatchObject({ flowId: "flow-9", code: "c9" });
    expect(await connection.storage.get(storageKeys.providerFlow)).toBeUndefined();
  });

  it("asks for a second factor when the account has one", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/oidc/complete", {
      status: 202,
      body: { challengeToken: "ch", methods: ["totp"] },
    });
    server.answer("POST /api/v1/auth/login/mfa", {
      status: 200,
      body: { token: "nxs_2", session, user: ada },
    });
    const connection = fakeConnection(server, { storage: createMemoryStorage() });
    await connection.storage.set(storageKeys.providerFlow, "f");
    Object.defineProperty(window, "location", { value: { hash: "#code=c" }, configurable: true });

    await renderWithApp(<ProviderReturn />, connection);
    await fireEvent.changeText(await screen.findByTestId("mfa-code"), "123456");
    await fireEvent.press(screen.getByTestId("mfa-submit"));

    await waitFor(() => {
      expect(connection.signedIn).toHaveBeenCalledWith("nxs_2", ada);
    });
    await fireEvent.press(screen.getByText("Back"));
    expect(router.replace).toHaveBeenCalledWith("/login");
  });

  it.each([
    ["no saved flow", undefined, "#code=c"],
    ["the provider refused", "f", "#error=provider_error"],
  ])("explains a failed sign-in: %s", async (_name, flowId, fragment) => {
    const server = fakeServer();
    const connection = fakeConnection(server, { storage: createMemoryStorage() });
    if (flowId !== undefined) {
      await connection.storage.set(storageKeys.providerFlow, flowId);
    }
    Object.defineProperty(window, "location", { value: { hash: fragment }, configurable: true });

    await renderWithApp(<ProviderReturn />, connection);

    expect(
      await screen.findByText("Signing in with the provider did not work. Try again."),
    ).toBeOnTheScreen();
    await fireEvent.press(screen.getByText("Back"));
    expect(router.replace).toHaveBeenCalledWith("/login");
  });

  it("waits for the instance before finishing", async () => {
    const connection = fakeConnection(fakeServer(), { client: undefined });
    const { result } = await renderHook(() => useProviderReturn("#code=c"), {
      wrapper: wrapperFor(connection),
    });

    expect(result.current).toEqual({ kind: "working" });
  });
});

describe("starting a provider sign-in on web", () => {
  it("saves the flow and sends the browser to the provider", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/oidc/providers/corp/begin", {
      status: 200,
      body: { flowId: "flow-1", authorizationUrl: "https://idp.example.com/auth" },
    });
    const assign = jest.fn();
    Object.defineProperty(window, "location", { value: { assign }, configurable: true });
    const storage = createMemoryStorage();

    expect(await startProviderSignIn(server.client, "corp", storage)).toBeUndefined();

    expect(assign).toHaveBeenCalledWith("https://idp.example.com/auth");
    expect(await storage.get(storageKeys.providerFlow)).toBe("flow-1");
    expect(server.requests.at(-1)?.body).toEqual({ returnTo: "/oidc-return" });
    server.answer("POST /api/v1/auth/oidc/providers/corp/begin", {
      status: 404,
      body: { code: "provider_not_found", message: "x" },
    });
    expect(await startProviderSignIn(server.client, "corp", storage)).toEqual({
      kind: "failed",
      message: { key: "login.providerFailed" },
    });
  });

  it("uses the desktop app's sign-in window when there is one", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/oidc/providers/corp/begin", {
      status: 200,
      body: { flowId: "flow-2", authorizationUrl: "https://idp.example.com/auth" },
    });
    server.answer("POST /api/v1/auth/oidc/complete", {
      status: 200,
      body: { token: "nxs_d", session, user: ada },
    });
    const signInWithProvider = jest
      .fn<Promise<string | undefined>, [string]>()
      .mockResolvedValueOnce("flow=flow-2&code=the-code")
      .mockResolvedValueOnce(undefined);
    Object.assign(window, { nexusDesktop: { signInWithProvider } });
    const storage = createMemoryStorage();

    try {
      expect(await startProviderSignIn(server.client, "corp", storage)).toEqual({
        kind: "signedIn",
        result: { token: "nxs_d", session, user: ada },
      });
      expect(signInWithProvider).toHaveBeenCalledWith("https://idp.example.com/auth");
      expect(server.requests.at(-2)?.body).toEqual({ returnTo: "nexuschat://oidc-return" });
      expect(server.requests.at(-1)?.body).toEqual({
        flowId: "flow-2",
        code: "the-code",
        deviceName: deviceName(),
      });
      expect(await storage.get(storageKeys.providerFlow)).toBeUndefined();

      // The user closed the window.
      expect(await startProviderSignIn(server.client, "corp", storage)).toBeUndefined();
    } finally {
      delete (window as { nexusDesktop?: unknown }).nexusDesktop;
    }
  });
});

describe("passkeys", () => {
  it("are not offered by the native apps yet", async () => {
    expect(nativeSupported()).toBe(false);
    await expect(nativeAssertion({})).rejects.toThrow("not supported");
  });

  it("use the browser's WebAuthn JSON helpers", async () => {
    const parse = jest.fn((options: unknown) => ({ parsed: options }));
    const credential = { toJSON: () => ({ id: "cred" }) };
    const get = jest.fn().mockResolvedValue(credential);
    Object.assign(globalThis, { PublicKeyCredential: { parseRequestOptionsFromJSON: parse } });
    Object.defineProperty(window, "isSecureContext", { value: true, configurable: true });
    Object.defineProperty(globalThis.navigator, "credentials", { value: { get }, configurable: true });

    expect(passkeysSupported()).toBe(true);
    expect(await getAssertion({ publicKey: { challenge: "abc" } })).toEqual({ id: "cred" });
    expect(get).toHaveBeenCalledWith({ publicKey: { parsed: { challenge: "abc" } } });

    get.mockResolvedValue(null);
    await expect(getAssertion({ publicKey: {} })).rejects.toThrow("no credential was chosen");
    Reflect.deleteProperty(globalThis, "PublicKeyCredential");
    expect(passkeysSupported()).toBe(false);
  });

  it("log in through the hook, and report failures", async () => {
    const server = fakeServer();
    const connection = fakeConnection(server);
    const { result } = await renderHook(() => usePasskeyLogin(), { wrapper: wrapperFor(connection) });
    expect(result.current.supported).toBe(false);

    server.answer("POST /api/v1/auth/passkey/begin", {
      status: 409,
      body: { code: "webauthn_unavailable", message: "x" },
    });
    await act(() => result.current.logIn());
    await waitFor(() => {
      expect(result.current.error).toEqual({ key: "login.passkeyUnavailable" });
    });

    server.answer("POST /api/v1/auth/passkey/begin", {
      status: 200,
      body: { ceremonyId: "c", options: { publicKey: {} } },
    });
    await act(() => result.current.logIn());
    await waitFor(() => {
      expect(result.current.error).toEqual({ key: "login.passkeyFailed" });
    });
  });
});

describe("the web build's instance", () => {
  it("is the one serving the page", () => {
    Object.defineProperty(window, "location", {
      value: { protocol: "https:", origin: "https://chat.example.com" },
      configurable: true,
    });
    expect(defaultAddress()).toBe("https://chat.example.com");
    Object.defineProperty(window, "location", {
      value: { protocol: "file:", origin: "null" },
      configurable: true,
    });
    expect(defaultAddress()).toBeUndefined();
  });
});
