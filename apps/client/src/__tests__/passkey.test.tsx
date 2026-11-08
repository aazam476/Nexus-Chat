import { act, renderHook, waitFor } from "@testing-library/react-native";
import type { ReactNode } from "react";

import { usePasskeyLogin } from "../auth/usePasskeyLogin";
import { ConnectionContext } from "../connection/ConnectionContext";
import { fakeServer } from "../testing/fakeServer";
import { ada, fakeConnection } from "../testing/renderWithApp";

jest.mock("../auth/webauthn", () => ({
  passkeysSupported: () => true,
  getAssertion: jest.fn().mockResolvedValue({ id: "credential-1" }),
}));

const session = {
  id: "0195f0e0-0000-7000-8000-0000000000aa",
  deviceName: "iOS app",
  createdAt: "2026-11-08T12:00:00Z",
  lastSeenAt: "2026-11-08T12:00:00Z",
  expiresAt: "2027-02-06T12:00:00Z",
  current: true,
};

describe("passkey login", () => {
  it("sends the authenticator's answer and starts the session", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/passkey/begin", {
      status: 200,
      body: { ceremonyId: "ceremony-1", options: { publicKey: { challenge: "x" } } },
    });
    server.answer("POST /api/v1/auth/passkey/finish", {
      status: 200,
      body: { token: "nxs_k", session, user: ada },
    });
    const connection = fakeConnection(server);
    const wrapper = ({ children }: { children: ReactNode }) => (
      <ConnectionContext value={connection}>{children}</ConnectionContext>
    );
    const { result } = await renderHook(() => usePasskeyLogin(), { wrapper });

    expect(result.current.supported).toBe(true);
    await act(() => result.current.logIn());

    await waitFor(() => {
      expect(connection.signedIn).toHaveBeenCalledWith("nxs_k", ada);
    });
    expect(server.requests.at(-1)?.body).toEqual({
      ceremonyId: "ceremony-1",
      credential: { id: "credential-1" },
      deviceName: "iOS app",
    });
  });

  it("reports a passkey the server rejects", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/passkey/begin", {
      status: 200,
      body: { ceremonyId: "c", options: { publicKey: {} } },
    });
    server.answer("POST /api/v1/auth/passkey/finish", {
      status: 401,
      body: { code: "invalid_passkey", message: "x" },
    });
    const connection = fakeConnection(server);
    const wrapper = ({ children }: { children: ReactNode }) => (
      <ConnectionContext value={connection}>{children}</ConnectionContext>
    );
    const { result } = await renderHook(() => usePasskeyLogin(), { wrapper });

    await act(() => result.current.logIn());

    await waitFor(() => {
      expect(result.current.error).toEqual({ key: "login.passkeyFailed" });
    });
    expect(connection.signedIn).not.toHaveBeenCalled();
  });
});
