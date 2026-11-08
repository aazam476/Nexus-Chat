import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import * as WebBrowser from "expo-web-browser";
import { router } from "expo-router";

import Login from "../../app/login";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { ada, fakeConnection, renderWithApp } from "../testing/renderWithApp";
import { isolate } from "../i18n/bidi";

jest.mock("expo-web-browser", () => ({
  ...jest.requireActual<typeof WebBrowser>("expo-web-browser"),
  openAuthSessionAsync: jest.fn(),
}));

const session = {
  id: "0195f0e0-0000-7000-8000-0000000000aa",
  deviceName: "iOS app",
  createdAt: "2026-11-08T12:00:00Z",
  lastSeenAt: "2026-11-08T12:00:00Z",
  expiresAt: "2027-02-06T12:00:00Z",
  current: true,
};

async function logIn(server: FakeServer, loginName = "ada", password = "a long password") {
  const connection = await renderWithApp(<Login />, fakeConnection(server));
  await fireEvent.changeText(screen.getByTestId("login-name"), loginName);
  await fireEvent.changeText(screen.getByTestId("login-password"), password);
  await fireEvent.press(screen.getByTestId("login-submit"));
  return connection;
}

describe("the login screen", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it("logs in with a password", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/login", { status: 200, body: { token: "nxs_t", session, user: ada } });

    const connection = await logIn(server, " ada ");

    await waitFor(() => {
      expect(connection.signedIn).toHaveBeenCalledWith("nxs_t", ada);
    });
    expect(server.requests[1]?.body).toEqual({
      login: "ada",
      password: "a long password",
      deviceName: "iOS app",
    });
  });

  it("asks for a second factor and completes with it", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/login", {
      status: 202,
      body: { challengeToken: "challenge", methods: ["totp", "backup_code"] },
    });
    server.answer("POST /api/v1/auth/login/mfa", {
      status: 200,
      body: { token: "nxs_m", session, user: ada },
    });
    const connection = await logIn(server);

    await fireEvent.press(await screen.findByText("Use a backup code instead"));
    await fireEvent.press(screen.getByText("Use an authenticator code instead"));
    await fireEvent.changeText(screen.getByTestId("mfa-code"), " 123456 ");
    await fireEvent.press(screen.getByTestId("mfa-submit"));

    await waitFor(() => {
      expect(connection.signedIn).toHaveBeenCalledWith("nxs_m", ada);
    });
    expect(server.requests.at(-1)?.body).toEqual({
      challengeToken: "challenge",
      method: "totp",
      code: "123456",
      deviceName: "iOS app",
    });
  });

  it("starts over after a wrong second factor", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/login", {
      status: 202,
      body: { challengeToken: "challenge", methods: ["backup_code"] },
    });
    server.answer("POST /api/v1/auth/login/mfa", {
      status: 401,
      body: { code: "invalid_second_factor", message: "no" },
    });
    await logIn(server);
    await fireEvent.changeText(await screen.findByTestId("mfa-code"), "AAAAA-BBBBB");
    await fireEvent.press(screen.getByTestId("mfa-submit"));

    expect(
      await screen.findByText("That code did not work. Log in again to get a new attempt."),
    ).toBeOnTheScreen();
    expect(screen.getByTestId("login-submit")).toBeOnTheScreen();
  });

  it("goes back from the second factor step", async () => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/login", {
      status: 202,
      body: { challengeToken: "c", methods: ["totp"] },
    });
    await logIn(server);

    await fireEvent.press(await screen.findByText("Back"));

    expect(screen.getByTestId("login-submit")).toBeOnTheScreen();
  });

  it.each([
    [
      { status: 401, body: { code: "invalid_credentials", message: "x" } },
      "The username or password is wrong.",
    ],
    [
      { status: 429, body: { code: "login_delayed", message: "x" }, headers: { "Retry-After": "30" } },
      "Too many attempts. Try again in 30 seconds.",
    ],
    [
      { status: 429, body: { code: "login_delayed", message: "x" } },
      "Too many attempts. Try again in 60 seconds.",
    ],
    [
      { status: 429, body: { code: "rate_limited", message: "x" } },
      "Too many attempts. Wait a minute and try again.",
    ],
    [
      { status: 500, body: { code: "internal_error", message: "x" } },
      "Something went wrong. Try again in a moment.",
    ],
  ])("explains a failed login", async (answer, text) => {
    const server = fakeServer();
    server.answer("POST /api/v1/auth/login", answer);

    await logIn(server);

    expect(await screen.findByText(text)).toBeOnTheScreen();
  });

  it("offers providers and signs in with one in the in-app browser", async () => {
    const server = fakeServer();
    server.answer("GET /api/v1/auth/oidc/providers", {
      status: 200,
      body: [{ id: "corp", displayName: "Corp" }],
    });
    server.answer("POST /api/v1/auth/oidc/providers/corp/begin", {
      status: 200,
      body: { flowId: "flow-1", authorizationUrl: "https://idp.example.com/auth" },
    });
    server.answer("POST /api/v1/auth/oidc/complete", {
      status: 200,
      body: { token: "nxs_p", session, user: ada },
    });
    jest
      .mocked(WebBrowser.openAuthSessionAsync)
      .mockResolvedValue({ type: "success", url: "nexuschat://oidc-return#flow=flow-1&code=the-code" });
    const connection = await renderWithApp(<Login />, fakeConnection(server));

    await fireEvent.press(await screen.findByText(`Continue with ${isolate("Corp")}`));

    await waitFor(() => {
      expect(connection.signedIn).toHaveBeenCalledWith("nxs_p", ada);
    });
    expect(WebBrowser.openAuthSessionAsync).toHaveBeenCalledWith(
      "https://idp.example.com/auth",
      "nexuschat://oidc-return",
    );
    expect(server.requests.at(-1)?.body).toEqual({
      flowId: "flow-1",
      code: "the-code",
      deviceName: "iOS app",
    });
  });

  it("explains a provider sign-in that found no account", async () => {
    const server = fakeServer();
    server.answer("GET /api/v1/auth/oidc/providers", {
      status: 200,
      body: [{ id: "corp", displayName: "Corp" }],
    });
    server.answer("POST /api/v1/auth/oidc/providers/corp/begin", {
      status: 200,
      body: { flowId: "f", authorizationUrl: "https://idp" },
    });
    server.answer("POST /api/v1/auth/oidc/complete", {
      status: 403,
      body: { code: "no_account", message: "x" },
    });
    jest
      .mocked(WebBrowser.openAuthSessionAsync)
      .mockResolvedValue({ type: "success", url: "nexuschat://oidc-return#code=c" });
    await renderWithApp(<Login />, fakeConnection(server));

    await fireEvent.press(await screen.findByText(`Continue with ${isolate("Corp")}`));

    expect(await screen.findByText(/No account is linked to this login/)).toBeOnTheScreen();
  });

  it("ignores a provider sign-in the user closed, and reports one that would not start", async () => {
    const server = fakeServer();
    server.answer("GET /api/v1/auth/oidc/providers", {
      status: 200,
      body: [{ id: "corp", displayName: "Corp" }],
    });
    server.answer("POST /api/v1/auth/oidc/providers/corp/begin", {
      status: 200,
      body: { flowId: "f", authorizationUrl: "https://idp" },
    });
    jest
      .mocked(WebBrowser.openAuthSessionAsync)
      .mockResolvedValue({ type: WebBrowser.WebBrowserResultType.CANCEL });
    const connection = await renderWithApp(<Login />, fakeConnection(server));
    await fireEvent.press(await screen.findByText(`Continue with ${isolate("Corp")}`));
    expect(connection.signedIn).not.toHaveBeenCalled();

    server.answer("POST /api/v1/auth/oidc/providers/corp/begin", {
      status: 502,
      body: { code: "provider_unavailable", message: "x" },
    });
    await fireEvent.press(screen.getByText(`Continue with ${isolate("Corp")}`));
    expect(
      await screen.findByText("Signing in with the provider did not work. Try again."),
    ).toBeOnTheScreen();
  });

  it("links to sign-up and to another instance, and hides sign-up when closed", async () => {
    const server = fakeServer();
    const connection = await renderWithApp(<Login />, fakeConnection(server));
    await fireEvent.press(screen.getByTestId("go-register"));
    expect(router.push).toHaveBeenCalledWith("/register");
    await fireEvent.press(screen.getByText("Use another instance"));
    await waitFor(() => {
      expect(router.replace).toHaveBeenCalledWith("/connect");
    });
    expect(connection.forgetInstance).toHaveBeenCalled();
  });

  it("leaves once signed in, and hides sign-up on a closed instance", async () => {
    const server = fakeServer();
    const connection = fakeConnection(server, { user: ada, address: undefined, instance: undefined });
    await renderWithApp(<Login />, connection);
    expect(router.replace).toHaveBeenCalledWith("/home");
    expect(screen.getByText(`Log in to ${isolate("localhost")}`)).toBeOnTheScreen();

    const closed = fakeConnection(server);
    closed.instance = {
      info: { name: "Closed", registrationMode: "closed", secureConnection: true },
      setupRequired: false,
      domainFromEnvironment: false,
    };
    await renderWithApp(<Login />, closed);
    expect(screen.queryByTestId("go-register")).toBeNull();
  });
});
